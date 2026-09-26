package realtime

import (
	"context"
	"database/sql"
	"errors"
	"math/rand/v2"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	"github.com/google/uuid"
)

var (
	ErrCommandQueueFull   = errors.New("room command queue is full")
	ErrStaleQueue         = errors.New("stale queue mutation")
	ErrDuplicate          = errors.New("duplicate mutation")
	ErrControlLocked      = errors.New("playback control is locked")
	ErrControlDenied      = errors.New("playback control is denied")
	ErrSeekForbidden      = errors.New("playback seek is forbidden")
	ErrPreconditionFailed = errors.New("playback precondition failed")
	ErrNoHistory          = errors.New("no playback history available")
)

// maxHistoryEntries bounds the in-memory playback history that powers
// control.previous; older entries stay in the database but are not reachable.
const maxHistoryEntries = 100

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
	store       *storesqlite.Store
	queueSize   int
	timeout     time.Duration
	idle        time.Duration
	broadcaster Broadcaster
	mu          sync.Mutex
	runtimes    map[string]*RoomRuntime
	closed      bool
}

func NewManager(store *storesqlite.Store, queueSize int, timeout, idle time.Duration, broadcaster Broadcaster) *Manager {
	return &Manager{store: store, queueSize: max(1, queueSize), timeout: timeout, idle: idle, broadcaster: broadcaster, runtimes: map[string]*RoomRuntime{}}
}

func (m *Manager) Room(ctx context.Context, roomID string) (*RoomRuntime, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, context.Canceled
	}
	if runtime := m.runtimes[roomID]; runtime != nil {
		return runtime, nil
	}
	runtime, err := newRoomRuntime(ctx, m.store, roomID, m.queueSize, m.timeout, m.idle, m.broadcaster, func() {
		m.mu.Lock()
		delete(m.runtimes, roomID)
		m.mu.Unlock()
	})
	if err != nil {
		return nil, err
	}
	m.runtimes[roomID] = runtime
	return runtime, nil
}

func (m *Manager) Close() {
	m.mu.Lock()
	m.closed = true
	runtimes := make([]*RoomRuntime, 0, len(m.runtimes))
	for _, runtime := range m.runtimes {
		runtimes = append(runtimes, runtime)
	}
	m.mu.Unlock()
	for _, runtime := range runtimes {
		runtime.Close()
	}
}

type command struct {
	ctx    context.Context
	apply  func(*RoomRuntime) (any, error)
	result chan commandResult
}
type commandResult struct {
	value any
	err   error
}

type RoomRuntime struct {
	roomID           string
	queueRepo        *storesqlite.QueueRepository
	stateRepo        *storesqlite.PlaybackStateRepository
	realtimeRepo     *storesqlite.RealtimeRepository
	chatRepo         *storesqlite.ChatRepository
	playlistRepo     *storesqlite.UserPlaylistRepository
	queue            []storesqlite.QueueItem
	state            storesqlite.PlaybackState
	history          []storesqlite.HistoryEntry
	historySkipped   int
	queueVersion     int64
	commands         chan command
	ctx              context.Context
	cancel           context.CancelFunc
	done             chan struct{}
	timeout          time.Duration
	broadcaster      Broadcaster
	mutations        map[string]struct{}
	controlScope     string
	controlMutations map[controlKey]controlRecord
	idle             time.Duration
	lastActivity     time.Time
	clients          int
	advancePending   bool
	onClose          func()
	closeOnce        sync.Once
}

func newRoomRuntime(ctx context.Context, store *storesqlite.Store, roomID string, capacity int, timeout, idle time.Duration, broadcaster Broadcaster, onClose func()) (*RoomRuntime, error) {
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
	r := &RoomRuntime{roomID: roomID, queueRepo: queueRepo, stateRepo: stateRepo, realtimeRepo: storesqlite.NewRealtimeRepository(store, time.Now), chatRepo: storesqlite.NewChatRepository(store), playlistRepo: storesqlite.NewUserPlaylistRepository(store, time.Now, nil), queue: queue, state: *state, commands: make(chan command, capacity), ctx: runtimeCtx, cancel: cancel, done: make(chan struct{}), timeout: timeout, broadcaster: broadcaster, mutations: map[string]struct{}{}, idle: idle, lastActivity: time.Now(), onClose: onClose}
	if history, historyErr := queueRepo.LoadHistory(ctx, roomID, maxHistoryEntries); historyErr == nil {
		r.history = history
	}
	r.controlScope = uuid.NewString()
	r.controlMutations = make(map[controlKey]controlRecord)
	go r.loop()
	return r, nil
}

