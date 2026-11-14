package realtime

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"math/rand/v2"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform"
	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	"github.com/google/uuid"
)

var (
	ErrCommandQueueFull   = errors.New("room command queue is full")
	ErrStaleQueue         = errors.New("stale queue mutation")
	ErrControlLocked      = errors.New("playback control is locked")
	ErrControlDenied      = errors.New("playback control is denied")
	ErrForbidden          = errors.New("room operation is forbidden")
	ErrSeekForbidden      = errors.New("playback seek is forbidden")
	ErrPreconditionFailed = errors.New("playback precondition failed")
	ErrNoHistory          = errors.New("no playback history available")
)

const (
	endedToleranceMS = int64(3000)
	fallbackGraceMS  = int64(5000)
	readyGraceMS     = 8000 * time.Millisecond
	stallGraceMS     = 15000 * time.Millisecond
)

type Broadcaster interface {
	BroadcastRoom(string, string, any)
	BroadcastAll(string, any)
}

type CommittedWatermark struct {
	StateVersion int64
	PlayEpoch    int64
	QueueVersion int64
}

// ControlResult.Committed captures the accepted state inside the serial command,
// after persistence for applied mutations or without mutation for a noop.
type ControlResult struct {
	Applied    bool
	Committed  *CommittedWatermark
	MutationID string
	Replayed   bool
}

type Manager struct {
	store        *storesqlite.Store
	queueSize    int
	timeout      time.Duration
	idle         time.Duration
	broadcaster  Broadcaster
	mu           sync.Mutex
	runtimes     map[string]*RoomRuntime
	loading      map[string]*roomLoad
	closed       bool
	limits       QueueLimits
	chat         *chatPolicy
	preResolve   func(context.Context, storesqlite.Music)
	verifyStream func(context.Context, storesqlite.Music) error
}

// SetPreResolver installs best-effort media preparation for the next queued track.
// It must be configured before rooms are loaded.
func (m *Manager) SetPreResolver(resolve func(context.Context, storesqlite.Music)) {
	m.mu.Lock()
	m.preResolve = resolve
	m.mu.Unlock()
}

// SetStreamVerifier installs the platform resolver used to verify client failure claims.
func (m *Manager) SetStreamVerifier(verify func(context.Context, storesqlite.Music) error) {
	m.mu.Lock()
	m.verifyStream = verify
	m.mu.Unlock()
}

func NewManager(store *storesqlite.Store, queueSize int, timeout, idle time.Duration, broadcaster Broadcaster, limits ...QueueLimits) *Manager {
	queueLimits := QueueLimits{MaxSize: 1000, MaxUserSongs: 100, HistorySize: 50}
	if len(limits) > 0 {
		queueLimits = limits[0]
	}
	return &Manager{chat: newChatPolicy(ChatLimits{MinInterval: time.Second, MaxHistorySize: 1000, MaxMessageLength: 200}), limits: queueLimits, store: store, queueSize: max(1, queueSize), timeout: timeout, idle: idle, broadcaster: broadcaster, runtimes: map[string]*RoomRuntime{}, loading: map[string]*roomLoad{}}
}

type roomLoad struct {
	done   chan struct{}
	err    error
	cancel context.CancelFunc
}

func (m *Manager) Room(ctx context.Context, roomID string) (*RoomRuntime, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Cached runtimes must obey the same admission check as a new load. Keep
	// this read outside the manager lock so other rooms remain independent.
	var active bool
	if err := m.store.Reader().QueryRowContext(ctx, "select exists(select 1 from room where id=? and deleted_at is null)", roomID).Scan(&active); err != nil {
		return nil, err
	}
	if !active {
		return nil, storesqlite.ErrRoomGone
	}
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		m.mu.Lock()
		if m.closed {
			m.mu.Unlock()
			return nil, context.Canceled
		}
		if rt := m.runtimes[roomID]; rt != nil && rt.ctx.Err() == nil {
			m.mu.Unlock()
			return rt, nil
		}
		if load := m.loading[roomID]; load != nil {
			m.mu.Unlock()
			select {
			case <-load.done:
				if load.err != nil {
					return nil, load.err
				}
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		loadCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), m.timeout)
		load := &roomLoad{done: make(chan struct{}), cancel: cancel}
		m.loading[roomID] = load
		m.mu.Unlock()
		var active bool
		err := m.store.Reader().QueryRowContext(loadCtx, "select exists(select 1 from room where id=? and deleted_at is null)", roomID).Scan(&active)
		if err == nil && !active {
			err = storesqlite.ErrRoomGone
		}
		var rt *RoomRuntime
		if err == nil {
			rt, err = newRoomRuntime(loadCtx, m.store, roomID, m.queueSize, m.timeout, m.idle, m.broadcaster, m.release, m.limits, m.chat)
			if rt != nil {
				rt.preResolve = m.preResolve
				rt.verifyStream = m.verifyStream
			}
		}
		m.mu.Lock()
		if err == nil && (m.closed || loadCtx.Err() != nil) {
			err = context.Canceled
		}
		if err == nil {
			m.runtimes[roomID] = rt
		}
		if err != nil && rt != nil {
			m.mu.Unlock()
			rt.Close()
			m.mu.Lock()
			rt = nil
		}
		load.err = err
		delete(m.loading, roomID)
		close(load.done)
		m.mu.Unlock()
		cancel()
		return rt, err
	}
}

func (m *Manager) release(rt *RoomRuntime) {
	m.mu.Lock()
	if m.runtimes[rt.roomID] == rt {
		delete(m.runtimes, rt.roomID)
	}
	m.mu.Unlock()
}

// CloseRoom stops an existing runtime without creating a replacement for a deleted room.
func (m *Manager) CloseRoom(roomID string) {
	m.mu.Lock()
	runtime := m.runtimes[roomID]
	load := m.loading[roomID]
	if load != nil {
		load.cancel()
	}
	m.mu.Unlock()
	if load != nil {
		<-load.done
	}
	if runtime != nil {
		runtime.Close()
	}
}

func (m *Manager) Close() {
	m.mu.Lock()
	m.closed = true
	loads := make([]*roomLoad, 0, len(m.loading))
	for _, load := range m.loading {
		load.cancel()
		loads = append(loads, load)
	}
	runtimes := make([]*RoomRuntime, 0, len(m.runtimes))
	for _, runtime := range m.runtimes {
		runtimes = append(runtimes, runtime)
	}
	m.mu.Unlock()
	for _, load := range loads {
		<-load.done
	}
	for _, runtime := range runtimes {
		runtime.Close()
	}
}

// ReloadQueue refreshes the in-memory queue of a room that is already loaded, so rows removed
// outside the realtime layer stop being served from memory and written back. removedTrackID names
// the track that triggered the reload: when it is still the room's current music, the runtime drops
// it instead of persisting a reference to a row that no longer exists. Rooms that were never loaded
// are skipped: removing a track must not spawn a runtime just to broadcast to nobody.
func (m *Manager) ReloadQueue(ctx context.Context, roomID, removedTrackID string) error {
	m.mu.Lock()
	runtime := m.runtimes[roomID]
	closed := m.closed
	m.mu.Unlock()
	if closed || runtime == nil {
		return nil
	}
	return runtime.ReloadQueue(ctx, removedTrackID)
}

type command struct {
	ctx    context.Context
	apply  func(*RoomRuntime, context.Context) (any, error)
	result chan commandResult
}
type commandResult struct {
	value any
	err   error
}

type RoomRuntime struct {
	roomID              string
	queueRepo           *storesqlite.QueueRepository
	stateRepo           *storesqlite.PlaybackStateRepository
	realtimeRepo        *storesqlite.RealtimeRepository
	chatRepo            *storesqlite.ChatRepository
	playlistRepo        *storesqlite.UserPlaylistRepository
	queue               []storesqlite.QueueItem
	state               storesqlite.PlaybackState
	history             []storesqlite.HistoryEntry
	historySkipped      int
	queueVersion        int64
	commands            chan command
	ctx                 context.Context
	cancel              context.CancelFunc
	done                chan struct{}
	timeout             time.Duration
	broadcaster         Broadcaster
	mutations           map[string]queueMutationRecord
	mutationOrder       []string
	mutationCounts      map[string]int
	limits              QueueLimits
	chat                *chatPolicy
	controlScope        string
	controlMutations    map[controlKey]controlRecord
	controlOrder        []controlKey
	controlCounts       map[string]int
	idle                time.Duration
	lastActivity        time.Time
	clients             int
	advancePending      bool
	advanceFailures     int
	advanceRetryAt      time.Time
	currentQueueID      string
	sessionState        string
	sessionStateSince   time.Time
	failedEpoch         int64
	clockOwner          string
	timeoutStartPending bool
	eventWake           chan struct{}
	eventLogMu          sync.Mutex
	eventLogQueue       []playbackEvent
	eventLogGen         uint64
	preResolve          func(context.Context, storesqlite.Music)
	preResolveBusy      bool
	verifyStream        func(context.Context, storesqlite.Music) error
	verificationEpochs  map[int64]struct{}
	onClose             func(*RoomRuntime)
	closeOnce           sync.Once
}

type playbackEvent struct {
	roomID                        string
	at                            int64
	kind, queueID, result, reason string
	epoch                         int64
	actor                         *string
}

