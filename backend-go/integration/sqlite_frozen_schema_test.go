package integration

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	_ "modernc.org/sqlite"
)

// TestSQLiteFrozenSchemaCompatibility proves that current Go repositories can
// read and write a database created from the frozen pre-Go production schema.
func TestSQLiteFrozenSchemaCompatibility(t *testing.T) {
	repositoryRoot, err := findRepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	databasePath := filepath.Join(t.TempDir(), "frozen-schema.db")
	schema, err := os.ReadFile(filepath.Join(repositoryRoot, "contracts", "db", "schema.sql"))
	if err != nil {
		t.Fatal(err)
	}
	database, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(string(schema)); err != nil {
		_ = database.Close()
		t.Fatalf("apply frozen schema: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := storesqlite.OpenStore(context.Background(), storesqlite.StoreConfig{
		Path:               databasePath,
		BusyTimeout:        time.Second,
		ReadConnections:    2,
		WriteQueueCapacity: 100,
	})
	if err != nil {
		t.Fatalf("OpenStore(frozen fixture): %v", err)
	}
	ctx := context.Background()
	rooms := storesqlite.NewRoomRepository(store)
	profiles := storesqlite.NewUserProfileRepository(store, nil)
	accounts := storesqlite.NewUserAccountRepository(store)
	access := storesqlite.NewRoomAccessRepository(store)
	queue := storesqlite.NewQueueRepository(store, nil)
	playback := storesqlite.NewPlaybackStateRepository(store)
	chat := storesqlite.NewChatRepository(store)
	settings := storesqlite.NewSiteSettingRepository(store)
	subsonic := storesqlite.NewSubsonicSourceRepository(store)
	local := storesqlite.NewLocalTrackRepository(store)

	profile := storesqlite.UserProfile{
		PublicID:      "go-roundtrip-user",
		DisplayName:   "Go Roundtrip User",
		CurrentRoomID: "go-roundtrip-room",
		CreatedAt:     1000,
		LastSeenAt:    1000,
	}
	if err := profiles.UpsertProfile(ctx, profile); err != nil {
		t.Fatalf("Go seed profile: %v", err)
	}
	room := storesqlite.Room{
		ID:            "go-roundtrip-room",
		Name:          "Go Roundtrip Room",
		OwnerPublicID: profile.PublicID,
		Visibility:    "PRIVATE",
		CreatedAt:     1000,
		LastActiveAt:  1000,
	}
	if err := rooms.Upsert(ctx, room); err != nil {
		t.Fatalf("Go seed room: %v", err)
	}
	if err := access.UpsertMembership(ctx, storesqlite.RoomMembership{
		RoomID: room.ID, PublicID: profile.PublicID, Role: "OWNER", CreatedAt: 1000, UpdatedAt: 1000,
	}); err != nil {
		t.Fatalf("Go seed membership: %v", err)
	}
	if err := accounts.Create(ctx, storesqlite.UserAccount{
		Username: "go-roundtrip", PublicID: profile.PublicID, PasswordHash: "fixture-hash", Role: "USER", Enabled: true, CreatedAt: 1000, UpdatedAt: 1000,
	}); err != nil {
		t.Fatalf("Go seed account: %v", err)
	}
	music := storesqlite.Music{ID: "go-roundtrip-music", Name: "往返歌曲", Artists: []string{"Go"}, Duration: 1234, Platform: "local", CoverURL: "cover"}
	queueItem := storesqlite.QueueItem{QueueID: "go-roundtrip-queue", Music: music, EnqueuedBy: storesqlite.UserSummary{PublicID: profile.PublicID, Name: profile.DisplayName}, Status: "READY"}
	if err := queue.ReplaceQueue(ctx, room.ID, []storesqlite.QueueItem{queueItem}); err != nil {
		t.Fatalf("Go seed queue: %v", err)
	}
	if err := queue.AppendHistory(ctx, storesqlite.HistoryEntry{ID: "go-roundtrip-history", RoomID: room.ID, Music: music, PlayedAt: 1100}); err != nil {
		t.Fatalf("Go seed history: %v", err)
	}
	playable := storesqlite.PlayableMusic{ID: music.ID, Name: music.Name, Artists: music.Artists, Duration: music.Duration, Platform: music.Platform, URL: "/media", CoverURL: music.CoverURL}
	if err := playback.Upsert(ctx, storesqlite.PlaybackState{RoomID: room.ID, CurrentMusic: &playable, LikedUserIDs: map[string]struct{}{profile.PublicID: {}}, LikeMarkers: []int64{100}, StateVersion: 1, LastPersistedAt: 1200}); err != nil {
		t.Fatalf("Go seed playback: %v", err)
	}
	roomID := room.ID
	if err := chat.AppendMessage(ctx, &roomID, storesqlite.ChatMessage{ID: "go-roundtrip-chat", UserID: profile.PublicID, UserName: profile.DisplayName, Content: "Go chat", Timestamp: 1300, Type: "CHAT"}); err != nil {
		t.Fatalf("Go seed chat: %v", err)
	}
	roomPlaylists := storesqlite.NewRoomPlaylistRepository(store, func() time.Time { return time.UnixMilli(1400) }, func() string { return "go-roundtrip-room-playlist" })
	rp, err := roomPlaylists.CreatePlaylist(ctx, room.ID, "Go Room Playlist")
	if err != nil {
		t.Fatalf("Go seed room playlist: %v", err)
	}
	roomPlaylists = storesqlite.NewRoomPlaylistRepository(store, func() time.Time { return time.UnixMilli(1401) }, func() string { return "go-roundtrip-room-track" })
	if _, err := roomPlaylists.AddTrack(ctx, room.ID, rp.ID, &music); err != nil {
		t.Fatalf("Go seed room playlist track: %v", err)
	}
	userPlaylists := storesqlite.NewUserPlaylistRepository(store, func() time.Time { return time.UnixMilli(1500) }, func() string { return "go-roundtrip-user-playlist" })
	up, err := userPlaylists.CreatePlaylist(ctx, profile.PublicID, "Go User Playlist")
	if err != nil {
		t.Fatalf("Go seed user playlist: %v", err)
	}
	userPlaylists = storesqlite.NewUserPlaylistRepository(store, func() time.Time { return time.UnixMilli(1501) }, func() string { return "go-roundtrip-user-track" })
	if _, err := userPlaylists.AddTrackIfAbsent(ctx, profile.PublicID, up.ID, &music); err != nil {
		t.Fatalf("Go seed user playlist track: %v", err)
	}
	settingValue := "go-value"
	if err := settings.Upsert(ctx, storesqlite.SiteSetting{Key: "go.roundtrip", Value: &settingValue, Secret: true, UpdatedAt: 1600}); err != nil {
		t.Fatalf("Go seed site setting: %v", err)
	}
	ownerRoomID := room.ID
	if err := subsonic.Upsert(ctx, storesqlite.SubsonicSource{ID: "go-roundtrip-subsonic", OwnerRoomID: &ownerRoomID, Label: "Go Subsonic", BaseURL: "https://example.invalid", Username: "user", Password: "password", Client: "musicparty", APIVersion: "1.16.1", Enabled: true, CreatedAt: 1700, UpdatedAt: 1700}); err != nil {
		t.Fatalf("Go seed Subsonic source: %v", err)
	}
	if err := subsonic.UpsertRoomBinding(ctx, storesqlite.RoomSubsonicSource{RoomID: room.ID, SourceID: "go-roundtrip-subsonic", Enabled: true, CreatedAt: 1701, UpdatedAt: 1701}); err != nil {
		t.Fatalf("Go seed Subsonic binding: %v", err)
	}
	originalHash := "go-roundtrip-hash"
	if err := local.Upsert(ctx, storesqlite.LocalTrack{ID: music.ID, OriginalHash: &originalHash, Title: music.Name, Artists: music.Artists, DurationMS: music.Duration, Status: "COMPLETED", CreatedAt: 1800, UpdatedAt: 1800}); err != nil {
		t.Fatalf("Go seed local track: %v", err)
	}
	if err := local.GrantUploadUser(ctx, "go-roundtrip-uploader", 1801); err != nil {
		t.Fatalf("Go seed upload grant: %v", err)
	}
	result, err := storesqlite.Check(ctx, store.Reader())
	if err != nil {
		t.Fatalf("Check(frozen fixture after Go writes): %v", err)
	}
	if !result.OK() || result.ApplicationTables != 23 {
		t.Fatalf("frozen fixture after Go writes is unhealthy: %+v", result)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := storesqlite.OpenStore(ctx, storesqlite.StoreConfig{
		Path:               databasePath,
		BusyTimeout:        time.Second,
		ReadConnections:    2,
		WriteQueueCapacity: 100,
	})
	if err != nil {
		t.Fatalf("reopen frozen fixture: %v", err)
	}
	defer reopened.Close()
	reopenedRoom, err := storesqlite.NewRoomRepository(reopened).FindByID(ctx, room.ID)
	if err != nil {
		t.Fatalf("read room after reopen: %v", err)
	}
	if reopenedRoom.Name != room.Name || reopenedRoom.OwnerPublicID != profile.PublicID {
		t.Fatalf("room changed after reopen: %+v", reopenedRoom)
	}
	reopenedQueue, err := storesqlite.NewQueueRepository(reopened, nil).LoadQueue(ctx, room.ID)
	if err != nil {
		t.Fatalf("read queue after reopen: %v", err)
	}
	if len(reopenedQueue) != 1 || reopenedQueue[0].Music.ID != music.ID {
		t.Fatalf("queue changed after reopen: %+v", reopenedQueue)
	}
	reopenedCheck, err := storesqlite.Check(ctx, reopened.Reader())
	if err != nil || !reopenedCheck.OK() || reopenedCheck.ApplicationTables != 23 {
		t.Fatalf("reopened frozen fixture is unhealthy: result=%+v err=%v", reopenedCheck, err)
	}
}