func (r *RoomRuntime) Execute(ctx context.Context, apply func(*RoomRuntime) (any, error)) (any, error) {
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
	value, err := r.Execute(ctx, func(runtime *RoomRuntime) (any, error) { return runtime.snapshot(), nil })
	if err != nil {
		return nil, err
	}
	return value.(map[string]any), nil
}

func (r *RoomRuntime) Attach(ctx context.Context) error {
	_, err := r.Execute(ctx, func(runtime *RoomRuntime) (any, error) {
		runtime.clients++
		return nil, nil
	})
	return err
}

func (r *RoomRuntime) Detach(ctx context.Context) error {
	_, err := r.Execute(ctx, func(runtime *RoomRuntime) (any, error) {
		if runtime.clients > 0 {
			runtime.clients--
		}
		return nil, nil
	})
	return err
}

func (r *RoomRuntime) Enqueue(ctx context.Context, session account.Session, music storesqlite.Music, mutationID string) (storesqlite.QueueItem, error) {
	value, err := r.Execute(ctx, func(runtime *RoomRuntime) (any, error) {
		if err := runtime.claimMutation(mutationID); err != nil {
			return nil, err
		}
		item := storesqlite.QueueItem{QueueID: uuid.NewString(), Music: music, EnqueuedBy: storesqlite.UserSummary{PublicID: session.PublicID, Name: session.DisplayName, Guest: session.Guest}, Status: "PENDING"}
		next := append(slices.Clone(runtime.queue), item)
		nextState := clonePlaybackState(runtime.state)
		started := startFirst(&nextState, &next)
		if started {
			if err := runtime.realtimeRepo.CommitRoomState(ctx, runtime.roomID, next, nextState, nil); err != nil {
				return nil, err
			}
		} else if err := runtime.queueRepo.SynchronizeQueue(ctx, runtime.roomID, next); err != nil {
			return nil, err
		}
		runtime.queue = next
		runtime.state = nextState
		runtime.rememberMutation(mutationID)
		runtime.queueVersion++
		runtime.broadcast("queue.patch", map[string]any{"operation": "append", "queueVersion": runtime.queueVersion, "items": []storesqlite.QueueItem{item}})
		runtime.broadcastState()
		return item, nil
	})
	if err != nil {
		return storesqlite.QueueItem{}, err
	}
	return value.(storesqlite.QueueItem), nil
}

func (r *RoomRuntime) EnqueueMany(ctx context.Context, session account.Session, musics []storesqlite.Music, mutationID string) ([]storesqlite.QueueItem, error) {
	value, err := r.Execute(ctx, func(runtime *RoomRuntime) (any, error) {
		if err := runtime.claimMutation(mutationID); err != nil {
			return nil, err
		}
		if len(musics) == 0 {
			return nil, errors.New("playlist is empty")
		}
		items := make([]storesqlite.QueueItem, 0, len(musics))
		for _, music := range musics {
			items = append(items, storesqlite.QueueItem{QueueID: uuid.NewString(), Music: music, EnqueuedBy: storesqlite.UserSummary{PublicID: session.PublicID, Name: session.DisplayName, Guest: session.Guest}, Status: "PENDING"})
		}
		next := append(slices.Clone(runtime.queue), items...)
		nextState := clonePlaybackState(runtime.state)
		started := startFirst(&nextState, &next)
		if started {
			if err := runtime.realtimeRepo.CommitRoomState(ctx, runtime.roomID, next, nextState, nil); err != nil {
				return nil, err
			}
		} else if err := runtime.queueRepo.SynchronizeQueue(ctx, runtime.roomID, next); err != nil {
			return nil, err
		}
		runtime.queue = next
		runtime.state = nextState
		runtime.rememberMutation(mutationID)
		runtime.queueVersion++
		runtime.broadcast("queue.patch", map[string]any{"operation": "append", "queueVersion": runtime.queueVersion, "items": items})
		runtime.broadcastState()
		return items, nil
	})
	if err != nil {
		return nil, err
	}
	return value.([]storesqlite.QueueItem), nil
}

