package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/coder/websocket"
)

type catalog struct {
	Scenarios []httpScenario `json:"scenarios"`
}

type httpScenario struct {
	ID             string            `json:"id"`
	Action         string            `json:"action"`
	Method         string            `json:"method"`
	Path           string            `json:"path"`
	Query          map[string]string `json:"query"`
	Body           map[string]any    `json:"body"`
	OmitCSRF       bool              `json:"omitCsrf"`
	ExpectedStatus int               `json:"expectedStatus"`
}

type baseline struct {
	Version   int            `json:"version"`
	HTTP      []httpResult   `json:"http"`
	WebSocket []socketResult `json:"webSocket"`
}

type httpResult struct {
	ID      string            `json:"id"`
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Query   map[string]string `json:"query,omitempty"`
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
	Cookies []cookieResult    `json:"cookies"`
	Body    any               `json:"body"`
}

type cookieResult struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	MaxAge   int    `json:"maxAge"`
	HttpOnly bool   `json:"httpOnly"`
	Secure   bool   `json:"secure"`
	SameSite string `json:"sameSite"`
}

type socketResult struct {
	ID        string `json:"id"`
	CloseCode int    `json:"closeCode,omitempty"`
	Messages  []any  `json:"messages,omitempty"`
}

var normalizedFields = map[string]string{
	"publicId": "<public-id>", "ownerPublicId": "<public-id>", "userId": "<public-id>", "sessionToken": "<session-token>", "lastLoginAt": "<timestamp>",
	"createdAt": "<timestamp>", "updatedAt": "<timestamp>", "expiresAt": "<timestamp>",
	"serverReceiveTime": "<timestamp>", "serverSendTime": "<timestamp>", "serverTimestamp": "<timestamp>", "timestamp": "<timestamp>",
	"joinedAt": "<timestamp>", "lastSeenAt": "<timestamp>", "usedAt": "<timestamp>", "revokedAt": "<timestamp>",
}

var uuidPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func main() {
	baseURL := flag.String("base-url", "http://127.0.0.1:8080", "running Java baseline URL")
	adminUser := flag.String("admin-user", "contract-admin", "isolated bootstrap administrator")
	adminPassword := flag.String("admin-password", "Contract-Password-123!", "isolated bootstrap password")
	mode := flag.String("mode", "compare", "capture or compare")
	repository := flag.String("repo", "..", "MusicParty repository root")
	flag.Parse()

	root, err := filepath.Abs(*repository)
	if err != nil {
		fatal(err)
	}
	actual, err := probe(context.Background(), root, *baseURL, *adminUser, *adminPassword)
	if err != nil {
		fatal(err)
	}
	encoded, err := json.MarshalIndent(actual, "", "  ")
	if err != nil {
		fatal(err)
	}
	encoded = append(encoded, '\n')
	goldenPath := filepath.Join(root, "contracts", "http", "golden", "java-baseline.json")
	switch *mode {
	case "capture":
		if err := os.WriteFile(goldenPath, encoded, 0o644); err != nil {
			fatal(err)
		}
		fmt.Printf("captured Java baseline: %s\n", goldenPath)
	case "compare":
		expected, err := os.ReadFile(goldenPath)
		if err != nil {
			fatal(err)
		}
		if !bytes.Equal(expected, encoded) {
			actualPath := goldenPath + ".actual"
			_ = os.WriteFile(actualPath, encoded, 0o644)
			fatal(fmt.Errorf("java contract differs from golden; actual written to %s", actualPath))
		}
		fmt.Println("HTTP/WebSocket endpoint matches frozen Java golden")
	default:
		fatal(fmt.Errorf("unknown mode %q", *mode))
	}
}