func newRoomRuntime(ctx context.Context, store *storesqlite.Store, roomID string, capacity int, timeout, idle time.Duration, broadcaster Broadcaster, onClose func(*RoomRuntime), limits QueueLimits, chat *chatPolicy) (*RoomRuntime, error) {
	queueRepo := storesqlite.NewQueueRepository(store, time.Now)
	queue, err := queueRepo.LoadQueue(ctx, roomID)
	if err != nil {
		return nil, err
	}
	stateRepo := storesqlite.NewPlaybackStateRepository(store)
	state, err := stateRepo.FindByRoomID(ctx, roomID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) && !strings.Contains(err.Error(), sql.ErrNoRows.Error()) {
			return nil, err
		}
		now := time.Now().UnixMilli()
		state = &storesqlite.PlaybackState{RoomID: roomID, LikedUserIDs: map[string]struct{}{}, LikeMarkers: []int64{}, LastPersistedAt: now}
	}
	runtimeCtx, cancel := context.WithCancel(context.Background())
	r := &RoomRuntime{roomID: roomID, queueRepo: queueRepo, stateRepo: stateRepo, realtimeRepo: storesqlite.NewRealtimeRepository(store, time.Now), chatRepo: storesqlite.NewChatRepository(store), playlistRepo: storesqlite.NewUserPlaylistRepository(store, time.Now, nil), queue: queue, state: *state, commands: make(chan command, capacity), ctx: runtimeCtx, cancel: cancel, done: make(chan struct{}), timeout: timeout, broadcaster: broadcaster, queueVersion: 1, limits: limits, chat: chat, mutations: map[string]queueMutationRecord{}, mutationCounts: map[string]int{}, idle: idle, lastActivity: time.Now(), onClose: onClose, sessionState: "ready", sessionStateSince: time.Now(), eventWake: make(chan struct{}, 1)}
	go r.writeEventLog(store)
	if history, historyErr := queueRepo.LoadHistory(ctx, roomID, limits.HistorySize); historyErr == nil {
		r.history = history
	}
	r.controlScope = uuid.NewString()
	if r.state.CurrentMusic != nil {
		r.currentQueueID = uuid.NewString()
		r.sessionState = "ready"
		r.sessionStateSince = time.Now()
	}
	r.controlMutations = make(map[controlKey]controlRecord)
	r.controlCounts = make(map[string]int)
	go r.loop()
	return r, nil
}

func (r *RoomRuntime) Execute(ctx context.Context, apply func(*RoomRuntime, context.Context) (any, error)) (any, error) {
	request := command{ctx: ctx, apply: apply, result: make(chan commandResult, 1)}
	timer := time.NewTimer(r.timeout)
	defer timer.Stop()
	select {
	case r.commands <- request:
	case <-timer.C:
		return nil, ErrCommandQueueFull
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-r.done:
		return nil, context.Canceled
	}
	select {
	case result := <-request.result:
		return result.value, result.err
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-r.done:
		return nil, context.Canceled
	}
}

func (r *RoomRuntime) Snapshot(ctx context.Context) (map[string]any, error) {
	value, err := r.Execute(ctx, func(runtime *RoomRuntime, ctx context.Context) (any, error) { return runtime.snapshot(), nil })
	if err != nil {
		return nil, err
	}
	return value.(map[string]any), nil
}

func (r *RoomRuntime) Attach(ctx context.Context) error {
	_, err := r.Execute(ctx, func(runtime *RoomRuntime, ctx context.Context) (any, error) {
		runtime.clients++
		return nil, nil
	})
	return err
}

func (r *RoomRuntime) Detach(ctx context.Context) error {
	_, err := r.Execute(ctx, func(runtime *RoomRuntime, ctx context.Context) (any, error) {
		if runtime.clients > 0 {
			runtime.clients--
		}
		return nil, nil
	})
	return err
}

func (r *RoomRuntime) Enqueue(ctx context.Context, session account.Session, music storesqlite.Music, mutationID string) (storesqlite.QueueItem, error) {
	value, err := r.executeQueue(ctx, mutationID, []any{"enqueue", session.PublicID, music.Platform, music.ID}, func(runtime *RoomRuntime, ctx context.Context) (any, error) {
		item := storesqlite.QueueItem{QueueID: uuid.NewString(), Music: music, EnqueuedBy: storesqlite.UserSummary{PublicID: session.PublicID, Name: session.DisplayName, Guest: session.Guest}, Status: "PENDING"}
		next := append(slices.Clone(runtime.queue), item)
		nextState := clonePlaybackState(runtime.state)
		startedID := startFirst(&nextState, &next)
		started := startedID != ""
		if err := runtime.checkQueueCapacity(session.PublicID, next); err != nil {
			return nil, err
		}
		if started {
			if err := runtime.realtimeRepo.CommitRoomState(ctx, runtime.roomID, next, nextState, nil); err != nil {
				return nil, err
			}
		} else if err := runtime.queueRepo.SynchronizeQueue(ctx, runtime.roomID, next); err != nil {
			return nil, err
		}
		if started {
			runtime.currentQueueID = startedID
			runtime.sessionState = "ready"
			runtime.sessionStateSince = time.Now()
			runtime.clockOwner = ""
		}
		runtime.queue = next
		runtime.state = nextState
		runtime.queueVersion++
		pending := []storesqlite.QueueItem{item}
		if item.QueueID == startedID {
			pending = []storesqlite.QueueItem{}
		}
		runtime.broadcast("queue.patch", map[string]any{"operation": "append", "queueEpoch": runtime.controlScope, "queueVersion": runtime.queueVersion, "items": pending, "queueIds": consumedQueueIDs(startedID)})
		runtime.broadcastState()
		return item, nil
	})
	if err != nil {
		return storesqlite.QueueItem{}, err
	}
	return value.(storesqlite.QueueItem), nil
}

// EnqueueMany admits a whole batch or rejects it without changes. Collection
// identifies the authorized source request so metadata and playlist edits do not
// turn an identical retry into a new mutation.
func (r *RoomRuntime) EnqueueMany(ctx context.Context, session account.Session, musics []storesqlite.Music, mutationID string, collection ...string) ([]storesqlite.QueueItem, error) {
	fingerprint := []any{"enqueue-many", session.PublicID, musics}
	if len(collection) > 0 {
		fingerprint = []any{"enqueue-collection", session.PublicID, collection}
	}
	value, err := r.executeQueue(ctx, mutationID, fingerprint, func(runtime *RoomRuntime, ctx context.Context) (any, error) {
		if len(musics) == 0 {
			return nil, errors.New("playlist is empty")
		}
		items := make([]storesqlite.QueueItem, 0, len(musics))
		for _, music := range musics {
			items = append(items, storesqlite.QueueItem{QueueID: uuid.NewString(), Music: music, EnqueuedBy: storesqlite.UserSummary{PublicID: session.PublicID, Name: session.DisplayName, Guest: session.Guest}, Status: "PENDING"})
		}
		next := append(slices.Clone(runtime.queue), items...)
		nextState := clonePlaybackState(runtime.state)
		startedID := startFirst(&nextState, &next)
		started := startedID != ""
		if err := runtime.checkQueueCapacity(session.PublicID, next); err != nil {
			return nil, err
		}
		if started {
			if err := runtime.realtimeRepo.CommitRoomState(ctx, runtime.roomID, next, nextState, nil); err != nil {
				return nil, err
			}
		} else if err := runtime.queueRepo.SynchronizeQueue(ctx, runtime.roomID, next); err != nil {
			return nil, err
		}
		if started {
			runtime.currentQueueID = startedID
			runtime.sessionState = "ready"
			runtime.sessionStateSince = time.Now()
			runtime.clockOwner = ""
		}
		runtime.queue = next
		runtime.state = nextState
		runtime.queueVersion++
		pending := slices.DeleteFunc(slices.Clone(items), func(item storesqlite.QueueItem) bool { return item.QueueID == startedID })
		runtime.broadcast("queue.patch", map[string]any{"operation": "append", "queueEpoch": runtime.controlScope, "queueVersion": runtime.queueVersion, "items": pending, "queueIds": consumedQueueIDs(startedID)})
		runtime.broadcastState()
		return items, nil
	})
	if err != nil {
		return nil, err
	}
	return value.([]storesqlite.QueueItem), nil
}

func (r *RoomRuntime) Remove(ctx context.Context, ids []string, mutationID string) error {
	_, err := r.executeQueue(ctx, mutationID, []any{"remove", ids}, func(runtime *RoomRuntime, ctx context.Context) (any, error) {
		remove := map[string]struct{}{}
		for _, id := range ids {
			remove[id] = struct{}{}
		}
		next := slices.DeleteFunc(slices.Clone(runtime.queue), func(item storesqlite.QueueItem) bool { _, ok := remove[item.QueueID]; return ok })
		if len(next) == len(runtime.queue) {
			return nil, ErrStaleQueue
		}
		if err := runtime.queueRepo.SynchronizeQueue(ctx, runtime.roomID, next); err != nil {
			return nil, err
		}
		runtime.queue = next
		runtime.queueVersion++
		runtime.broadcast("queue.patch", map[string]any{"operation": "remove", "queueEpoch": runtime.controlScope, "queueVersion": runtime.queueVersion, "queueIds": ids})
		runtime.broadcastState()
		return nil, nil
	})
	return err
}