func (r *RoomRuntime) Remove(ctx context.Context, ids []string, mutationID string) error {
	_, err := r.Execute(ctx, func(runtime *RoomRuntime) (any, error) {
		if err := runtime.claimMutation(mutationID); err != nil {
			return nil, err
		}
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
		runtime.rememberMutation(mutationID)
		runtime.queueVersion++
		runtime.broadcast("queue.patch", map[string]any{"operation": "remove", "queueVersion": runtime.queueVersion, "queueIds": ids})
		runtime.broadcastState()
		return nil, nil
	})
	return err
}

func (r *RoomRuntime) Top(ctx context.Context, ids []string, mutationID string) error {
	_, err := r.Execute(ctx, func(runtime *RoomRuntime) (any, error) {
		if err := runtime.claimMutation(mutationID); err != nil {
			return nil, err
		}
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
		next := append(selected, rest...)
		if err := runtime.queueRepo.SynchronizeQueue(ctx, runtime.roomID, next); err != nil {
			return nil, err
		}
		runtime.queue = next
		runtime.rememberMutation(mutationID)
		runtime.queueVersion++
		runtime.broadcast("queue.patch", map[string]any{"operation": "snapshot", "queueVersion": runtime.queueVersion, "queue": next})
		runtime.broadcastState()
		return nil, nil
	})
	return err
}

func (r *RoomRuntime) Reorder(ctx context.Context, oldIndex, newIndex int, mutationID string) error {
	_, err := r.Execute(ctx, func(runtime *RoomRuntime) (any, error) {
		if err := runtime.claimMutation(mutationID); err != nil {
			return nil, err
		}
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
		runtime.rememberMutation(mutationID)
		runtime.queueVersion++
		runtime.broadcast("queue.patch", map[string]any{"operation": "snapshot", "queueVersion": runtime.queueVersion, "queue": next})
		runtime.broadcastState()
		return nil, nil
	})
	return err
}

func (r *RoomRuntime) ReorderByID(ctx context.Context, queueID, targetQueueID, placement, mutationID string) error {
	_, err := r.Execute(ctx, func(runtime *RoomRuntime) (any, error) {
		if err := runtime.claimMutation(mutationID); err != nil {
			return nil, err
		}
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
		runtime.rememberMutation(mutationID)
		runtime.queueVersion++
		runtime.broadcast("queue.patch", map[string]any{"operation": "move", "queueVersion": runtime.queueVersion, "queueId": queueID, "targetQueueId": targetQueueID, "position": placement})
		runtime.broadcastState()
		return nil, nil
	})
	return err
}