func probe(ctx context.Context, root, rawBaseURL, adminUser, adminPassword string) (baseline, error) {
	base, err := url.Parse(strings.TrimRight(rawBaseURL, "/"))
	if err != nil {
		return baseline{}, err
	}
	content, err := os.ReadFile(filepath.Join(root, "contracts", "http", "golden", "scenarios.json"))
	if err != nil {
		return baseline{}, err
	}
	var scenarios catalog
	if err := json.Unmarshal(content, &scenarios); err != nil {
		return baseline{}, err
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return baseline{}, err
	}
	client := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	result := baseline{Version: 1, HTTP: []httpResult{}, WebSocket: []socketResult{}}
	logoutIndex := len(scenarios.Scenarios)
	for index, scenario := range scenarios.Scenarios {
		if scenario.ID == "logout" {
			logoutIndex = index
			break
		}
		entry, err := runHTTP(ctx, client, base, scenario, adminUser, adminPassword)
		if err != nil {
			return result, err
		}
		result.HTTP = append(result.HTTP, entry)
	}
	invitationResults, err := runInvitationFlow(ctx, client, base)
	if err != nil {
		return result, err
	}
	result.HTTP = append(result.HTTP, invitationResults...)
	playlistResults, err := runUserPlaylistFlow(ctx, client, base)
	if err != nil {
		return result, err
	}
	result.HTTP = append(result.HTTP, playlistResults...)
	ws, err := runWebSocket(ctx, base, jar)
	if err != nil {
		return result, err
	}
	result.WebSocket = ws
	for _, scenario := range scenarios.Scenarios[logoutIndex:] {
		entry, err := runHTTP(ctx, client, base, scenario, adminUser, adminPassword)
		if err != nil {
			return result, err
		}
		result.HTTP = append(result.HTTP, entry)
	}
	return result, nil
}

func runHTTP(ctx context.Context, client *http.Client, base *url.URL, scenario httpScenario, adminUser, adminPassword string) (httpResult, error) {
	body := scenario.Body
	if scenario.Action == "login" {
		body = map[string]any{"username": adminUser, "password": adminPassword}
	}
	entry, _, err := requestHTTP(ctx, client, base, scenario.ID, scenario.Method, scenario.Path, scenario.Query, body, scenario.OmitCSRF, scenario.ExpectedStatus)
	return entry, err
}