func (r *RoomRuntime) Top(ctx context.Context, ids []string, mutationID string) error {
	_, err := r.executeQueue(ctx, mutationID, []any{"top", ids}, func(runtime *RoomRuntime, ctx context.Context) (any, error) {
		selected := []storesqlite.QueueItem{}
		rest := []storesqlite.QueueItem{}
		wanted := map[string]struct{}{}
		for _, id := range ids {
			wanted[id] = struct{}{}
		}
		for _, item := range runtime.queue {
			if _, ok := wanted[item.QueueID]; ok {
				selected = append(selected, item)
			} else {
				rest = append(rest, item)
			}
		}
		if len(selected) == 0 {
			return nil, ErrStaleQueue
		}
		var priority int64
		for _, item := range runtime.queue {
			priority = max(priority, item.PlaybackPriority)
		}
		for i := range selected {
			selected[i].PlaybackPriority = priority + int64(len(selected)-i)
		}
		next := append(selected, rest...)
		if err := runtime.queueRepo.SynchronizeQueue(ctx, runtime.roomID, next); err != nil {
			return nil, err
		}
		runtime.queue = next
		runtime.queueVersion++
		runtime.broadcast("queue.patch", map[string]any{"operation": "snapshot", "queueEpoch": runtime.controlScope, "queueVersion": runtime.queueVersion, "queue": next})
		runtime.broadcastState()
		return nil, nil
	})
	return err
}

func (r *RoomRuntime) Reorder(ctx context.Context, oldIndex, newIndex int, mutationID string) error {
	_, err := r.executeQueue(ctx, mutationID, []any{"reorder", oldIndex, newIndex}, func(runtime *RoomRuntime, ctx context.Context) (any, error) {
		if oldIndex < 0 || oldIndex >= len(runtime.queue) || newIndex < 0 || newIndex >= len(runtime.queue) {
			return nil, ErrStaleQueue
		}
		next := slices.Clone(runtime.queue)
		item := next[oldIndex]
		next = slices.Delete(next, oldIndex, oldIndex+1)
		next = slices.Insert(next, newIndex, item)
		if err := runtime.queueRepo.SynchronizeQueue(ctx, runtime.roomID, next); err != nil {
			return nil, err
		}
		runtime.queue = next
		runtime.queueVersion++
		runtime.broadcast("queue.patch", map[string]any{"operation": "snapshot", "queueEpoch": runtime.controlScope, "queueVersion": runtime.queueVersion, "queue": next})
		runtime.broadcastState()
		return nil, nil
	})
	return err
}

func (r *RoomRuntime) ReorderByID(ctx context.Context, queueID, targetQueueID, placement, mutationID string) error {
	_, err := r.executeQueue(ctx, mutationID, []any{"reorder-id", queueID, targetQueueID, placement}, func(runtime *RoomRuntime, ctx context.Context) (any, error) {
		if queueID == "" || targetQueueID == "" || queueID == targetQueueID {
			return nil, ErrStaleQueue
		}
		oldIndex, targetIndex := -1, -1
		for index, item := range runtime.queue {
			if item.QueueID == queueID {
				oldIndex = index
			}
			if item.QueueID == targetQueueID {
				targetIndex = index
			}
		}
		if oldIndex < 0 || targetIndex < 0 {
			return nil, ErrStaleQueue
		}
		next := slices.Clone(runtime.queue)
		item := next[oldIndex]
		next = slices.Delete(next, oldIndex, oldIndex+1)
		if oldIndex < targetIndex {
			targetIndex--
		}
		insertIndex := targetIndex
		if strings.EqualFold(placement, "after") {
			insertIndex++
		}
		insertIndex = min(max(0, insertIndex), len(next))
		if insertIndex == oldIndex {
			return nil, ErrStaleQueue
		}
		next = slices.Insert(next, insertIndex, item)
		if err := runtime.queueRepo.SynchronizeQueue(ctx, runtime.roomID, next); err != nil {
			return nil, err
		}
		runtime.queue = next
		runtime.queueVersion++
		runtime.broadcast("queue.patch", map[string]any{"operation": "move", "queueEpoch": runtime.controlScope, "queueVersion": runtime.queueVersion, "queueId": queueID, "targetQueueId": targetQueueID, "position": placement})
		runtime.broadcastState()
		return nil, nil
	})
	return err
}

func (r *RoomRuntime) TogglePause(ctx context.Context, mutation ...ControlMutation) (ControlResult, error) {
	return r.TogglePauseAs(ctx, false, mutation...)
}

// TogglePauseAs accepts administrator authority from the authenticated session.
func (r *RoomRuntime) TogglePauseAs(ctx context.Context, admin bool, mutation ...ControlMutation) (ControlResult, error) {
	value, err := r.executeControl(ctx, "control.toggle-pause", 0, nil, mutation, func(runtime *RoomRuntime, ctx context.Context) (any, error) {
		if !admin && runtime.state.PauseLocked && !runtime.state.Paused {
			return nil, ErrControlLocked
		}
		if runtime.state.CurrentMusic == nil {
			if len(runtime.queue) == 0 {
				return nil, ErrControlDenied
			}
			if err := runtime.advance(ctx); err != nil {
				return nil, err
			}
			return ControlResult{Applied: true, Committed: runtime.watermark()}, nil
		}
		if err := runtime.mutateStateDirect(ctx, func(s *storesqlite.PlaybackState) {
			now := time.Now().UnixMilli()
			if !s.Paused {
				s.PositionAnchor = position(*s, now)
			}
			s.Paused = !s.Paused
			s.TimestampAnchor = now
			s.PositionUpdatedAt = now
		}); err != nil {
			return nil, err
		}
		runtime.sessionStateSince = time.Now()
		return ControlResult{Applied: true, Committed: runtime.watermark()}, nil
	})
	if err != nil {
		return controlResultValue(value), err
	}
	return value.(ControlResult), nil
}

func (r *RoomRuntime) ToggleShuffle(ctx context.Context, mutation ...ControlMutation) (ControlResult, error) {
	return r.ToggleShuffleAs(ctx, false, mutation...)
}

// ToggleShuffleAs allows administrators to bypass the room shuffle lock.
func (r *RoomRuntime) ToggleShuffleAs(ctx context.Context, admin bool, mutation ...ControlMutation) (ControlResult, error) {
	return r.mutateStateChecked(ctx, "control.toggle-shuffle", 0, nil, mutation, func(s storesqlite.PlaybackState) error {
		if !admin && s.ShuffleLocked {
			return ErrControlLocked
		}
		return nil
	}, func(s *storesqlite.PlaybackState) { s.Shuffle = !s.Shuffle })
}
func (r *RoomRuntime) Seek(ctx context.Context, requested int64, publicID string, _ bool, expectedEpoch *int64, mutation ...ControlMutation) (ControlResult, error) {
	return r.mutateStateChecked(WithControlActor(ctx, publicID), "control.seek", requested, expectedEpoch, mutation, func(s storesqlite.PlaybackState) error {
		if expectedEpoch != nil && *expectedEpoch != s.PlayEpoch {
			return ErrPreconditionFailed
		}
		if s.CurrentMusic == nil {
			return ErrControlDenied
		}
		return nil
	}, func(s *storesqlite.PlaybackState) {
		positionMS := max(0, requested)
		if s.CurrentMusic != nil && s.CurrentMusic.Duration > 0 {
			positionMS = min(positionMS, s.CurrentMusic.Duration)
		}
		s.PositionAnchor = positionMS
		s.TimestampAnchor = time.Now().UnixMilli()
		s.PositionUpdatedAt = s.TimestampAnchor
		s.PlayEpoch++
	})
}
func (r *RoomRuntime) Like(ctx context.Context, publicID string, expectedEpoch *int64, mutation ...ControlMutation) (ControlResult, error) {
	value, err := r.executeControl(ctx, "control.like", 0, expectedEpoch, mutation, func(runtime *RoomRuntime, ctx context.Context) (any, error) {
		if expectedEpoch != nil && *expectedEpoch != runtime.state.PlayEpoch {
			return nil, ErrPreconditionFailed
		}
		if runtime.state.CurrentMusic == nil {
			return ControlResult{Committed: runtime.watermark()}, nil
		}
		if _, exists := runtime.state.LikedUserIDs[publicID]; exists {
			return ControlResult{Committed: runtime.watermark()}, nil
		}
		music := playedMusic(runtime.state.CurrentMusic)
		next := clonePlaybackState(runtime.state)
		if next.LikedUserIDs == nil {
			next.LikedUserIDs = map[string]struct{}{}
		}
		next.LikedUserIDs[publicID] = struct{}{}
		next.LikeMarkers = append(next.LikeMarkers, position(next, time.Now().UnixMilli()))
		next.StateVersion++
		next.LastPersistedAt = time.Now().UnixMilli()
		if err := runtime.realtimeRepo.CommitRoomLike(ctx, next, publicID, music, true); err != nil {
			return nil, err
		}
		runtime.state = next
		runtime.broadcastState()
		return ControlResult{Applied: true, Committed: runtime.watermark()}, nil
	})
	if err != nil {
		return controlResultValue(value), err
	}
	return value.(ControlResult), nil
}

