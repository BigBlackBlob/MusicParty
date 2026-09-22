package httpapi

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	roomdomain "github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/room"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/observability"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/realtime"
	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	wsruntime "github.com/BigBlackBlob/MusicParty/backend-go/internal/ws"
	"github.com/coder/websocket"
	"github.com/stretchr/testify/require"
)

// This helper runs in a separate test-binary process. Only production HTTP/WS,
// authentication, runtime and persistence components serve requests. The platform
// resolves synthetic metadata only; it never opens media or an upstream service.
// Configuration is not loaded from the host environment. httpStore owns a fresh
// t.TempDir, and the OS allocates an exclusive loopback listener, never port 18081.
func TestControlProcessHelper(t *testing.T) {
	if os.Getenv("MUSICPARTY_P3_HELPER") != "1" {
		t.Skip("isolated subprocess only")
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	slog.SetDefault(logger)
	store := httpStore(t)
	for _, room := range []string{"lounge", "locked", "other", "capacity", "fault", "idle"} {
		insertStage5Room(t, store, room, "PUBLIC", 0)
	}
	require.NoError(t, storesqlite.NewPlaybackStateRepository(store).Upsert(context.Background(), storesqlite.PlaybackState{
		RoomID: "locked", PauseLocked: true, SkipLocked: true, ShuffleLocked: true,
		LikedUserIDs: map[string]struct{}{}, LikeMarkers: []int64{},
	}))
	// Fault injection belongs only to this disposable fixture. No production
	// error hook or existing data is involved; SQLite rejects this room's commit.
	require.NoError(t, store.Write(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `create trigger p3_reject_commit before insert on room_playback_state
			when NEW.room_id = 'fault' begin select raise(abort, 'p3 injected commit failure'); end`)
		return err
	}))
	accounts := account.New(store)
	rooms := roomdomain.New(store, accounts)
	cfg := testConfig(t)
	hub := wsruntime.NewHub(1024)
	manager := realtime.NewManager(store, 100, 5*time.Second, 100*time.Millisecond, hub)
	defer manager.Close()
	defer hub.Close()
	platforms := NewPlatformAPI(cfg, fixturePlatform{})
	auth := NewAuthAPI(cfg, accounts)
	auth.SetSessionCloser(hub)
	api := NewWebSocketAPI(cfg, store, accounts, rooms, hub, manager, platforms)
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	require.NotEqual(t, 18081, listener.Addr().(*net.TCPAddr).Port)
	server := &http.Server{Handler: NewHandler(cfg, logger, observability.NewHealth(), observability.NewMetrics(),
		auth, api, NewDesktopAPI(cfg, accounts, rooms, platforms)), ReadHeaderTimeout: 5 * time.Second}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	// stdout is a machine control channel, not a service log. It never carries
	// credentials, HTTP bodies, database contents or complete websocket payloads.
	fmt.Fprintln(os.Stdout, "P3_READY http://"+listener.Addr().String())
	_, _ = io.Copy(io.Discard, os.Stdin)
	require.NoError(t, server.Close())
	require.ErrorIs(t, <-done, http.ErrServerClosed)
}