func requestHTTP(ctx context.Context, client *http.Client, base *url.URL, id, method, path string, queryValues map[string]string, body map[string]any, omitCSRF bool, expectedStatus int) (httpResult, any, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return httpResult{}, nil, err
		}
		reader = bytes.NewReader(encoded)
	}
	target := base.ResolveReference(&url.URL{Path: path})
	query := target.Query()
	for name, value := range queryValues {
		query.Set(name, value)
	}
	target.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, method, target.String(), reader)
	if err != nil {
		return httpResult{}, nil, err
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if !omitCSRF && method != http.MethodGet && method != http.MethodHead && path != "/api/account/login" && path != "/api/invites/redeem" {
		for _, cookie := range client.Jar.Cookies(base) {
			if cookie.Name == "MP_CSRF" {
				request.Header.Set("X-CSRF-Token", cookie.Value)
			}
		}
	}
	response, err := client.Do(request)
	if err != nil {
		return httpResult{}, nil, fmt.Errorf("%s: %w", id, err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return httpResult{}, nil, err
	}
	if response.StatusCode != expectedStatus {
		return httpResult{}, nil, fmt.Errorf("%s: status %d, expected %d; body=%s", id, response.StatusCode, expectedStatus, responseBody)
	}
	rawBody := decodeBody(responseBody, response.Header.Get("Content-Type"))
	entry := httpResult{ID: id, Method: method, Path: path, Query: queryValues, Status: response.StatusCode, Headers: selectedHeaders(response.Header), Cookies: cookieContracts(response.Cookies()), Body: normalize(rawBody, "")}
	return entry, rawBody, nil
}

func runInvitationFlow(ctx context.Context, adminClient *http.Client, base *url.URL) ([]httpResult, error) {
	results := []httpResult{}
	created, rawCreated, err := requestHTTP(ctx, adminClient, base, "invite-create", http.MethodPost, "/api/rooms/lounge/invites", nil, map[string]any{"label": "契约邀请"}, false, http.StatusOK)
	if err != nil {
		return nil, err
	}
	results = append(results, created)
	createdObject, ok := rawCreated.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invite-create body is not an object")
	}
	secret, _ := createdObject["secret"].(string)
	if secret == "" {
		return nil, fmt.Errorf("invite-create did not return a secret")
	}
	metadataPath := "/api/join/" + url.PathEscape(secret) + "/metadata"
	metadata, _, err := requestHTTP(ctx, adminClient, base, "invite-metadata", http.MethodGet, metadataPath, nil, nil, true, http.StatusOK)
	if err != nil {
		return nil, err
	}
	metadata.Path = "/api/join/{secret}/metadata"
	results = append(results, metadata)

	memberJar, _ := cookiejar.New(nil)
	memberClient := &http.Client{Jar: memberJar, Timeout: 10 * time.Second}
	redeemed, rawRedeemed, err := requestHTTP(ctx, memberClient, base, "invite-redeem", http.MethodPost, "/api/invites/redeem", nil, map[string]any{"secret": secret, "displayName": "契约成员"}, true, http.StatusOK)
	if err != nil {
		return nil, err
	}
	results = append(results, redeemed)
	redeemedObject, _ := rawRedeemed.(map[string]any)
	memberPublicID, _ := redeemedObject["publicId"].(string)

	repeatedJar, _ := cookiejar.New(nil)
	repeatedClient := &http.Client{Jar: repeatedJar, Timeout: 10 * time.Second}
	repeated, _, err := requestHTTP(ctx, repeatedClient, base, "invite-redeem-repeated", http.MethodPost, "/api/invites/redeem", nil, map[string]any{"secret": secret, "displayName": "重复兑换"}, true, http.StatusUnauthorized)
	if err != nil {
		return nil, err
	}
	results = append(results, repeated)

	listed, _, err := requestHTTP(ctx, adminClient, base, "invite-list", http.MethodGet, "/api/rooms/lounge/invites", nil, nil, true, http.StatusOK)
	if err != nil {
		return nil, err
	}
	results = append(results, listed)
	members, _, err := requestHTTP(ctx, adminClient, base, "member-list", http.MethodGet, "/api/rooms/lounge/members", nil, nil, true, http.StatusOK)
	if err != nil {
		return nil, err
	}
	results = append(results, members)
	if memberPublicID != "" {
		removed, _, err := requestHTTP(ctx, adminClient, base, "member-remove", http.MethodDelete, "/api/rooms/lounge/members/"+url.PathEscape(memberPublicID), nil, nil, false, http.StatusNoContent)
		if err != nil {
			return nil, err
		}
		removed.Path = "/api/rooms/lounge/members/{publicId}"
		results = append(results, removed)
	}
	return results, nil
}