// Unlike withdraws only the caller's like, so another member's stays. A client
// that never liked the track gets the same noop as a repeated like: canceling
// twice is as safe as liking twice.
func (r *RoomRuntime) Unlike(ctx context.Context, publicID string, expectedEpoch *int64, mutation ...ControlMutation) (ControlResult, error) {
	value, err := r.executeControl(ctx, "control.unlike", 0, expectedEpoch, mutation, func(runtime *RoomRuntime, ctx context.Context) (any, error) {
		if expectedEpoch != nil && *expectedEpoch != runtime.state.PlayEpoch {
			return nil, ErrPreconditionFailed
		}
		if runtime.state.CurrentMusic == nil {
			return ControlResult{Committed: runtime.watermark()}, nil
		}
		if _, exists := runtime.state.LikedUserIDs[publicID]; !exists {
			return ControlResult{Committed: runtime.watermark()}, nil
		}
		music := playedMusic(runtime.state.CurrentMusic)
		next := clonePlaybackState(runtime.state)
		delete(next.LikedUserIDs, publicID)
		// LikeMarkers carries one entry per outstanding like without saying who
		// made which one, so the newest marker leaves with the newest like.
		if len(next.LikeMarkers) > 0 {
			next.LikeMarkers = next.LikeMarkers[:len(next.LikeMarkers)-1]
		}
		next.StateVersion++
		next.LastPersistedAt = time.Now().UnixMilli()
		if err := runtime.realtimeRepo.CommitRoomLike(ctx, next, publicID, music, false); err != nil {
			return nil, err
		}
		runtime.state = next
		runtime.broadcastState()
		return ControlResult{Applied: true, Committed: runtime.watermark()}, nil
	})
	if err != nil {
		return controlResultValue(value), err
	}
	return value.(ControlResult), nil
}

const (
	// A member's own liked list is one row of the existing `user_playlist` system
	// playlist table, the same one /api/me/liked-songs maintains for the web.
	LikedSongsSystemKey = storesqlite.LikedSongsSystemKey
	LikedSongsName      = storesqlite.LikedSongsName
)

func playedMusic(music *storesqlite.PlayableMusic) storesqlite.Music {
	return storesqlite.Music{ID: music.ID, Name: music.Name, Artists: slices.Clone(music.Artists), Duration: music.Duration, Platform: music.Platform, CoverURL: music.CoverURL}
}

// EnsureLikedSongs returns the member's own liked list, creating it on first use.
func (r *RoomRuntime) EnsureLikedSongs(ctx context.Context, publicID string) (storesqlite.Playlist, error) {
	playlist, err := r.playlistRepo.FindSystemPlaylist(ctx, publicID, LikedSongsSystemKey)
	if err == nil {
		return *playlist, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return storesqlite.Playlist{}, err
	}
	return r.playlistRepo.CreateSystemPlaylist(ctx, publicID, LikedSongsName, LikedSongsSystemKey)
}

func (r *RoomRuntime) Next(ctx context.Context, mutation ...ControlMutation) (ControlResult, error) {
	return r.NextAs(ctx, "", true, mutation...)
}

// NextAs lets an administrator skip any track while a DJ can skip only the
// track they enqueued. Authorization runs inside the room actor against its
// current committed playback state.
func (r *RoomRuntime) NextAs(ctx context.Context, publicID string, admin bool, mutation ...ControlMutation) (ControlResult, error) {
	return r.NextFor(ctx, publicID, admin, nil, "", mutation...)
}

func (r *RoomRuntime) NextFor(ctx context.Context, publicID string, admin bool, epoch *int64, queueID string, mutation ...ControlMutation) (ControlResult, error) {
	value, err := r.executeControl(ctx, "control.next", 0, epoch, mutation, func(runtime *RoomRuntime, ctx context.Context) (any, error) {
		if epoch != nil && (*epoch != runtime.state.PlayEpoch || queueID != runtime.currentQueueID) {
			return ControlResult{Committed: runtime.watermark()}, nil
		}
		if !admin && (runtime.state.CurrentEnqueuerID == nil || *runtime.state.CurrentEnqueuerID != publicID) {
			return nil, ErrForbidden
		}
		if !admin && runtime.state.SkipLocked {
			return nil, ErrControlLocked
		}
		if err := runtime.advance(ctx); err != nil {
			return nil, err
		}
		return ControlResult{Applied: true, Committed: runtime.watermark()}, nil
	})
	if err != nil {
		return controlResultValue(value), err
	}
	return value.(ControlResult), nil
}

// Previous replays the newest not-yet-walked-back history entry without
// touching the queue. Walking back is reset by any advance (next or natural
// end), which starts taking from the queue again; the walked-past entries
// remain reachable as history. Shuffle only affects advance's queue pick.
func (r *RoomRuntime) Previous(ctx context.Context, expectedEpoch *int64, mutation ...ControlMutation) (ControlResult, error) {
	return r.PreviousAs(ctx, false, expectedEpoch, mutation...)
}

// PreviousAs allows administrators to bypass skip locks; epoch checks still apply.
func (r *RoomRuntime) PreviousAs(ctx context.Context, admin bool, expectedEpoch *int64, mutation ...ControlMutation) (ControlResult, error) {
	return r.PreviousFor(ctx, admin, expectedEpoch, "", mutation...)
}

func (r *RoomRuntime) PreviousFor(ctx context.Context, admin bool, expectedEpoch *int64, queueID string, mutation ...ControlMutation) (ControlResult, error) {
	value, err := r.executeControl(ctx, "control.previous", 0, expectedEpoch, mutation, func(runtime *RoomRuntime, ctx context.Context) (any, error) {
		if expectedEpoch != nil && (*expectedEpoch != runtime.state.PlayEpoch || (queueID != "" && queueID != runtime.currentQueueID)) {
			return nil, ErrPreconditionFailed
		}
		if !admin && runtime.state.SkipLocked {
			return nil, ErrControlLocked
		}
		if runtime.state.CurrentMusic == nil {
			return nil, ErrControlDenied
		}
		if runtime.historyCursor() == 0 {
			return nil, ErrNoHistory
		}
		entry := runtime.history[runtime.historySkipped]
		if err := runtime.mutateStateDirect(ctx, func(s *storesqlite.PlaybackState) {
			now := time.Now().UnixMilli()
			s.CurrentMusic = playableMusic(entry.Music)
			s.CurrentEnqueuerID = entry.EnqueuerPublicID
			s.CurrentEnqueuerName = nil
			s.Paused = false
			s.PositionAnchor = 0
			s.TimestampAnchor = now
			s.PositionUpdatedAt = now
			s.PlayEpoch++
			s.LikedUserIDs = map[string]struct{}{}
			s.LikeMarkers = []int64{}
		}); err != nil {
			return nil, err
		}
		runtime.historySkipped++
		return ControlResult{Applied: true, Committed: runtime.watermark()}, nil
	})
	if err != nil {
		return controlResultValue(value), err
	}
	return value.(ControlResult), nil
}

func (r *RoomRuntime) TogglePauseFor(ctx context.Context, admin bool, context PlaybackContext, mutation ...ControlMutation) (ControlResult, error) {
	return r.mutateStateChecked(ctx, "control.toggle-pause", context.PositionMS, &context.PlayEpoch, mutation, func(s storesqlite.PlaybackState) error {
		if context.PlayEpoch != r.state.PlayEpoch {
			return ErrPreconditionFailed
		}
		if context.QueueID != r.currentQueueID {
			return ErrPreconditionFailed
		}
		if !admin && s.PauseLocked && !s.Paused {
			return ErrControlLocked
		}
		if s.CurrentMusic == nil {
			return ErrControlDenied
		}
		return nil
	}, func(s *storesqlite.PlaybackState) {
		now := time.Now().UnixMilli()
		if !s.Paused {
			s.PositionAnchor = r.currentPosition(now)
		}
		s.Paused = !s.Paused
		s.TimestampAnchor = now
		s.PositionUpdatedAt = now
	})
}

func (r *RoomRuntime) SeekFor(ctx context.Context, requested int64, admin bool, context PlaybackContext, mutation ...ControlMutation) (ControlResult, error) {
	return r.mutateStateChecked(ctx, "control.seek", requested, &context.PlayEpoch, mutation, func(s storesqlite.PlaybackState) error {
		if context.PlayEpoch != r.state.PlayEpoch {
			return ErrPreconditionFailed
		}
		if context.QueueID != r.currentQueueID {
			return ErrPreconditionFailed
		}
		if s.CurrentMusic == nil {
			return ErrControlDenied
		}
		return nil
	}, func(s *storesqlite.PlaybackState) {
		positionMS := max(0, requested)
		if s.CurrentMusic != nil && s.CurrentMusic.Duration > 0 {
			positionMS = min(positionMS, s.CurrentMusic.Duration)
		}
		s.PositionAnchor = positionMS
		s.TimestampAnchor = time.Now().UnixMilli()
		s.PositionUpdatedAt = s.TimestampAnchor
		// Every seek bumps the epoch so every client re-seeks. Admin status only
		// decides whether a locked control is allowed, not whether the jump is real.
		s.PlayEpoch++
	})
}

func (r *RoomRuntime) historyCursor() int {
	return max(0, len(r.history)-r.historySkipped)
}

