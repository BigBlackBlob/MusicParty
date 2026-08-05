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
	ErrCommandQueueFull = errors.New("room command queue is full")
	ErrStaleQueue       = errors.New("stale queue mutation")
	ErrDuplicate        = errors.New("duplicate mutation")
	ErrControlLocked    = errors.New("playback control is locked")
	ErrControlDenied    = errors.New("playback control is denied")
)

type Broadcaster interface {
	BroadcastRoom(string, string, any)
	BroadcastAll(string, any)
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
	roomID         string
	queueRepo      *storesqlite.QueueRepository
	stateRepo      *storesqlite.PlaybackStateRepository
	realtimeRepo   *storesqlite.RealtimeRepository
	chatRepo       *storesqlite.ChatRepository
	queue          []storesqlite.QueueItem
	state          storesqlite.PlaybackState
	queueVersion   int64
	commands       chan command
	ctx            context.Context
	cancel         context.CancelFunc
	done           chan struct{}
	timeout        time.Duration
	broadcaster    Broadcaster
	mutations      map[string]struct{}
	idle           time.Duration
	lastActivity   time.Time
	clients        int
	advancePending bool
	onClose        func()
	closeOnce      sync.Once
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
	r := &RoomRuntime{roomID: roomID, queueRepo: queueRepo, stateRepo: stateRepo, realtimeRepo: storesqlite.NewRealtimeRepository(store, time.Now), chatRepo: storesqlite.NewChatRepository(store), queue: queue, state: *state, commands: make(chan command, capacity), ctx: runtimeCtx, cancel: cancel, done: make(chan struct{}), timeout: timeout, broadcaster: broadcaster, mutations: map[string]struct{}{}, idle: idle, lastActivity: time.Now(), onClose: onClose}
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

func (r *RoomRuntime) TogglePause(ctx context.Context) error {
	value, err := r.Execute(ctx, func(runtime *RoomRuntime) (any, error) {
		if runtime.state.PauseLocked && !runtime.state.Paused {
			return nil, ErrControlLocked
		}
		if runtime.state.CurrentMusic == nil {
			if len(runtime.queue) == 0 {
				return nil, ErrControlDenied
			}
			return nil, runtime.advance(ctx)
		}
		return nil, runtime.mutateStateDirect(ctx, func(s *storesqlite.PlaybackState) {
			now := time.Now().UnixMilli()
			if !s.Paused {
				s.PositionAnchor = position(*s, now)
			}
			s.Paused = !s.Paused
			s.TimestampAnchor = now
			s.PositionUpdatedAt = now
		})
	})
	_ = value
	return err
}

func (r *RoomRuntime) ToggleShuffle(ctx context.Context) error {
	return r.mutateStateChecked(ctx, func(s storesqlite.PlaybackState) error {
		if s.ShuffleLocked {
			return ErrControlLocked
		}
		return nil
	}, func(s *storesqlite.PlaybackState) { s.Shuffle = !s.Shuffle })
}
func (r *RoomRuntime) Seek(ctx context.Context, requested int64, publicID string, admin bool) error {
	return r.mutateStateChecked(ctx, func(s storesqlite.PlaybackState) error {
		if s.CurrentMusic == nil {
			return ErrControlDenied
		}
		if !admin && (s.CurrentEnqueuerID == nil || *s.CurrentEnqueuerID != publicID) {
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
func (r *RoomRuntime) Like(ctx context.Context, publicID string) error {
	_, err := r.Execute(ctx, func(runtime *RoomRuntime) (any, error) {
		if runtime.state.CurrentMusic == nil {
			return nil, nil
		}
		if _, exists := runtime.state.LikedUserIDs[publicID]; exists {
			return nil, nil
		}
		return nil, runtime.mutateStateDirect(ctx, func(s *storesqlite.PlaybackState) {
			if s.LikedUserIDs == nil {
				s.LikedUserIDs = map[string]struct{}{}
			}
			s.LikedUserIDs[publicID] = struct{}{}
			s.LikeMarkers = append(s.LikeMarkers, position(*s, time.Now().UnixMilli()))
		})
	})
	return err
}

func (r *RoomRuntime) Next(ctx context.Context) error {
	_, err := r.Execute(ctx, func(runtime *RoomRuntime) (any, error) {
		if runtime.state.SkipLocked {
			return nil, ErrControlLocked
		}
		return nil, runtime.advance(ctx)
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
		index := 0
		if nextState.Shuffle {
			index = rand.IntN(len(nextQueue))
		}
		item := nextQueue[index]
		nextQueue = slices.Delete(nextQueue, index, index+1)
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
	nextState.StateVersion++
	nextState.LastPersistedAt = now
	if err := r.realtimeRepo.CommitRoomState(ctx, r.roomID, nextQueue, nextState, history); err != nil {
		r.advancePending = false
		return err
	}
	r.queue = nextQueue
	r.state = nextState
	r.advancePending = false
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

func (r *RoomRuntime) mutateStateChecked(ctx context.Context, check func(storesqlite.PlaybackState) error, mutate func(*storesqlite.PlaybackState)) error {
	_, err := r.Execute(ctx, func(runtime *RoomRuntime) (any, error) {
		if check != nil {
			if err := check(runtime.state); err != nil {
				return nil, err
			}
		}
		return nil, runtime.mutateStateDirect(ctx, mutate)
	})
	return err
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

func startFirst(state *storesqlite.PlaybackState, queue *[]storesqlite.QueueItem) bool {
	if state.CurrentMusic != nil || len(*queue) == 0 {
		return false
	}
	item := (*queue)[0]
	*queue = slices.Delete(*queue, 0, 1)
	now := time.Now().UnixMilli()
	state.CurrentMusic = playableMusic(item.Music)
	state.CurrentEnqueuerID = &item.EnqueuedBy.PublicID
	state.CurrentEnqueuerName = &item.EnqueuedBy.Name
	state.Paused = false
	state.PositionAnchor = 0
	state.TimestampAnchor = now
	state.PositionUpdatedAt = now
	state.PlayEpoch++
	state.StateVersion++
	state.LastPersistedAt = now
	return true
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
	return map[string]any{"nowPlaying": nowPlaying, "queue": r.queue, "isShuffle": r.state.Shuffle, "isPaused": r.state.Paused, "isPauseLocked": r.state.PauseLocked, "isSkipLocked": r.state.SkipLocked, "isShuffleLocked": r.state.ShuffleLocked, "isLoading": r.state.Loading, "serverTimestamp": time.Now().UnixMilli(), "stateVersion": r.state.StateVersion, "playEpoch": r.state.PlayEpoch, "queueVersion": r.queueVersion}
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