func startControlProcess(t *testing.T) string {
	t.Helper()
	executable, err := os.Executable()
	require.NoError(t, err)
	process := exec.Command(executable, "-test.run=^TestControlProcessHelper$", "-test.timeout=150s")
	process.Dir = t.TempDir()
	// Only OS runtime variables are inherited; business settings, proxy URLs and
	// credentials from the shell cannot reach the test server.
	process.Env = []string{"MUSICPARTY_P3_HELPER=1"}
	for _, key := range []string{"SYSTEMROOT", "WINDIR", "TEMP", "TMP", "TMPDIR"} {
		if value, ok := os.LookupEnv(key); ok {
			process.Env = append(process.Env, key+"="+value)
		}
	}
	stdin, err := process.StdinPipe()
	require.NoError(t, err)
	stdout, err := process.StdoutPipe()
	require.NoError(t, err)
	process.Stderr = io.Discard
	require.NoError(t, process.Start())
	ready := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			if strings.HasPrefix(scanner.Text(), "P3_READY ") {
				ready <- strings.TrimPrefix(scanner.Text(), "P3_READY ")
			}
		}
	}()
	t.Cleanup(func() {
		_ = stdin.Close()
		finished := make(chan error, 1)
		go func() { finished <- process.Wait() }()
		select {
		case err := <-finished:
			require.NoError(t, err, "isolated helper must shut down cleanly")
		case <-time.After(5 * time.Second):
			_ = process.Process.Kill()
			<-finished
			t.Error("isolated helper needed forced shutdown")
		}
	})
	select {
	case baseURL := <-ready:
		parsed, err := url.Parse(baseURL)
		require.NoError(t, err)
		require.Equal(t, "127.0.0.1", parsed.Hostname())
		require.NotEqual(t, "18081", parsed.Port())
		t.Logf("isolation: dedicated child process, fresh temporary data, %s, no mounts or existing services", baseURL)
		return baseURL
	case <-time.After(20 * time.Second):
		t.Fatal("isolated helper failed to become ready; raw output intentionally suppressed")
		return ""
	}
}

// Each client obtains its own disposable identity through the real HTTP guest
// endpoint. The cookie jar stays in memory and is never included in diagnostics.
func controlGuest(t *testing.T, baseURL, name string) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	client := &http.Client{Jar: jar, Timeout: 5 * time.Second}
	body, err := json.Marshal(map[string]string{"displayName": name})
	require.NoError(t, err)
	request, err := http.NewRequest(http.MethodPost, baseURL+"/api/account/guest", strings.NewReader(string(body)))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", baseURL)
	response, err := client.Do(request)
	require.NoError(t, err)
	response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode)
	return client
}

type controlPeer struct {
	t      *testing.T
	name   string
	conn   *websocket.Conn
	mu     sync.Mutex
	frames []wsruntime.Envelope
	done   chan struct{}
}

func connectControlPeer(t *testing.T, baseURL, room, name string, client *http.Client) *controlPeer {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(baseURL, "http")+"/api/desktop/v1/ws?roomId="+room,
		&websocket.DialOptions{HTTPClient: client, HTTPHeader: http.Header{"Origin": {baseURL}}})
	require.NoError(t, err)
	peer := &controlPeer{t: t, name: name, conn: conn, done: make(chan struct{})}
	go func() {
		defer close(peer.done)
		for {
			_, raw, err := conn.Read(context.Background())
			if err != nil {
				return
			}
			var frame wsruntime.Envelope
			if json.Unmarshal(raw, &frame) != nil {
				return
			}
			peer.mu.Lock()
			peer.frames = append(peer.frames, frame)
			peer.mu.Unlock()
		}
	}()
	t.Cleanup(func() { peer.close() })
	peer.send("client.hello", "", map[string]any{"apiVersion": "2026-01", "clientVersion": "0.2.0"})
	peer.await(0, "server.hello", "")
	peer.await(0, "player.state", "")
	return peer
}

func (p *controlPeer) close() {
	_ = p.conn.CloseNow()
	<-p.done
}

func (p *controlPeer) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.frames)
}

func (p *controlPeer) framesSince(start int) []wsruntime.Envelope {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]wsruntime.Envelope(nil), p.frames[start:]...)
}