func (r *RoomRuntime) Clear(ctx context.Context, mutationID string) error {
	_, err := r.executeQueue(ctx, mutationID, []any{"clear"}, func(runtime *RoomRuntime, ctx context.Context) (any, error) {
		if len(runtime.queue) == 0 {
			return nil, ErrStaleQueue
		}
		next := []storesqlite.QueueItem{}
		if err := runtime.queueRepo.SynchronizeQueue(ctx, runtime.roomID, next); err != nil {
			return nil, err
		}
		runtime.queue = next
		runtime.queueVersion++
		runtime.broadcast("queue.patch", map[string]any{"operation": "clear", "queueEpoch": runtime.controlScope, "queueVersion": runtime.queueVersion, "queue": next})
		runtime.broadcastState()
		return nil, nil
	})
	return err
}

// ReloadQueue replaces the runtime queue with the persisted one. The queue version only moves
// forward: clients treat a version of zero or less as invalid, so it is never rewritten.
func (r *RoomRuntime) ReloadQueue(ctx context.Context, removedTrackID string) error {
	_, err := r.Execute(ctx, func(runtime *RoomRuntime, ctx context.Context) (any, error) {
		next, err := runtime.queueRepo.LoadQueue(ctx, runtime.roomID)
		if err != nil {
			return nil, err
		}
		if removedTrackID != "" {
			history := make([]storesqlite.HistoryEntry, 0, len(runtime.history))
			skipped := 0
			for index, entry := range runtime.history {
				if entry.Music.Platform == "local" && entry.Music.ID == removedTrackID {
					continue
				}
				history = append(history, entry)
				if index < runtime.historySkipped {
					skipped++
				}
			}
			runtime.history, runtime.historySkipped = history, skipped
		}
		if removedTrackID != "" && runtime.isCurrentTrack(removedTrackID) {
			return nil, runtime.dropCurrentTrack(ctx, next)
		}
		runtime.queue = next
		runtime.queueVersion++
		runtime.broadcast("queue.patch", map[string]any{"operation": "snapshot", "queueEpoch": runtime.controlScope, "queueVersion": runtime.queueVersion, "queue": next})
		runtime.broadcastState()
		return nil, nil
	})
	return err
}

// isCurrentTrack reports whether the named local track is the room's current music.
func (r *RoomRuntime) isCurrentTrack(trackID string) bool {
	current := r.state.CurrentMusic
	return current != nil && current.Platform == "local" && current.ID == trackID
}

// dropCurrentTrack forgets a current track whose file is gone and starts the next queued item. Queue
// and playback state land in one write, and no history entry is written for the removed track: the
// point of the reload is that a deleted row is never referenced again.
func (r *RoomRuntime) dropCurrentTrack(ctx context.Context, queue []storesqlite.QueueItem) error {
	nextQueue := slices.Clone(queue)
	nextState := clonePlaybackState(r.state)
	nextQueueID := ""
	nextState.CurrentMusic = nil
	nextState.CurrentEnqueuerID = nil
	nextState.CurrentEnqueuerName = nil
	nextState.LikedUserIDs = map[string]struct{}{}
	nextState.LikeMarkers = []int64{}
	// A removal ignores shuffle but still honors explicitly prioritized tracks.
	skipShuffle := nextState.Shuffle
	nextState.Shuffle = false
	if len(nextQueue) > 0 {
		item := takeNext(&nextState, &nextQueue)
		nextQueueID = item.QueueID
		nextState.CurrentMusic = playableMusic(item.Music)
		nextState.CurrentEnqueuerID = &item.EnqueuedBy.PublicID
		nextState.CurrentEnqueuerName = &item.EnqueuedBy.Name
		nextState.Paused = false
		nextState.PlayEpoch++
	} else {
		nextState.Paused = true
	}
	nextState.Shuffle = skipShuffle
	now := time.Now().UnixMilli()
	nextState.PositionAnchor = 0
	nextState.TimestampAnchor = now
	nextState.PositionUpdatedAt = now
	nextState.StateVersion++
	nextState.LastPersistedAt = now
	if err := r.realtimeRepo.CommitRoomState(ctx, r.roomID, nextQueue, nextState, nil); err != nil {
		return err
	}
	r.queue = nextQueue
	r.state = nextState
	if nextQueueID != "" && nextState.CurrentMusic != nil {
		r.currentQueueID = nextQueueID
		r.sessionState = "ready"
		r.sessionStateSince = time.Now()
		r.clockOwner = ""
	} else {
		r.currentQueueID = ""
		r.sessionState = "ended"
		r.sessionStateSince = time.Now()
		r.clockOwner = ""
	}
	r.queueVersion++
	r.broadcast("queue.patch", map[string]any{"operation": "snapshot", "queueEpoch": r.controlScope, "queueVersion": r.queueVersion, "queue": nextQueue})
	r.broadcastState()
	return nil
}

func (r *RoomRuntime) advance(ctx context.Context) error {
	nextQueue := slices.Clone(r.queue)
	nextState := clonePlaybackState(r.state)
	nextQueueID, nextSessionState := "", "ended"
	var history *storesqlite.HistoryEntry
	removed := []string{}
	if r.state.CurrentMusic != nil {
		music := storesqlite.Music{ID: r.state.CurrentMusic.ID, Name: r.state.CurrentMusic.Name, Artists: r.state.CurrentMusic.Artists, Duration: r.state.CurrentMusic.Duration, Platform: r.state.CurrentMusic.Platform, CoverURL: r.state.CurrentMusic.CoverURL}
		history = &storesqlite.HistoryEntry{ID: uuid.NewString(), RoomID: r.roomID, Music: music, EnqueuerPublicID: r.state.CurrentEnqueuerID, PlayedAt: time.Now().UnixMilli()}
	}
	if len(nextQueue) == 0 {
		nextState.CurrentMusic = nil
		nextState.CurrentEnqueuerID = nil
		nextState.CurrentEnqueuerName = nil
		nextState.Paused = true
	} else {
		item := takeNext(&nextState, &nextQueue)
		nextQueueID, nextSessionState = item.QueueID, "ready"
		removed = append(removed, item.QueueID)
		nextState.CurrentMusic = playableMusic(item.Music)
		nextState.CurrentEnqueuerID = &item.EnqueuedBy.PublicID
		nextState.CurrentEnqueuerName = &item.EnqueuedBy.Name
		nextState.Paused = false
		nextState.PlayEpoch++
	}
	now := time.Now().UnixMilli()
	nextState.PositionAnchor = 0
	nextState.TimestampAnchor = now
	nextState.PositionUpdatedAt = now
	// Likes are statements about the track that was playing, so a new track starts
	// with an empty set; without this the heart never goes out and `likedUserIds`
	// keeps answering "someone liked something this session".
	nextState.LikedUserIDs = map[string]struct{}{}
	nextState.LikeMarkers = []int64{}
	nextState.StateVersion++
	nextState.LastPersistedAt = now
	if err := r.realtimeRepo.CommitRoomState(ctx, r.roomID, nextQueue, nextState, history); err != nil {
		r.advancePending = false
		return err
	}
	r.queue = nextQueue
	r.state = nextState
	r.currentQueueID, r.sessionState = nextQueueID, nextSessionState
	r.sessionStateSince = time.Now()
	r.clockOwner = ""
	r.advancePending = false
	if history != nil {
		r.history = append([]storesqlite.HistoryEntry{*history}, r.history...)
		if len(r.history) > r.limits.HistorySize {
			r.history = r.history[:r.limits.HistorySize]
		}
	}
	r.historySkipped = 0
	r.queueVersion++
	r.broadcast("queue.patch", map[string]any{"operation": "remove", "queueEpoch": r.controlScope, "queueVersion": r.queueVersion, "queueIds": removed})
	r.broadcastState()
	r.logEvent("playback.advanced", r.currentQueueID, "applied", "", nil)
	return nil
}

func (r *RoomRuntime) AppendChat(ctx context.Context, session account.Session, content string, public bool) (storesqlite.ChatMessage, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return storesqlite.ChatMessage{}, errors.New("chat message is empty")
	}
	if len([]rune(content)) > r.chat.limits.MaxMessageLength {
		return storesqlite.ChatMessage{}, errors.New("chat message is too long")
	}
	stream := r.roomID
	if public {
		stream = ""
	}
	key := chatKey{stream: stream, user: session.PublicID}
	reservation, err := r.chat.reserve(key)
	if err != nil {
		return storesqlite.ChatMessage{}, err
	}
	message := storesqlite.ChatMessage{ID: uuid.NewString(), UserID: session.PublicID, UserName: session.DisplayName, Content: content, Timestamp: reservation.UnixMilli(), Type: "CHAT"}
	var room *string
	messageType := "public-chat.message"
	if !public {
		room = &r.roomID
		messageType = "chat.message"
	}
	if err := r.chatRepo.AppendMessageRetaining(ctx, room, message, r.chat.limits.MaxHistorySize); err != nil {
		if ctx.Err() == nil && !errors.Is(err, storesqlite.ErrClosed) {
			r.chat.release(key, reservation)
		}
		return storesqlite.ChatMessage{}, err
	}
	if public {
		r.broadcaster.BroadcastAll(messageType, message)
	} else {
		r.broadcast(messageType, message)
	}
	return message, nil
}