func runUserPlaylistFlow(ctx context.Context, client *http.Client, base *url.URL) ([]httpResult, error) {
	sessionToken := cookieValue(client.Jar, base, "MP_SESSION")
	if sessionToken == "" {
		return nil, fmt.Errorf("user playlist flow has no MP_SESSION cookie")
	}
	authQuery := map[string]string{"sessionToken": sessionToken}
	results := []httpResult{}
	created, rawCreated, err := requestHTTP(ctx, client, base, "user-playlist-create", http.MethodPost, "/api/me/playlists", authQuery, map[string]any{"name": "契约歌单 🎶"}, false, http.StatusOK)
	if err != nil {
		return nil, err
	}
	created.Query = map[string]string{"sessionToken": "<session-token>"}
	results = append(results, created)
	createdObject, _ := rawCreated.(map[string]any)
	playlistID, _ := createdObject["id"].(string)
	if playlistID == "" {
		return nil, fmt.Errorf("user-playlist-create did not return an id")
	}

	listed, _, err := requestHTTP(ctx, client, base, "user-playlist-list", http.MethodGet, "/api/me/playlists", authQuery, nil, true, http.StatusOK)
	if err != nil {
		return nil, err
	}
	listed.Query = map[string]string{"sessionToken": "<session-token>"}
	results = append(results, listed)
	playlistPath := "/api/me/playlists/" + url.PathEscape(playlistID)
	renamed, _, err := requestHTTP(ctx, client, base, "user-playlist-rename", http.MethodPatch, playlistPath, authQuery, map[string]any{"name": "契约歌单已重命名"}, false, http.StatusOK)
	if err != nil {
		return nil, err
	}
	renamed.Path = "/api/me/playlists/{playlistId}"
	renamed.Query = map[string]string{"sessionToken": "<session-token>"}
	results = append(results, renamed)

	musics := []any{
		map[string]any{"id": "contract-song-1", "name": "第一首歌", "artists": []string{"甲", "乙"}, "duration": 123000, "platform": "local", "coverUrl": ""},
		map[string]any{"id": "contract-song-2", "name": "第二首歌 🎵", "artists": []string{"丙"}, "duration": 456000, "platform": "local", "coverUrl": "https://example.invalid/cover.jpg"},
	}
	batchPath := playlistPath + "/tracks/batch"
	added, _, err := requestHTTP(ctx, client, base, "user-playlist-add-tracks", http.MethodPost, batchPath, authQuery, map[string]any{"musics": musics}, false, http.StatusOK)
	if err != nil {
		return nil, err
	}
	added.Path = "/api/me/playlists/{playlistId}/tracks/batch"
	added.Query = map[string]string{"sessionToken": "<session-token>"}
	results = append(results, added)
	tracksPath := playlistPath + "/tracks"
	tracks, rawTracks, err := requestHTTP(ctx, client, base, "user-playlist-tracks", http.MethodGet, tracksPath, map[string]string{"sessionToken": sessionToken, "offset": "0", "limit": "100"}, nil, true, http.StatusOK)
	if err != nil {
		return nil, err
	}
	tracks.Path = "/api/me/playlists/{playlistId}/tracks"
	tracks.Query = map[string]string{"sessionToken": "<session-token>", "offset": "0", "limit": "100"}
	results = append(results, tracks)
	trackIDs := []string{}
	if list, ok := rawTracks.([]any); ok {
		for _, item := range list {
			if object, ok := item.(map[string]any); ok {
				if id, ok := object["id"].(string); ok {
					trackIDs = append(trackIDs, id)
				}
			}
		}
	}
	if len(trackIDs) != 2 {
		return nil, fmt.Errorf("user-playlist-tracks returned %d track ids, expected 2", len(trackIDs))
	}
	reordered, _, err := requestHTTP(ctx, client, base, "user-playlist-reorder", http.MethodPost, playlistPath+"/tracks/reorder", authQuery, map[string]any{"trackIds": []string{trackIDs[1], trackIDs[0]}}, false, http.StatusOK)
	if err != nil {
		return nil, err
	}
	reordered.Path = "/api/me/playlists/{playlistId}/tracks/reorder"
	reordered.Query = map[string]string{"sessionToken": "<session-token>"}
	results = append(results, reordered)
	exported, _, err := requestHTTP(ctx, client, base, "user-playlist-export", http.MethodGet, playlistPath+"/export", map[string]string{"sessionToken": sessionToken, "format": "json"}, nil, true, http.StatusOK)
	if err != nil {
		return nil, err
	}
	exported.Path = "/api/me/playlists/{playlistId}/export"
	exported.Query = map[string]string{"sessionToken": "<session-token>", "format": "json"}
	results = append(results, exported)
	imported, _, err := requestHTTP(ctx, client, base, "user-playlist-import-upstream-failure", http.MethodPost, playlistPath+"/import", authQuery, map[string]any{"platform": "netease", "playlistId": "contract-external-playlist"}, false, http.StatusBadGateway)
	if err != nil {
		return nil, err
	}
	imported.Path = "/api/me/playlists/{playlistId}/import"
	imported.Query = map[string]string{"sessionToken": "<session-token>"}
	results = append(results, imported)

	likedPath := "/api/me/liked-songs/local/contract-liked"
	liked, _, err := requestHTTP(ctx, client, base, "liked-song-add", http.MethodPut, likedPath, authQuery, map[string]any{"id": "contract-liked", "name": "喜欢的歌", "artists": []string{"歌手"}, "duration": 120000, "platform": "local", "coverUrl": ""}, false, http.StatusOK)
	if err != nil {
		return nil, err
	}
	liked.Path = "/api/me/liked-songs/{platform}/{musicId}"
	liked.Query = map[string]string{"sessionToken": "<session-token>"}
	results = append(results, liked)
	likedList, _, err := requestHTTP(ctx, client, base, "liked-song-list", http.MethodGet, "/api/me/liked-songs", authQuery, nil, true, http.StatusOK)
	if err != nil {
		return nil, err
	}
	likedList.Query = map[string]string{"sessionToken": "<session-token>"}
	results = append(results, likedList)
	unliked, _, err := requestHTTP(ctx, client, base, "liked-song-delete", http.MethodDelete, likedPath, authQuery, nil, false, http.StatusOK)
	if err != nil {
		return nil, err
	}
	unliked.Path = "/api/me/liked-songs/{platform}/{musicId}"
	unliked.Query = map[string]string{"sessionToken": "<session-token>"}
	results = append(results, unliked)
	for index, trackID := range trackIDs {
		removedTrack, _, err := requestHTTP(ctx, client, base, fmt.Sprintf("user-playlist-track-delete-%d", index+1), http.MethodDelete, playlistPath+"/tracks/"+url.PathEscape(trackID), authQuery, nil, false, http.StatusOK)
		if err != nil {
			return nil, err
		}
		removedTrack.Path = "/api/me/playlists/{playlistId}/tracks/{trackId}"
		removedTrack.Query = map[string]string{"sessionToken": "<session-token>"}
		results = append(results, removedTrack)
	}
	enqueued, _, err := requestHTTP(ctx, client, base, "user-playlist-enqueue-empty-rejected", http.MethodPost, playlistPath+"/enqueue", authQuery, nil, false, http.StatusInternalServerError)
	if err != nil {
		return nil, err
	}
	enqueued.Path = "/api/me/playlists/{playlistId}/enqueue"
	enqueued.Query = map[string]string{"sessionToken": "<session-token>"}
	results = append(results, enqueued)

	deleted, _, err := requestHTTP(ctx, client, base, "user-playlist-delete", http.MethodDelete, playlistPath, authQuery, nil, false, http.StatusOK)
	if err != nil {
		return nil, err
	}
	deleted.Path = "/api/me/playlists/{playlistId}"
	deleted.Query = map[string]string{"sessionToken": "<session-token>"}
	results = append(results, deleted)
	return results, nil
}

