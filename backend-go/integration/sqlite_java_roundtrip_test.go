package integration

import (
	"context"
	"os"
	"testing"
	"time"

	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
)

// TestSQLiteJavaRoundTrip is opt-in because it requires a database initialized
// by the Java SqliteSchemaInitializer. The seed phase writes through the Go
// repositories; an external Java repository probe then updates the same rows;
// the verify phase confirms Go can read the Java mutation without migration.
func TestSQLiteJavaRoundTrip(t *testing.T) {
	databasePath := os.Getenv("MUSICPARTY_JAVA_SQLITE_FIXTURE")
	if databasePath == "" {
		t.Skip("MUSICPARTY_JAVA_SQLITE_FIXTURE is not set")
	}
	store, err := storesqlite.OpenStore(context.Background(), storesqlite.StoreConfig{
		Path:               databasePath,
		BusyTimeout:        time.Second,
		ReadConnections:    2,
		WriteQueueCapacity: 100,
	})
	if err != nil {
		t.Fatalf("OpenStore(Java fixture): %v", err)
	}
	defer store.Close()
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

	switch os.Getenv("MUSICPARTY_JAVA_SQLITE_PHASE") {
	case "verify-java":
		room, err := rooms.FindByID(ctx, "go-roundtrip-room")
		if err != nil {
			t.Fatalf("Go read Java-updated room: %v", err)
		}
		if room.LastActiveAt != 4242 {
			t.Fatalf("Java-updated last_active_at = %d, want 4242", room.LastActiveAt)
		}
		membership, err := access.FindMembership(ctx, "go-roundtrip-room", "java-roundtrip-user")
		if err != nil {
			t.Fatalf("Go read Java-created membership: %v", err)
		}
		if membership.Role != "MEMBER" {
			t.Fatalf("Java-created membership role = %q, want MEMBER", membership.Role)
		}
		roomID := "go-roundtrip-room"
		messages, err := chat.FetchMessages(ctx, &roomID, 0, 20)
		if err != nil {
			t.Fatalf("Go read Java-created chat: %v", err)
		}
		foundJavaChat := false
		for _, message := range messages {
			foundJavaChat = foundJavaChat || message.ID == "java-roundtrip-chat"
		}
		if !foundJavaChat {
			t.Fatalf("Java-created chat not found: %+v", messages)
		}
		setting, err := settings.FindValue(ctx, "java.roundtrip")
		if err != nil || setting == nil || *setting != "ok" {
			t.Fatalf("Go read Java-created setting = %v, %v", setting, err)
		}
		uploadUsers, err := local.FindAllowedUploadUsers(ctx)
		if err != nil {
			t.Fatalf("Go read Java-created upload grant: %v", err)
		}
		if _, found := uploadUsers["java-roundtrip-uploader"]; !found {
			t.Fatalf("Java-created upload grant not found: %v", uploadUsers)
		}
		return
	case "", "seed-go":
	default:
		t.Fatalf("unknown MUSICPARTY_JAVA_SQLITE_PHASE %q", os.Getenv("MUSICPARTY_JAVA_SQLITE_PHASE"))
	}

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
		t.Fatalf("Check(Java fixture after Go writes): %v", err)
	}
	if !result.OK() || result.ApplicationTables != 23 {
		t.Fatalf("Java fixture after Go writes is unhealthy: %+v", result)
	}
}