func (r *RoomRuntime) ChatHistory(ctx context.Context, offset, limit int, public bool) ([]storesqlite.ChatMessage, error) {
	var room *string
	if !public {
		room = &r.roomID
	}
	return r.chatRepo.FetchMessages(ctx, room, max(0, offset), min(100, max(1, limit)))
}

const (
	defaultHistoryPageLimit = 50
	maxHistoryPageLimit     = 200
)

// PlaybackHistoryItem is one played track. `music` is the metadata the queue item carried (the
// same shape as QueueItem.music); no stream URL is persisted with it, so replaying an entry means
// enqueuing platform + music id again rather than opening an old address.
type PlaybackHistoryItem struct {
	ID               string            `json:"id"`
	Music            storesqlite.Music `json:"music"`
	EnqueuerPublicID *string           `json:"enqueuerPublicId"`
	EnqueuerName     *string           `json:"enqueuerName"`
	PlayedAt         int64             `json:"playedAt"`
}

// PlaybackHistoryPage answers `history.list`: the whole session, newest first, one page at a time.
// Total counts every row the room ever wrote, which is deliberately larger than the
// configured history window the runtime keeps for control.previous.
type PlaybackHistoryPage struct {
	RoomID string                `json:"roomId"`
	Total  int                   `json:"total"`
	Offset int                   `json:"offset"`
	Items  []PlaybackHistoryItem `json:"items"`
}

// PlaybackHistory reads straight from the store instead of queueing behind playback: it is a read
// for whoever already got into the room, so it stays out of the mutation and idempotency channels.
func (r *RoomRuntime) PlaybackHistory(ctx context.Context, offset, limit int) (PlaybackHistoryPage, error) {
	page := PlaybackHistoryPage{RoomID: r.roomID, Offset: max(0, offset), Items: []PlaybackHistoryItem{}}
	if limit <= 0 {
		limit = defaultHistoryPageLimit
	}
	limit = min(maxHistoryPageLimit, limit)
	total, err := r.queueRepo.CountHistory(ctx, r.roomID)
	if err != nil {
		return page, err
	}
	entries, err := r.queueRepo.LoadHistoryPage(ctx, r.roomID, page.Offset, limit)
	if err != nil {
		return page, err
	}
	page.Total = total
	for _, entry := range entries {
		page.Items = append(page.Items, PlaybackHistoryItem{ID: entry.ID, Music: entry.Music, EnqueuerPublicID: entry.EnqueuerPublicID, EnqueuerName: entry.EnqueuerName, PlayedAt: entry.PlayedAt})
	}
	return page, nil
}

func (r *RoomRuntime) mutateStateChecked(ctx context.Context, kind string, positionMS int64, expectedEpoch *int64, mutation []ControlMutation, check func(storesqlite.PlaybackState) error, mutate func(*storesqlite.PlaybackState)) (ControlResult, error) {
	value, err := r.executeControl(ctx, kind, positionMS, expectedEpoch, mutation, func(runtime *RoomRuntime, ctx context.Context) (any, error) {
		if check != nil {
			if checkErr := check(runtime.state); checkErr != nil {
				return nil, checkErr
			}
		}
		if err := runtime.mutateStateDirect(ctx, mutate); err != nil {
			return nil, err
		}
		return ControlResult{Applied: true, Committed: runtime.watermark()}, nil
	})
	if err != nil {
		return controlResultValue(value), err
	}
	return value.(ControlResult), nil
}

func (r *RoomRuntime) mutateStateDirect(ctx context.Context, mutate func(*storesqlite.PlaybackState)) error {
	nextState := clonePlaybackState(r.state)
	mutate(&nextState)
	now := time.Now().UnixMilli()
	nextState.StateVersion++
	nextState.LastPersistedAt = now
	if err := r.stateRepo.Upsert(ctx, nextState); err != nil {
		return err
	}
	r.state = nextState
	r.broadcastState()
	return nil
}

func (r *RoomRuntime) watermark() *CommittedWatermark {
	return &CommittedWatermark{StateVersion: r.state.StateVersion, PlayEpoch: r.state.PlayEpoch, QueueVersion: r.queueVersion}
}

func consumedQueueIDs(id string) []string {
	if id == "" {
		return []string{}
	}
	return []string{id}
}

// startFirst returns the queue ID consumed when starting an idle player.
func startFirst(state *storesqlite.PlaybackState, queue *[]storesqlite.QueueItem) string {
	if state.CurrentMusic != nil || len(*queue) == 0 {
		return ""
	}
	item := takeNext(state, queue)
	now := time.Now().UnixMilli()
	state.CurrentMusic = playableMusic(item.Music)
	state.CurrentEnqueuerID = &item.EnqueuedBy.PublicID
	state.CurrentEnqueuerName = &item.EnqueuedBy.Name
	state.Paused = false
	state.PositionAnchor = 0
	state.TimestampAnchor = now
	state.PositionUpdatedAt = now
	state.PlayEpoch++
	state.LikedUserIDs = map[string]struct{}{}
	state.LikeMarkers = []int64{}
	state.StateVersion++
	state.LastPersistedAt = now
	return item.QueueID
}

// takeNext consumes explicit priority before shuffle while preserving the
// visible order of the remaining queue.
func takeNext(state *storesqlite.PlaybackState, queue *[]storesqlite.QueueItem) storesqlite.QueueItem {
	index := 0
	var priority int64
	for i, item := range *queue {
		if item.PlaybackPriority > priority {
			index, priority = i, item.PlaybackPriority
		}
	}
	if priority == 0 && state.Shuffle {
		index = rand.IntN(len(*queue))
	}
	item := (*queue)[index]
	*queue = slices.Delete(*queue, index, index+1)
	return item
}

func playableMusic(music storesqlite.Music) *storesqlite.PlayableMusic {
	escapedID := url.PathEscape(music.ID)
	streamURL := ""
	switch {
	case music.Platform == "netease":
		streamURL = "/api/netease/stream/" + escapedID
	case music.Platform == "bilibili":
		streamURL = "/api/bilibili/stream/" + escapedID
	case music.Platform == "youtube":
		streamURL = "/api/youtube/stream/" + escapedID
	case music.Platform == "local":
		streamURL = "/api/local/media/" + escapedID
	case music.Platform == "navidrome":
		streamURL = "/api/navidrome/stream/" + escapedID
	case strings.HasPrefix(music.Platform, "subsonic-"):
		sourceAndRoom := strings.TrimPrefix(music.Platform, "subsonic-")
		sourceID, roomID, found := strings.Cut(sourceAndRoom, "@")
		if found && sourceID != "" && roomID != "" {
			streamURL = "/api/subsonic/" + url.PathEscape(roomID) + "/" + url.PathEscape(sourceID) + "/stream/" + escapedID
		} else if sourceID != "" {
			streamURL = "/api/subsonic/" + url.PathEscape(sourceID) + "/stream/" + escapedID
		}
	}
	return &storesqlite.PlayableMusic{ID: music.ID, Name: music.Name, Artists: slices.Clone(music.Artists), Duration: music.Duration, Platform: music.Platform, URL: streamURL, CoverURL: music.CoverURL}
}

func clonePlaybackState(state storesqlite.PlaybackState) storesqlite.PlaybackState {
	clone := state
	if state.CurrentMusic != nil {
		music := *state.CurrentMusic
		music.Artists = slices.Clone(state.CurrentMusic.Artists)
		clone.CurrentMusic = &music
	}
	clone.LikedUserIDs = map[string]struct{}{}
	for id := range state.LikedUserIDs {
		clone.LikedUserIDs[id] = struct{}{}
	}
	clone.LikeMarkers = slices.Clone(state.LikeMarkers)
	return clone
}

func position(state storesqlite.PlaybackState, now int64) int64 {
	if state.Paused || state.CurrentMusic == nil {
		return max(0, state.PositionAnchor)
	}
	return max(0, state.PositionAnchor+now-state.TimestampAnchor)
}
func (r *RoomRuntime) currentPosition(now int64) int64 {
	if r.sessionState == "ready" || r.sessionState == "stalled" || r.sessionState == "resolving" {
		return max(0, r.state.PositionAnchor)
	}
	return position(r.state, now)
}

type clientQueueItem struct {
	PlaybackPriority int64                      `json:"playbackPriority,omitempty"`
	QueueID          string                     `json:"queueId"`
	Music            *storesqlite.PlayableMusic `json:"music"`
	EnqueuedBy       storesqlite.UserSummary    `json:"enqueuedBy"`
	Status           string                     `json:"status"`
}

func clientQueue(queue []storesqlite.QueueItem) []clientQueueItem {
	items := make([]clientQueueItem, len(queue))
	for index, item := range queue {
		items[index] = clientQueueItem{PlaybackPriority: item.PlaybackPriority, QueueID: item.QueueID, Music: playableMusic(item.Music), EnqueuedBy: item.EnqueuedBy, Status: item.Status}
	}
	return items
}