func cookieValue(jar http.CookieJar, base *url.URL, name string) string {
	for _, cookie := range jar.Cookies(base) {
		if cookie.Name == name {
			return cookie.Value
		}
	}
	return ""
}

func runWebSocket(ctx context.Context, base *url.URL, jar http.CookieJar) ([]socketResult, error) {
	wsURL := *base
	if wsURL.Scheme == "https" {
		wsURL.Scheme = "wss"
	} else {
		wsURL.Scheme = "ws"
	}
	wsURL.Path = "/ws"
	wsURL.RawQuery = "room-id=lounge"
	origin := base.Scheme + "://" + base.Host

	unauthorized, _, err := websocket.Dial(ctx, wsURL.String(), &websocket.DialOptions{HTTPHeader: http.Header{"Origin": []string{origin}}})
	unauthorizedResult := socketResult{ID: "unauthorized-handshake"}
	if err == nil {
		readContext, cancel := context.WithTimeout(ctx, 3*time.Second)
		_, _, readErr := unauthorized.Read(readContext)
		cancel()
		unauthorizedResult.CloseCode = int(websocket.CloseStatus(readErr))
		_ = unauthorized.CloseNow()
	} else {
		unauthorizedResult.CloseCode = int(websocket.CloseStatus(err))
	}
	if unauthorizedResult.CloseCode != 1008 {
		return nil, fmt.Errorf("unauthorized websocket close code %d, expected 1008", unauthorizedResult.CloseCode)
	}

	headers := http.Header{"Origin": []string{origin}}
	for _, cookie := range jar.Cookies(base) {
		headers.Add("Cookie", cookie.String())
	}
	connection, _, err := websocket.Dial(ctx, wsURL.String(), &websocket.DialOptions{HTTPHeader: headers})
	if err != nil {
		return nil, fmt.Errorf("authorized websocket dial: %w", err)
	}
	defer connection.CloseNow()
	results := []socketResult{unauthorizedResult}
	results = append(results, socketResult{ID: "connect", Messages: []any{}})

	scenarios := []struct {
		id       string
		messages []map[string]any
		expect   []string
		delay    time.Duration
	}{
		{"resync", []map[string]any{{"type": "player.resync", "requestId": "resync-1", "roomId": "lounge", "payload": map[string]any{}}}, []string{"player.state"}, 0},
		{"ping", []map[string]any{{"type": "sync.ping", "requestId": "ping-1", "roomId": "lounge", "payload": map[string]any{"pingId": "contract-ping", "clientSendTime": 1234}}}, []string{"sync.pong"}, 0},
		{"identity-and-presence", []map[string]any{{"type": "user.me", "payload": map[string]any{}}, {"type": "users.online", "payload": map[string]any{}}}, []string{"user.me", "users.online"}, 0},
		{"queue-version-gap", []map[string]any{{"type": "queue.reorder", "requestId": "reorder-gap", "roomId": "lounge", "payload": map[string]any{"oldIndex": -1, "newIndex": 0, "mutationId": "reorder-gap"}}}, []string{"queue.reorder.nack"}, 0},
		{"chat-message", []map[string]any{{"type": "chat.message", "payload": map[string]any{"content": "契约聊天消息 🎵"}}}, []string{"chat.message"}, 0},
		{"chat-history", []map[string]any{{"type": "chat.history.fetch", "payload": map[string]any{"offset": 0, "limit": 20}}}, []string{"chat.history"}, 0},
		{"public-chat-message", []map[string]any{{"type": "public-chat.message", "payload": map[string]any{"content": "契约公共消息"}}}, []string{"public-chat.message"}, 1100 * time.Millisecond},
		{"public-chat-history", []map[string]any{{"type": "public-chat.history.fetch", "payload": map[string]any{"offset": 0, "limit": 20}}}, []string{"public-chat.history"}, 0},
	}
	for _, scenario := range scenarios {
		if scenario.delay > 0 {
			time.Sleep(scenario.delay)
		}
		for _, message := range scenario.messages {
			encoded, _ := json.Marshal(message)
			if err := connection.Write(ctx, websocket.MessageText, encoded); err != nil {
				return nil, fmt.Errorf("write websocket scenario %s: %w", scenario.id, err)
			}
		}
		messages, err := readExpectedMessages(ctx, connection, scenario.expect)
		if err != nil {
			return nil, fmt.Errorf("websocket scenario %s: %w", scenario.id, err)
		}
		results = append(results, socketResult{ID: scenario.id, Messages: messages})
	}
	if err := connection.Close(websocket.StatusNormalClosure, "contract reconnect"); err != nil {
		return nil, fmt.Errorf("close websocket before reconnect: %w", err)
	}
	reconnected, _, err := websocket.Dial(ctx, wsURL.String(), &websocket.DialOptions{HTTPHeader: headers})
	if err != nil {
		return nil, fmt.Errorf("reconnect websocket: %w", err)
	}
	defer reconnected.CloseNow()
	reconnectPayload, _ := json.Marshal(map[string]any{"type": "player.resync", "requestId": "reconnect-resync", "roomId": "lounge", "payload": map[string]any{}})
	if err := reconnected.Write(ctx, websocket.MessageText, reconnectPayload); err != nil {
		return nil, fmt.Errorf("write reconnect resync: %w", err)
	}
	reconnectMessages, err := readExpectedMessages(ctx, reconnected, []string{"player.state"})
	if err != nil {
		return nil, fmt.Errorf("read reconnect resync: %w", err)
	}
	results = append(results, socketResult{ID: "reconnect-resync", Messages: reconnectMessages})
	return results, nil
}