func (p *controlPeer) send(kind, requestID string, payload map[string]any) {
	p.t.Helper()
	raw, err := json.Marshal(map[string]any{"type": kind, "requestId": requestID, "payload": payload})
	require.NoError(p.t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(p.t, p.conn.Write(ctx, websocket.MessageText, raw))
}

func (p *controlPeer) await(start int, kind, requestID string) map[string]any {
	p.t.Helper()
	var found map[string]any
	require.Eventually(p.t, func() bool {
		p.mu.Lock()
		defer p.mu.Unlock()
		for _, frame := range p.frames[start:] {
			if frame.Type == kind && (requestID == "" || frame.RequestID == requestID) {
				found, _ = frame.Payload.(map[string]any)
				return found != nil
			}
		}
		return false
	}, 5*time.Second, 5*time.Millisecond, "%s awaits %s request %s", p.name, kind, requestID)
	return found
}

func (p *controlPeer) control(kind, requestID string, payload map[string]any) map[string]any {
	p.t.Helper()
	start := p.count()
	p.send("control."+kind, requestID, payload)
	return p.await(start, "control.ack", requestID)
}

func (p *controlPeer) snapshot() map[string]any {
	p.t.Helper()
	start := p.count()
	p.send("player.resync", "snapshot", map[string]any{})
	return p.await(start, "player.state", "")
}

// Record only protocol metadata, not complete frames, identities or credentials.
func (p *controlPeer) trace(start int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for index, frame := range p.frames[start:] {
		payload, ok := frame.Payload.(map[string]any)
		if !ok || (frame.Type != "control.ack" && frame.Type != "player.state") {
			continue
		}
		p.t.Logf("frame %s #%d type=%s request=%v outcome=%v code=%v state=%v epoch=%v queue=%v replayed=%v",
			p.name, start+index+1, frame.Type, frame.RequestID, payload["outcome"], payload["code"],
			payload["stateVersion"], payload["playEpoch"], payload["queueVersion"], payload["replayed"])
		if committed, ok := payload["committed"].(map[string]any); ok {
			p.t.Logf("watermark %s #%d state=%v epoch=%v queue=%v", p.name, start+index+1,
				committed["stateVersion"], committed["playEpoch"], committed["queueVersion"])
		}
	}
}

func assertControlWatermark(t *testing.T, state, ack map[string]any) {
	t.Helper()
	committed, ok := ack["committed"].(map[string]any)
	require.True(t, ok, "successful ACK has committed watermark")
	for _, key := range []string{"stateVersion", "playEpoch", "queueVersion"} {
		require.GreaterOrEqual(t, state[key].(float64), committed[key].(float64), key)
	}
}

func TestControlIsolatedProcessTwoClients(t *testing.T) {
	if testing.Short() {
		t.Skip("cross-process HTTP and websocket acceptance")
	}
	baseURL := startControlProcess(t)
	firstHTTP := controlGuest(t, baseURL, "P3 first")
	secondHTTP := controlGuest(t, baseURL, "P3 second")
	request, err := http.NewRequest(http.MethodGet, baseURL+"/api/desktop/v1/capabilities", nil)
	require.NoError(t, err)
	request.Header.Set("X-Desktop-API-Version", "2026-01")
	request.Header.Set("X-Desktop-Client-Version", "0.2.0")
	response, err := firstHTTP.Do(request)
	require.NoError(t, err)
	var capabilities map[string]any
	require.NoError(t, json.NewDecoder(response.Body).Decode(&capabilities))
	response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode)
	features, ok := capabilities["features"].(map[string]any)
	require.True(t, ok)
	for _, feature := range []string{"controlAck", "controlPreconditions", "controlIdempotency"} {
		require.Equal(t, true, features[feature], feature)
	}
	a := connectControlPeer(t, baseURL, "lounge", "A", firstHTTP)
	b := connectControlPeer(t, baseURL, "lounge", "B", secondHTTP)
	defer a.trace(0)
	defer b.trace(0)
	initial := a.await(0, "player.state", "")
	scope := initial["idempotencyScopeId"].(string)
	require.Equal(t, float64(60000), initial["idempotencyTtlMs"])
	mutation := func(id string) map[string]any { return map[string]any{"mutationId": id, "idempotencyScopeId": scope} }

	t.Run("normal_and_cross_client_convergence", func(t *testing.T) {
		start := a.count()
		a.send("enqueue", "enqueue", map[string]any{"platform": "local", "musicId": "synthetic-track", "mutationId": "enqueue"})
		a.await(start, "enqueue.ack", "")
		ack := a.control("toggle-pause", "pause", mutation("pause"))
		require.Equal(t, "applied", ack["outcome"])
		for _, peer := range []*controlPeer{a, b} {
			state := peer.snapshot()
			require.Equal(t, true, state["isPaused"])
			assertControlWatermark(t, state, ack)
		}
	})
	t.Run("replay_conflict_and_actor_scope", func(t *testing.T) {
		before := a.snapshot()
		first := a.control("toggle-shuffle", "first", mutation("same"))
		replay := a.control("toggle-shuffle", "retry", mutation("same"))
		require.Equal(t, true, replay["replayed"])
		require.Equal(t, first["committed"], replay["committed"])
		after := b.snapshot()
		require.Equal(t, before["stateVersion"].(float64)+1, after["stateVersion"])
		require.Equal(t, "MUTATION_CONFLICT", a.control("next", "conflict", mutation("same"))["code"])
		other := b.control("toggle-shuffle", "other-actor", mutation("same"))
		require.Equal(t, "applied", other["outcome"])
		require.Equal(t, false, other["replayed"])
		require.Equal(t, before["isShuffle"], a.snapshot()["isShuffle"])
	})
	t.Run("seek_permission_payload_and_epoch", func(t *testing.T) {
		before := a.snapshot()
		epoch := before["playEpoch"]
		require.Equal(t, "SEEK_FORBIDDEN", b.control("seek", "forbidden", map[string]any{"positionMs": 0, "expectedPlayEpoch": epoch})["code"])
		for index, payload := range []map[string]any{{}, {"positionMs": nil}, {"positionMs": "0"}, {"positionMs": 1.5}} {
			require.Equal(t, "PAYLOAD_INVALID", a.control("seek", fmt.Sprintf("invalid-%d", index), payload)["code"])
		}
		payload := mutation("seek")
		payload["positionMs"] = 0
		payload["expectedPlayEpoch"] = epoch
		first := a.control("seek", "seek", payload)
		require.Equal(t, "applied", first["outcome"])
		replay := a.control("seek", "seek-replay", payload)
		require.Equal(t, true, replay["replayed"])
		require.Equal(t, first["committed"], replay["committed"])
		require.Equal(t, "PRECONDITION_FAILED", a.control("like", "stale-like", map[string]any{"expectedPlayEpoch": epoch})["code"])
		assertControlWatermark(t, b.snapshot(), first)
	})
	t.Run("noop_and_next_empty_does_not_require_epoch_growth", func(t *testing.T) {
		like := a.control("like", "like", map[string]any{})
		require.Equal(t, "applied", like["outcome"])
		noop := a.control("like", "like-again", mutation("noop"))
		require.Equal(t, "noop", noop["outcome"])
		assertControlWatermark(t, b.snapshot(), noop)
		require.Equal(t, noop["committed"], a.control("like", "noop-replay", mutation("noop"))["committed"])
		before := a.snapshot()
		next := a.control("next", "empty-next", mutation("next"))
		require.Equal(t, "applied", next["outcome"])
		after := b.snapshot()
		require.Nil(t, after["nowPlaying"])
		require.Equal(t, before["playEpoch"], after["playEpoch"])
		assertControlWatermark(t, after, next)
		require.Equal(t, "noop", a.control("like", "empty-like", map[string]any{})["outcome"])
	})
	t.Run("two_inflight_commands_and_same_actor_deduplication", func(t *testing.T) {
		before := a.snapshot()
		startA, startB := a.count(), b.count()
		// Neither sender waits for the other's ACK. The server serializes both,
		// and the later toggle may overwrite the earlier acknowledged effect.
		a.send("control.toggle-shuffle", "concurrent-a", mutation("concurrent-a"))
		b.send("control.toggle-shuffle", "concurrent-b", mutation("concurrent-b"))
		ackA := a.await(startA, "control.ack", "concurrent-a")
		ackB := b.await(startB, "control.ack", "concurrent-b")
		require.Equal(t, "applied", ackA["outcome"])
		require.Equal(t, "applied", ackB["outcome"])
		after := b.snapshot()
		require.Equal(t, before["isShuffle"], after["isShuffle"])
		require.Equal(t, before["stateVersion"].(float64)+2, after["stateVersion"])
		assertControlWatermark(t, after, ackA)
		assertControlWatermark(t, after, ackB)
		require.NotEqual(t, ackA["committed"], ackB["committed"])
		sameActor := connectControlPeer(t, baseURL, "lounge", "A-tab", firstHTTP)
		defer sameActor.trace(0)
		startA, startB = a.count(), sameActor.count()
		a.send("control.toggle-shuffle", "dedupe-a", mutation("concurrent-same"))
		sameActor.send("control.toggle-shuffle", "dedupe-tab", mutation("concurrent-same"))
		ackA = a.await(startA, "control.ack", "dedupe-a")
		ackB = sameActor.await(startB, "control.ack", "dedupe-tab")
		require.Equal(t, "applied", ackA["outcome"])
		require.Equal(t, "applied", ackB["outcome"])
		require.NotEqual(t, ackA["replayed"], ackB["replayed"])
		require.Equal(t, ackA["committed"], ackB["committed"])
		require.Equal(t, after["stateVersion"].(float64)+1, b.snapshot()["stateVersion"])
	})
	t.Run("three_locks_and_legacy_denial_event", func(t *testing.T) {
		locked := connectControlPeer(t, baseURL, "locked", "locked", firstHTTP)
		defer locked.trace(0)
		for kind, code := range map[string]string{"toggle-pause": "PAUSE_LOCKED", "next": "SKIP_LOCKED", "toggle-shuffle": "SHUFFLE_LOCKED"} {
			start := locked.count()
			require.Equal(t, code, locked.control(kind, kind, map[string]any{})["code"])
			event := locked.await(start, "player.events", "")
			require.Equal(t, "CONTROL_DENIED", event["code"])
		}
	})
	t.Run("disconnect_reconnect_and_room_scope", func(t *testing.T) {
		a.close()
		reconnected := connectControlPeer(t, baseURL, "lounge", "A2", firstHTTP)
		defer reconnected.trace(0)
		require.Equal(t, scope, reconnected.await(0, "player.state", "")["idempotencyScopeId"])
		replay := reconnected.control("toggle-shuffle", "after-reconnect", mutation("same"))
		require.Equal(t, true, replay["replayed"])
		other := connectControlPeer(t, baseURL, "other", "other-room", firstHTTP)
		defer other.trace(0)
		require.NotEqual(t, scope, other.await(0, "player.state", "")["idempotencyScopeId"])
		require.Equal(t, "IDEMPOTENCY_SCOPE_MISMATCH", other.control("toggle-shuffle", "old-scope", mutation("same"))["code"])
		// A rejected target room must not invalidate the original room connection.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		bad, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(baseURL, "http")+"/api/desktop/v1/ws?roomId=missing",
			&websocket.DialOptions{HTTPClient: firstHTTP, HTTPHeader: http.Header{"Origin": {baseURL}}})
		require.NoError(t, err)
		_, _, err = bad.Read(ctx)
		require.Equal(t, websocket.StatusPolicyViolation, websocket.CloseStatus(err))
		_ = bad.CloseNow()
		require.Equal(t, "applied", reconnected.control("toggle-shuffle", "old-room-still-live", mutation("after-failed-switch"))["outcome"])
	})
	t.Run("capacity_rejects_without_eviction", func(t *testing.T) {
		peer := connectControlPeer(t, baseURL, "capacity", "capacity", firstHTTP)
		capacityScope := peer.await(0, "player.state", "")["idempotencyScopeId"]
		payload := func(id string) map[string]any {
			return map[string]any{"mutationId": id, "idempotencyScopeId": capacityScope}
		}
		var first map[string]any
		var firstCompleted time.Time
		for index := 0; index < 256; index++ {
			ack := peer.control("like", fmt.Sprintf("fill-%d", index), payload(fmt.Sprintf("slot-%d", index)))
			require.Equal(t, "noop", ack["outcome"])
			if index == 0 {
				first = ack
				firstCompleted = time.Now()
			}
		}
		full := peer.control("toggle-shuffle", "capacity-full", payload("overflow"))
		require.Equal(t, "IDEMPOTENCY_CAPACITY", full["code"])
		replay := peer.control("like", "first-survives", payload("slot-0"))
		require.Equal(t, true, replay["replayed"])
		require.Equal(t, first["committed"], replay["committed"])
		t.Log("capacity: 256 noops cached, next mutation refused, first result still replayable")
		// Use the real advertised TTL, not a shortened clock or a test constant.
		// Replaying below must not extend the original expiry deadline.
		t.Log("expiry: waiting for the advertised 60 second window")
		time.Sleep(time.Until(firstCompleted.Add(60*time.Second + 100*time.Millisecond)))
		expired := peer.control("toggle-shuffle", "expired-can-change-fingerprint", payload("slot-0"))
		require.Equal(t, "applied", expired["outcome"])
		require.Equal(t, false, expired["replayed"])
		assertControlWatermark(t, peer.snapshot(), expired)
		t.Log("expiry: original mutation key executed again after TTL with a new fingerprint")
	})
	t.Run("commit_failure_is_unknown_and_resync_preserves_state", func(t *testing.T) {
		peer := connectControlPeer(t, baseURL, "fault", "fault", firstHTTP)
		defer peer.trace(0)
		before := peer.await(0, "player.state", "")
		payload := map[string]any{"mutationId": "uncertain", "idempotencyScopeId": before["idempotencyScopeId"]}
		start := peer.count()
		peer.send("control.toggle-shuffle", "commit-failure", payload)
		// This verifies the real server's silence past the desktop deadline. The
		// desktop adapter's unknown/pending timer is checked separately, not here.
		time.Sleep(2100 * time.Millisecond)
		for _, frame := range peer.framesSince(start) {
			require.NotEqual(t, "control.ack", frame.Type)
			require.NotEqual(t, "player.state", frame.Type, "failed commit must not broadcast state")
		}
		after := peer.snapshot()
		require.Equal(t, before["stateVersion"], after["stateVersion"])
		require.Equal(t, before["isShuffle"], after["isShuffle"])
		peer.send("control.toggle-shuffle", "unknown-replay", payload)
		// A subsequent valid command provides a serialized barrier, so absence
		// of an ACK for the unknown replay is observed after it was dispatched.
		require.Equal(t, "noop", peer.control("like", "barrier", map[string]any{})["outcome"])
		for _, frame := range peer.framesSince(start) {
			require.NotEqual(t, "unknown-replay", frame.RequestID)
		}
	})
	t.Run("runtime_rebuild_rejects_previous_scope", func(t *testing.T) {
		peer := connectControlPeer(t, baseURL, "idle", "idle-first", firstHTTP)
		oldScope := peer.await(0, "player.state", "")["idempotencyScopeId"]
		payload := map[string]any{"mutationId": "old", "idempotencyScopeId": oldScope}
		require.Equal(t, "applied", peer.control("toggle-shuffle", "before-idle", payload)["outcome"])
		peer.close()
		time.Sleep(1500 * time.Millisecond)
		rebuilt := connectControlPeer(t, baseURL, "idle", "idle-rebuilt", firstHTTP)
		defer rebuilt.trace(0)
		require.NotEqual(t, oldScope, rebuilt.await(0, "player.state", "")["idempotencyScopeId"])
		require.Equal(t, "IDEMPOTENCY_SCOPE_MISMATCH", rebuilt.control("toggle-shuffle", "stale-runtime", payload)["code"])
		require.Equal(t, true, rebuilt.snapshot()["isShuffle"])
	})
}