func (r *RoomRuntime) snapshot() map[string]any {
	var nowPlaying any
	if r.state.CurrentMusic != nil {
		sessionState := r.sessionState
		if r.state.Paused {
			sessionState = "ready"
		}
		nowPlaying = map[string]any{
			"music":             r.state.CurrentMusic,
			"currentPosition":   r.currentPosition(time.Now().UnixMilli()),
			"enqueuedById":      r.state.CurrentEnqueuerID,
			"enqueuedByName":    r.state.CurrentEnqueuerName,
			"likedUserIds":      setValues(r.state.LikedUserIDs),
			"likeMarkers":       r.state.LikeMarkers,
			"playEpoch":         r.state.PlayEpoch,
			"positionUpdatedAt": r.state.PositionUpdatedAt,
			"sessionState":      sessionState,
			"queueId":           r.currentQueueID,
			"media":             map[string]any{"durationMs": r.state.CurrentMusic.Duration, "seekable": true},
		}
	}
	return map[string]any{"nowPlaying": nowPlaying, "queue": clientQueue(r.queue), "isShuffle": r.state.Shuffle, "isPaused": r.state.Paused, "isPauseLocked": r.state.PauseLocked, "isSkipLocked": r.state.SkipLocked, "isShuffleLocked": r.state.ShuffleLocked, "isLoading": r.state.Loading, "serverTimestamp": time.Now().UnixMilli(), "stateVersion": r.state.StateVersion, "playEpoch": r.state.PlayEpoch, "queueVersion": r.queueVersion, "queueEpoch": r.controlScope, "historyCursor": r.historyCursor(), "idempotencyScopeId": r.controlScope, "idempotencyTtlMs": ControlMutationTTL.Milliseconds()}
}
func (r *RoomRuntime) broadcastState() {
	state := r.snapshot()
	delete(state, "queue")
	r.broadcast("player.state", state)
}
func (r *RoomRuntime) broadcast(kind string, payload any) {
	if kind == "queue.patch" {
		if fields, ok := payload.(map[string]any); ok {
			for _, key := range []string{"items", "queue"} {
				if items, ok := fields[key].([]storesqlite.QueueItem); ok {
					fields[key] = clientQueue(items)
				}
			}
		}
	}
	if r.broadcaster != nil {
		r.broadcaster.BroadcastRoom(r.roomID, kind, payload)
	}
}
func (r *RoomRuntime) loop() {
	defer close(r.done)
	defer r.onClose(r)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if r.timeoutStartPending {
			r.timeoutStartPending = false
			if r.state.CurrentMusic != nil && !r.state.Paused && (r.sessionState == "ready" || r.sessionState == "stalled") {
				reason := "ready_timeout"
				if r.sessionState == "stalled" {
					reason = "stall_timeout"
				}
				ctx, cancel := context.WithTimeout(r.ctx, r.timeout)
				now := time.Now().UnixMilli()
				next := clonePlaybackState(r.state)
				next.PositionAnchor = r.currentPosition(now)
				next.TimestampAnchor = now
				next.PositionUpdatedAt = now
				next.StateVersion++
				next.LastPersistedAt = now
				if err := r.realtimeRepo.CommitRoomState(ctx, r.roomID, r.queue, next, nil); err == nil {
					r.state = next
					r.sessionState = "playing"
					r.sessionStateSince = time.Now()
					r.clockOwner = ""
					r.broadcastState()
					r.logEvent("playback.started", r.currentQueueID, "server", reason, nil)
				}
				cancel()
			}
		}
		// Process a pending advance before blocking so DB-write latency does not delay subsequent ticks.
		if r.advancePending && !time.Now().Before(r.advanceRetryAt) {
			ctx, cancel := context.WithTimeout(r.ctx, r.timeout)
			err := r.advance(ctx)
			cancel()
			if err != nil {
				r.advanceFailures++
				r.advanceRetryAt = time.Now().Add(min(30*time.Second, time.Second<<min(r.advanceFailures, 5)))
				slog.Warn("auto advance failed", "room", r.roomID, "attempt", r.advanceFailures, "error", err)
			} else {
				r.advanceFailures = 0
				r.advanceRetryAt = time.Time{}
			}
		}
		select {
		case <-r.ctx.Done():
			r.rejectPending()
			return
		case now := <-ticker.C:
			r.onTick(now)
			if r.idle > 0 && r.clients == 0 && now.Sub(r.lastActivity) >= r.idle {
				r.cancel()
			}
		case command := <-r.commands:
			if command.ctx.Err() != nil {
				command.result <- commandResult{err: command.ctx.Err()}
				continue
			}
			opCtx, cancel := context.WithTimeout(context.WithoutCancel(command.ctx), r.timeout)
			stop := context.AfterFunc(r.ctx, cancel)
			value, err := command.apply(r, opCtx)
			stop()
			cancel()
			r.lastActivity = time.Now()
			command.result <- commandResult{value: value, err: err}
			if errors.Is(err, storesqlite.ErrRoomGone) {
				r.cancel()
			}
		}
	}
}

func (r *RoomRuntime) onTick(now time.Time) {
	if r.state.CurrentMusic == nil || r.state.Paused {
		return
	}
	if (r.sessionState == "ready" && now.Sub(r.sessionStateSince) >= readyGraceMS) || (r.sessionState == "stalled" && now.Sub(r.sessionStateSince) >= stallGraceMS) {
		r.timeoutStartPending = true
		return
	}
	if r.sessionState != "playing" {
		return
	}
	current := r.currentPosition(now.UnixMilli())
	if r.state.CurrentMusic.Duration > 0 && current >= r.state.CurrentMusic.Duration+fallbackGraceMS && !r.advancePending && !now.Before(r.advanceRetryAt) {
		r.advancePending = true
	}
}

func (r *RoomRuntime) writeEventLog(store *storesqlite.Store) {
	var drained uint64
	for {
		r.eventLogMu.Lock()
		gen, pending := r.eventLogGen, len(r.eventLogQueue)
		r.eventLogMu.Unlock()
		select {
		case <-r.ctx.Done():
			// Room is closing. Drain whatever was queued, then exit even if a
			// write failed — otherwise this goroutine waits forever on a channel
			// nobody signals after the room loop has stopped.
			if pending == 0 || gen == drained {
				return
			}
			drained = gen
		case <-r.eventWake:
			drained = 0
		}
		for {
			r.eventLogMu.Lock()
			if len(r.eventLogQueue) == 0 {
				r.eventLogMu.Unlock()
				break
			}
			event := r.eventLogQueue[0]
			r.eventLogQueue = r.eventLogQueue[1:]
			r.eventLogMu.Unlock()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			_ = store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
				_, err := tx.ExecContext(ctx, `insert into playback_event_log(room_id, server_time_ms, event_type, play_epoch, queue_id, actor_public_id, result, reason) values(?,?,?,?,?,?,?,?)`, event.roomID, event.at, event.kind, event.epoch, event.queueID, event.actor, event.result, event.reason)
				if err != nil {
					return err
				}
				_, err = tx.ExecContext(ctx, `delete from playback_event_log where server_time_ms < ?`, time.Now().Add(-14*24*time.Hour).UnixMilli())
				return err
			})
			cancel()
		}
	}
}

func (r *RoomRuntime) logEvent(kind, queueID, result, reason string, actor *string) {
	r.logEventAt(kind, queueID, r.state.PlayEpoch, result, reason, actor)
}

func (r *RoomRuntime) logEventAt(kind, queueID string, epoch int64, result, reason string, actor *string) {
	event := playbackEvent{roomID: r.roomID, at: time.Now().UnixMilli(), kind: kind, epoch: epoch, queueID: queueID, actor: actor, result: result, reason: reason}
	r.eventLogMu.Lock()
	r.eventLogQueue = append(r.eventLogQueue, event)
	r.eventLogGen++
	r.eventLogMu.Unlock()
	select {
	case r.eventWake <- struct{}{}:
	default:
	}
}

type PlaybackReport struct {
	Code       string `json:"code"`
	PlayEpoch  int64  `json:"playEpoch"`
	QueueID    string `json:"queueId"`
	PositionMS int64  `json:"positionMs"`
	ClientTime int64  `json:"clientTime"`
	DurationMS int64  `json:"durationMs"`
}

type PlaybackContext struct {
	PlayEpoch    int64
	QueueID      string
	StateVersion int64
	PositionMS   int64
	ClientTime   int64
}