func readExpectedMessages(ctx context.Context, connection *websocket.Conn, expected []string) ([]any, error) {
	deadline, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	messages := []any{}
	remaining := append([]string(nil), expected...)
	for len(remaining) > 0 {
		_, payload, err := connection.Read(deadline)
		if err != nil {
			return messages, err
		}
		message := decodeAndNormalize(payload)
		messages = append(messages, message)
		object, _ := message.(map[string]any)
		messageType, _ := object["type"].(string)
		if messageType == remaining[0] {
			remaining = remaining[1:]
		}
	}
	return messages, nil
}

func selectedHeaders(headers http.Header) map[string]string {
	result := map[string]string{}
	for _, name := range []string{"Content-Type", "Retry-After", "Location", "Accept-Ranges", "Content-Range"} {
		if value := headers.Get(name); value != "" {
			result[strings.ToLower(name)] = value
		}
	}
	return result
}

func cookieContracts(cookies []*http.Cookie) []cookieResult {
	result := make([]cookieResult, 0, len(cookies))
	for _, cookie := range cookies {
		result = append(result, cookieResult{Name: cookie.Name, Path: cookie.Path, MaxAge: cookie.MaxAge, HttpOnly: cookie.HttpOnly, Secure: cookie.Secure, SameSite: sameSite(cookie.SameSite)})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func sameSite(value http.SameSite) string {
	switch value {
	case http.SameSiteLaxMode:
		return "Lax"
	case http.SameSiteStrictMode:
		return "Strict"
	case http.SameSiteNoneMode:
		return "None"
	default:
		return "Default"
	}
}

func decodeBody(content []byte, contentType string) any {
	if len(content) == 0 {
		return nil
	}
	if strings.Contains(contentType, "json") || json.Valid(content) {
		decoder := json.NewDecoder(bytes.NewReader(content))
		decoder.UseNumber()
		var value any
		if err := decoder.Decode(&value); err == nil {
			return value
		}
	}
	return string(content)
}

func decodeAndNormalize(content []byte) any {
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return string(content)
	}
	return normalize(value, "")
}

func normalize(value any, key string) any {
	if key == "secret" {
		return "<invite-secret>"
	}
	if key == "id" {
		if text, ok := value.(string); ok {
			if uuidPattern.MatchString(text) {
				return "<uuid>"
			}
			if strings.HasPrefix(text, "inv_") {
				return "<invite-id>"
			}
		}
	}
	if key == "playlistId" || key == "trackId" {
		if text, ok := value.(string); ok && uuidPattern.MatchString(text) {
			return "<uuid>"
		}
	}
	if replacement, ok := normalizedFields[key]; ok && value != nil && !reflect.ValueOf(value).IsZero() {
		return replacement
	}
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for childKey, child := range typed {
			result[childKey] = normalize(child, childKey)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index, child := range typed {
			result[index] = normalize(child, key)
		}
		return result
	default:
		return value
	}
}

func fatal(err error) {
	if err == nil || errors.Is(err, context.Canceled) {
		return
	}
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