func (r *RoomRuntime) TogglePause(ctx context.Context, mutation ...ControlMutation) (ControlResult, error) {
	value, err := r.executeControl(ctx, "control.toggle-pause", 0, nil, mutation, func(runtime *RoomRuntime) (any, error) {
		if runtime.state.PauseLocked && !runtime.state.Paused {
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
		return ControlResult{Applied: true, Committed: runtime.watermark()}, nil
	})
	if err != nil {
		return controlResultValue(value), err
	}
	return value.(ControlResult), nil
}

func (r *RoomRuntime) ToggleShuffle(ctx context.Context, mutation ...ControlMutation) (ControlResult, error) {
	return r.mutateStateChecked(ctx, "control.toggle-shuffle", 0, nil, mutation, func(s storesqlite.PlaybackState) error {
		if s.ShuffleLocked {
			return ErrControlLocked
		}
		return nil
	}, func(s *storesqlite.PlaybackState) { s.Shuffle = !s.Shuffle })
}
func (r *RoomRuntime) Seek(ctx context.Context, requested int64, publicID string, admin bool, expectedEpoch *int64, mutation ...ControlMutation) (ControlResult, error) {
	return r.mutateStateChecked(ctx, "control.seek", requested, expectedEpoch, mutation, func(s storesqlite.PlaybackState) error {
		if expectedEpoch != nil && *expectedEpoch != s.PlayEpoch {
			return ErrPreconditionFailed
		}
		if s.CurrentMusic == nil {
			return ErrControlDenied
		}
		if !admin && (s.CurrentEnqueuerID == nil || *s.CurrentEnqueuerID != publicID) {
			return ErrSeekForbidden
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
	value, err := r.executeControl(ctx, "control.like", 0, expectedEpoch, mutation, func(runtime *RoomRuntime) (any, error) {
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
		if err := runtime.syncLikedSongs(ctx, publicID, music, true); err != nil {
			return nil, err
		}
		if err := runtime.mutateStateDirect(ctx, func(s *storesqlite.PlaybackState) {
			if s.LikedUserIDs == nil {
				s.LikedUserIDs = map[string]struct{}{}
			}
			s.LikedUserIDs[publicID] = struct{}{}
			s.LikeMarkers = append(s.LikeMarkers, position(*s, time.Now().UnixMilli()))
		}); err != nil {
			return nil, err
		}
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
	value, err := r.executeControl(ctx, "control.unlike", 0, expectedEpoch, mutation, func(runtime *RoomRuntime) (any, error) {
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
		if err := runtime.syncLikedSongs(ctx, publicID, music, false); err != nil {
			return nil, err
		}
		if err := runtime.mutateStateDirect(ctx, func(s *storesqlite.PlaybackState) {
			delete(s.LikedUserIDs, publicID)
			// LikeMarkers carries one entry per outstanding like without saying who
			// made which one, so the newest marker leaves with the newest like.
			if len(s.LikeMarkers) > 0 {
				s.LikeMarkers = s.LikeMarkers[:len(s.LikeMarkers)-1]
			}
		}); err != nil {
			return nil, err
		}
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
	LikedSongsSystemKey = "liked-songs"
	LikedSongsName      = "喜欢的歌曲"
)

func playedMusic(music *storesqlite.PlayableMusic) storesqlite.Music {
	return storesqlite.Music{ID: music.ID, Name: music.Name, Artists: slices.Clone(music.Artists), Duration: music.Duration, Platform: music.Platform, CoverURL: music.CoverURL}
}

// syncLikedSongs mirrors a room like onto the member's own liked list, which is
// the same `liked-songs` system playlist the web pages maintain through
// /api/me/liked-songs. It runs before the room state commits, so a write that
// fails leaves the shared heart and the personal list agreeing on "not liked".
func (r *RoomRuntime) syncLikedSongs(ctx context.Context, publicID string, music storesqlite.Music, liked bool) error {
	if !liked {
		playlist, err := r.playlistRepo.FindSystemPlaylist(ctx, publicID, LikedSongsSystemKey)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		_, err = r.playlistRepo.DeleteTrackByMusicKey(ctx, publicID, playlist.ID, music.Platform+":"+music.ID)
		return err
	}
	playlist, err := r.EnsureLikedSongs(ctx, publicID)
	if err != nil {
		return err
	}
	_, err = r.playlistRepo.AddTrackIfAbsent(ctx, publicID, playlist.ID, &music)
	return err
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
	value, err := r.executeControl(ctx, "control.next", 0, nil, mutation, func(runtime *RoomRuntime) (any, error) {
		if runtime.state.SkipLocked {
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
	value, err := r.executeControl(ctx, "control.previous", 0, expectedEpoch, mutation, func(runtime *RoomRuntime) (any, error) {
		if expectedEpoch != nil && *expectedEpoch != runtime.state.PlayEpoch {
			return nil, ErrPreconditionFailed
		}
		if runtime.state.SkipLocked {
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

func (r *RoomRuntime) historyCursor() int {
	return max(0, len(r.history)-r.historySkipped)
}

func (r *RoomRuntime) Clear(ctx context.Context, mutationID string) error {
	_, err := r.Execute(ctx, func(runtime *RoomRuntime) (any, error) {
		if err := runtime.claimMutation(mutationID); err != nil {
			return nil, err
		}
		if len(runtime.queue) == 0 {
			return nil, ErrStaleQueue
		}
		next := []storesqlite.QueueItem{}
		if err := runtime.queueRepo.SynchronizeQueue(ctx, runtime.roomID, next); err != nil {
			return nil, err
		}
		runtime.queue = next
		runtime.rememberMutation(mutationID)
		runtime.queueVersion++
		runtime.broadcast("queue.patch", map[string]any{"operation": "clear", "queueVersion": runtime.queueVersion, "queue": next})
		runtime.broadcastState()
		return nil, nil
	})
	return err
}

func (r *RoomRuntime) advance(ctx context.Context) error {
	nextQueue := slices.Clone(r.queue)
	nextState := clonePlaybackState(r.state)
	var history *storesqlite.HistoryEntry
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
	r.advancePending = false
	if history != nil {
		r.history = append([]storesqlite.HistoryEntry{*history}, r.history...)
		if len(r.history) > maxHistoryEntries {
			r.history = r.history[:maxHistoryEntries]
		}
	}
	r.historySkipped = 0
	r.queueVersion++
	r.broadcast("queue.patch", map[string]any{"operation": "snapshot", "queueVersion": r.queueVersion, "queue": nextQueue})
	r.broadcastState()
	return nil
}

func (r *RoomRuntime) AppendChat(ctx context.Context, session account.Session, content string, public bool) (storesqlite.ChatMessage, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return storesqlite.ChatMessage{}, errors.New("chat message is empty")
	}
	if len([]rune(content)) > 200 {
		return storesqlite.ChatMessage{}, errors.New("chat message is too long")
	}
	message := storesqlite.ChatMessage{ID: uuid.NewString(), UserID: session.PublicID, UserName: session.DisplayName, Content: content, Timestamp: time.Now().UnixMilli(), Type: "CHAT"}
	var room *string
	messageType := "public-chat.message"
	if !public {
		room = &r.roomID
		messageType = "chat.message"
	}
	if err := r.chatRepo.AppendMessage(ctx, room, message); err != nil {
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
// maxHistoryEntries window the runtime keeps for control.previous.
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
	value, err := r.executeControl(ctx, kind, positionMS, expectedEpoch, mutation, func(runtime *RoomRuntime) (any, error) {
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

func startFirst(state *storesqlite.PlaybackState, queue *[]storesqlite.QueueItem) bool {
	if state.CurrentMusic != nil || len(*queue) == 0 {
		return false
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
	return true
}

// takeNext selects and removes one item while preserving the order of the
// remaining visible queue.
func takeNext(state *storesqlite.PlaybackState, queue *[]storesqlite.QueueItem) storesqlite.QueueItem {
	index := 0
	if state.Shuffle {
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
func (r *RoomRuntime) claimMutation(id string) error {
	if id == "" {
		return nil
	}
	if _, ok := r.mutations[id]; ok {
		return ErrDuplicate
	}
	return nil
}
func (r *RoomRuntime) rememberMutation(id string) {
	if id == "" {
		return
	}
	if len(r.mutations) > 2048 {
		clear(r.mutations)
	}
	r.mutations[id] = struct{}{}
}
func (r *RoomRuntime) snapshot() map[string]any {
	var nowPlaying any
	if r.state.CurrentMusic != nil {
		nowPlaying = map[string]any{"music": r.state.CurrentMusic, "currentPosition": position(r.state, time.Now().UnixMilli()), "enqueuedById": r.state.CurrentEnqueuerID, "enqueuedByName": r.state.CurrentEnqueuerName, "likedUserIds": setValues(r.state.LikedUserIDs), "likeMarkers": r.state.LikeMarkers, "playEpoch": r.state.PlayEpoch, "positionUpdatedAt": r.state.PositionUpdatedAt}
	}
	return map[string]any{"nowPlaying": nowPlaying, "queue": r.queue, "isShuffle": r.state.Shuffle, "isPaused": r.state.Paused, "isPauseLocked": r.state.PauseLocked, "isSkipLocked": r.state.SkipLocked, "isShuffleLocked": r.state.ShuffleLocked, "isLoading": r.state.Loading, "serverTimestamp": time.Now().UnixMilli(), "stateVersion": r.state.StateVersion, "playEpoch": r.state.PlayEpoch, "queueVersion": r.queueVersion, "historyCursor": r.historyCursor(), "idempotencyScopeId": r.controlScope, "idempotencyTtlMs": ControlMutationTTL.Milliseconds()}
}
func (r *RoomRuntime) broadcastState() {
	r.broadcast("player.state", r.snapshot())
}
func (r *RoomRuntime) broadcast(kind string, payload any) {
	if r.broadcaster != nil {
		r.broadcaster.BroadcastRoom(r.roomID, kind, payload)
	}
}
func (r *RoomRuntime) loop() {
	defer close(r.done)
	defer r.onClose()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
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
			value, err := command.apply(r)
			r.lastActivity = time.Now()
			command.result <- commandResult{value: value, err: err}
		}
	}
}

func (r *RoomRuntime) onTick(now time.Time) {
	if r.state.CurrentMusic == nil || r.state.Paused {
		return
	}
	current := position(r.state, now.UnixMilli())
	r.broadcast("player.progress", map[string]any{"currentPosition": current, "serverTimestamp": now.UnixMilli(), "stateVersion": r.state.StateVersion, "playEpoch": r.state.PlayEpoch})
	if r.state.CurrentMusic.Duration > 0 && current >= r.state.CurrentMusic.Duration && !r.advancePending {
		r.advancePending = true
		_ = r.advance(r.ctx)
	}
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