func (r *RoomRuntime) ReportPlayback(ctx context.Context, kind string, report PlaybackReport, actor *string, connectionID ...string) error {
	_, err := r.Execute(ctx, func(runtime *RoomRuntime, ctx context.Context) (any, error) {
		if report.PlayEpoch != runtime.state.PlayEpoch || report.QueueID == "" || report.QueueID != runtime.currentQueueID {
			return nil, nil
		}
		now := time.Now().UnixMilli()
		connection := ""
		if len(connectionID) > 0 {
			connection = connectionID[0]
		}
		nextState := clonePlaybackState(runtime.state)
		nextSessionState := runtime.sessionState
		switch kind {
		case "playback.error":
			if runtime.state.CurrentMusic == nil || runtime.failedEpoch == report.PlayEpoch || (runtime.sessionState != "ready" && (connection == "" || runtime.clockOwner != connection)) {
				runtime.logEvent(kind, report.QueueID, "ignored", "not_owner_or_state", actor)
				return nil, nil
			}
			if report.Code == "retries-exhausted" && runtime.sessionState == "ready" && runtime.clockOwner == "" {
				runtime.logEvent(kind, report.QueueID, "ignored", "no_owner", actor)
				return nil, nil
			}
			if report.Code == "stream-trial-only" || report.Code == "stream-unavailable" {
				runtime.verifyPlaybackError(report.PlayEpoch, report.QueueID, actor)
				return nil, nil
			}
			reason, message := "unknown", "Playback failed after retrying"
			switch report.Code {
			case "retries-exhausted":
			default:
				return nil, nil
			}
			music := runtime.state.CurrentMusic
			payload := map[string]string{"queueId": report.QueueID, "musicId": music.ID, "name": music.Name, "platform": music.Platform, "reason": reason, "message": message}
			epoch := runtime.state.PlayEpoch
			if err := runtime.advance(ctx); err != nil {
				return nil, err
			}
			runtime.failedEpoch = epoch
			runtime.broadcast("player.failed", payload)
			runtime.logEventAt(kind, report.QueueID, epoch, "applied", reason, actor)
			return nil, nil
		case "playback.started":
			if runtime.sessionState != "ready" && runtime.sessionState != "stalled" {
				runtime.logEvent(kind, report.QueueID, "ignored", "state", actor)
				return nil, nil
			}
			if runtime.sessionState == "stalled" && runtime.clockOwner != "" && runtime.clockOwner != connection {
				runtime.logEvent(kind, report.QueueID, "ignored", "not_owner", actor)
				return nil, nil
			}
			position := max(0, report.PositionMS)
			if runtime.sessionState == "ready" && report.DurationMS > 0 && report.DurationMS < 24*60*60*1000 {
				nextState.CurrentMusic.Duration = report.DurationMS
			}
			// Anchor at "now" so the clock starts when playback actually starts.
			// Backdating by positionMs or by a slow client clock makes progress
			// jump forward before any client has produced sound.
			nextState.PositionAnchor = position
			nextState.TimestampAnchor = now
			nextState.PositionUpdatedAt = now
			nextState.Paused = false
			nextSessionState = "playing"
			runtime.clockOwner = connection
		case "playback.stalled":
			if runtime.sessionState != "playing" {
				return nil, nil
			}
			if runtime.clockOwner != "" && runtime.clockOwner != connection {
				runtime.logEvent(kind, report.QueueID, "ignored", "not_owner", actor)
				return nil, nil
			}
			nextState.PositionAnchor = max(0, report.PositionMS)
			nextState.TimestampAnchor = now
			nextState.PositionUpdatedAt = now
			nextSessionState = "stalled"
		case "playback.ended":
			if runtime.sessionState != "playing" {
				runtime.logEvent(kind, report.QueueID, "ignored", "state", actor)
				return nil, nil
			}
			currentPosition := runtime.currentPosition(now)
			// Reject only reports that arrive before the track could have ended.
			// A late report (slow client, stalled network) is still a real end;
			// the server fallback only fires at duration+fallbackGraceMS, so a
			// report in between must count or the track waits out the grace.
			if runtime.state.CurrentMusic == nil || runtime.state.CurrentMusic.Duration <= 0 || currentPosition < runtime.state.CurrentMusic.Duration-endedToleranceMS {
				runtime.logEvent(kind, report.QueueID, "rejected", "too_early", actor)
				return nil, nil
			}
			endedEpoch := runtime.state.PlayEpoch
			if err := runtime.advance(ctx); err != nil {
				return nil, err
			}
			runtime.logEventAt(kind, report.QueueID, endedEpoch, "applied", "", actor)
			return nil, nil
		default:
			return nil, nil
		}
		nextState.StateVersion++
		nextState.LastPersistedAt = now
		if err := runtime.realtimeRepo.CommitRoomState(ctx, runtime.roomID, runtime.queue, nextState, nil); err != nil {
			return nil, err
		}
		runtime.state = nextState
		runtime.sessionState = nextSessionState
		runtime.sessionStateSince = time.Now()
		runtime.broadcastState()
		runtime.logEvent(kind, report.QueueID, "applied", "", actor)
		if kind == "playback.started" {
			runtime.prepareNextTrack()
		}
		return nil, nil
	})
	return err
}

// PlaybackConnectionClosed releases clock ownership when its websocket leaves.
func (r *RoomRuntime) PlaybackConnectionClosed(ctx context.Context, connectionID string) error {
	_, err := r.Execute(ctx, func(runtime *RoomRuntime, ctx context.Context) (any, error) {
		if runtime.clockOwner != connectionID {
			return nil, nil
		}
		runtime.clockOwner = ""
		if runtime.sessionState == "stalled" && runtime.state.CurrentMusic != nil && !runtime.state.Paused {
			now := time.Now().UnixMilli()
			next := clonePlaybackState(runtime.state)
			next.TimestampAnchor = now
			next.PositionUpdatedAt = now
			next.StateVersion++
			next.LastPersistedAt = now
			if err := runtime.realtimeRepo.CommitRoomState(ctx, runtime.roomID, runtime.queue, next, nil); err != nil {
				return nil, err
			}
			runtime.state = next
			runtime.sessionState = "playing"
			runtime.sessionStateSince = time.Now()
			runtime.broadcastState()
			runtime.logEvent("playback.started", runtime.currentQueueID, "server", "owner_disconnected", nil)
		}
		return nil, nil
	})
	return err
}

// PlaybackFailed sends a sanitized failure notice for a track that cannot be resolved.
func (r *RoomRuntime) PlaybackFailed(ctx context.Context, queueID, musicID, name, platformName, reason, message string) error {
	_, err := r.Execute(ctx, func(runtime *RoomRuntime, _ context.Context) (any, error) {
		payload := map[string]string{"queueId": queueID, "musicId": musicID, "name": name, "platform": platformName, "reason": reason, "message": message}
		runtime.broadcast("player.failed", payload)
		runtime.logEvent("player.failed", queueID, "applied", reason, nil)
		return nil, nil
	})
	return err
}

// prepareNextTrack resolves and cache-submits one queued item without blocking the room actor.
func (r *RoomRuntime) prepareNextTrack() {
	if r.preResolve == nil || r.preResolveBusy || len(r.queue) == 0 {
		return
	}
	item := r.queue[0]
	r.preResolveBusy = true
	go func() {
		defer func() {
			_, _ = r.Execute(context.Background(), func(runtime *RoomRuntime, _ context.Context) (any, error) {
				runtime.preResolveBusy = false
				return nil, nil
			})
		}()
		r.preResolve(r.ctx, item.Music)
	}()
}

// verifyPlaybackError resolves media off-actor, then sends the result back through the actor.
func (r *RoomRuntime) verifyPlaybackError(epoch int64, queueID string, actor *string) {
	if r.verifyStream == nil || r.state.CurrentMusic == nil {
		r.logEventAt("playback.error", queueID, epoch, "ignored", "unconfirmed", actor)
		return
	}
	if _, exists := r.verificationEpochs[epoch]; exists {
		return
	}
	if r.verificationEpochs == nil {
		r.verificationEpochs = make(map[int64]struct{})
	}
	music := storesqlite.Music{ID: r.state.CurrentMusic.ID, Name: r.state.CurrentMusic.Name, Artists: slices.Clone(r.state.CurrentMusic.Artists), Duration: r.state.CurrentMusic.Duration, Platform: r.state.CurrentMusic.Platform, CoverURL: r.state.CurrentMusic.CoverURL}
	r.verificationEpochs[epoch] = struct{}{}
	go func() {
		err := r.verifyStream(r.ctx, music)
		_, _ = r.Execute(context.Background(), func(runtime *RoomRuntime, ctx context.Context) (any, error) {
			delete(runtime.verificationEpochs, epoch)
			if runtime.state.PlayEpoch != epoch || runtime.currentQueueID != queueID || runtime.state.CurrentMusic == nil || runtime.failedEpoch == epoch {
				runtime.logEventAt("playback.error", queueID, epoch, "ignored", "stale_verification", actor)
				return nil, nil
			}
			var streamErr *platform.StreamResolutionError
			if !errors.As(err, &streamErr) || (streamErr.Kind != platform.StreamTrialOnly && streamErr.Kind != platform.StreamUnavailable) {
				runtime.logEventAt("playback.error", queueID, epoch, "ignored", "unconfirmed", actor)
				return nil, nil
			}
			reason, message := "unavailable", "This track is unavailable for streaming"
			if streamErr.Kind == platform.StreamTrialOnly {
				reason, message = "preview_only", "Only a preview is available for this track"
			}
			music := runtime.state.CurrentMusic
			payload := map[string]string{"queueId": queueID, "musicId": music.ID, "name": music.Name, "platform": music.Platform, "reason": reason, "message": message}
			if err := runtime.advance(ctx); err != nil {
				return nil, err
			}
			runtime.failedEpoch = epoch
			runtime.broadcast("player.failed", payload)
			runtime.logEventAt("playback.error", queueID, epoch, "applied", reason, actor)
			return nil, nil
		})
	}()
}

func (r *RoomRuntime) rejectPending() {
	for {
		select {
		case command := <-r.commands:
			command.result <- commandResult{err: context.Canceled}
		default:
			return
		}
	}
}
func (r *RoomRuntime) Close() { r.closeOnce.Do(func() { r.cancel(); <-r.done }) }
func setValues(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	slices.Sort(result)
	return result
}
