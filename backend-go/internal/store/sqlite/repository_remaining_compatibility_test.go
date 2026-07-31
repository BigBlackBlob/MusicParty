package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestQueueRepositorySparseOrderingHistoryAndJavaJSON(t *testing.T) {
	store := openContractStore(t)
	defer store.Close()
	ctx := context.Background()
	seedRoomAndUsers(t, store, "queue-room", "u-queue")
	repository := NewQueueRepository(store, func() time.Time { return time.UnixMilli(5000) })
	items := []QueueItem{
		queueFixture("a", "歌 <A>", "PENDING"),
		queueFixture("b", "歌 B", "READY"),
		queueFixture("c", "歌 C", "READY"),
	}
	if err := repository.ReplaceQueue(ctx, "queue-room", items); err != nil {
		t.Fatalf("ReplaceQueue(): %v", err)
	}
	assertQueueOrders(t, store, "queue-room", map[string]int64{"a": 1024, "b": 2048, "c": 3072})
	var rawJSON string
	if err := store.Reader().QueryRowContext(ctx, `select music_json from room_queue where id = 'a'`).Scan(&rawJSON); err != nil {
		t.Fatalf("read queue JSON: %v", err)
	}
	wantJSON := `{"queueId":"a","music":{"id":"music-a","name":"歌 <A>","artists":["艺术家"],"duration":1234,"platform":"local","coverUrl":"封面"},"enqueuedBy":{"publicId":"u-queue","name":"用户","isGuest":false},"status":"PENDING"}`
	if rawJSON != wantJSON {
		t.Fatalf("persisted Java-compatible JSON mismatch\ngot:  %s\nwant: %s", rawJSON, wantJSON)
	}

	inserted := queueFixture("inserted", "插入", "READY")
	desired := []QueueItem{items[0], inserted, items[1], items[2]}
	if err := repository.SynchronizeQueue(ctx, "queue-room", desired); err != nil {
		t.Fatalf("SynchronizeQueue(insert): %v", err)
	}
	assertQueueOrders(t, store, "queue-room", map[string]int64{"a": 1024, "inserted": 1536, "b": 2048, "c": 3072})
	desired[3].Status = "FAILED"
	if err := repository.SynchronizeQueue(ctx, "queue-room", desired); err != nil {
		t.Fatalf("SynchronizeQueue(status): %v", err)
	}
	loaded, err := repository.LoadQueue(ctx, "queue-room")
	if err != nil {
		t.Fatalf("LoadQueue(): %v", err)
	}
	if diff := cmp.Diff(desired, loaded); diff != "" {
		t.Fatalf("queue mismatch (-want +got):\n%s", diff)
	}

	oldMusic := Music{ID: "history", Name: "Old", Artists: []string{"A"}, Duration: 1, Platform: "local", CoverURL: "old"}
	newMusic := oldMusic
	newMusic.Name, newMusic.CoverURL = "New", "new"
	for _, entry := range []HistoryEntry{
		{ID: "h1", RoomID: "queue-room", Music: oldMusic, PlayedAt: 100},
		{ID: "h2", RoomID: "queue-room", Music: newMusic, PlayedAt: 300},
		{ID: "h3", RoomID: "queue-room", Music: oldMusic, PlayedAt: 200},
	} {
		if err := repository.AppendHistory(ctx, entry); err != nil {
			t.Fatalf("AppendHistory(%s): %v", entry.ID, err)
		}
	}
	count, err := repository.CountHistoryTracks(ctx, "queue-room")
	if err != nil || count != 1 {
		t.Fatalf("CountHistoryTracks() = %d, %v; want 1, nil", count, err)
	}
	historyTracks, err := repository.ListHistoryTracks(ctx, "queue-room", -10, 0)
	if err != nil {
		t.Fatalf("ListHistoryTracks(): %v", err)
	}
	if len(historyTracks) != 1 || historyTracks[0].Music.Name != "New" || historyTracks[0].CreatedAt != 300 || historyTracks[0].SortOrder != 0 {
		t.Fatalf("aggregated history track = %+v", historyTracks)
	}
	var playCount int
	var firstPlayed, lastPlayed int64
	if err := store.Reader().QueryRowContext(ctx, `
		select play_count, first_played_at, last_played_at from room_history_track
		where room_id = 'queue-room' and platform = 'local' and music_id = 'history'
	`).Scan(&playCount, &firstPlayed, &lastPlayed); err != nil {
		t.Fatalf("read history aggregate: %v", err)
	}
	if playCount != 3 || firstPlayed != 100 || lastPlayed != 300 {
		t.Fatalf("history aggregate = count %d, first %d, last %d", playCount, firstPlayed, lastPlayed)
	}
}

func TestPlaybackAndChatRepositoriesPreserveNullsCollectionsAndTransactions(t *testing.T) {
	store := openContractStore(t)
	defer store.Close()
	ctx := context.Background()
	seedRoomAndUsers(t, store, "state-room", "u-state")
	playback := NewPlaybackStateRepository(store)
	enqueuerID, enqueuerName := "u-state", "用户"
	state := PlaybackState{
		RoomID: "state-room", CurrentMusic: &PlayableMusic{ID: "m", Name: "歌", Artists: []string{"甲"}, Duration: 99, Platform: "local", URL: "/m", CoverURL: "/c"},
		CurrentEnqueuerID: &enqueuerID, CurrentEnqueuerName: &enqueuerName, PositionAnchor: 10, TimestampAnchor: 20,
		PositionUpdatedAt: 30, Shuffle: true, Paused: true, PauseLocked: true, Loading: true,
		LikedUserIDs: map[string]struct{}{"z": {}, "a": {}}, LikeMarkers: []int64{1, 2}, PlayEpoch: 4, StateVersion: 5, LastPersistedAt: 6,
	}
	if err := playback.Upsert(ctx, state); err != nil {
		t.Fatalf("playback Upsert(): %v", err)
	}
	gotState, err := playback.FindByRoomID(ctx, "state-room")
	if err != nil {
		t.Fatalf("playback FindByRoomID(): %v", err)
	}
	if diff := cmp.Diff(&state, gotState); diff != "" {
		t.Fatalf("playback state mismatch (-want +got):\n%s", diff)
	}
	if err := store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `update room_playback_state set liked_user_ids_json = null, like_markers_json = null where room_id = 'state-room'`)
		return err
	}); err != nil {
		t.Fatalf("set legacy null collections: %v", err)
	}
	gotState, err = playback.FindByRoomID(ctx, "state-room")
	if err != nil || len(gotState.LikedUserIDs) != 0 || len(gotState.LikeMarkers) != 0 {
		t.Fatalf("legacy null playback collections = %+v, %v", gotState, err)
	}

	chat := NewChatRepository(store)
	roomID := "state-room"
	old := ChatMessage{ID: "old", UserID: "u-state", UserName: "用户", Content: "旧", Timestamp: 1, Type: "CHAT"}
	if err := chat.AppendMessage(ctx, &roomID, old); err != nil {
		t.Fatalf("AppendMessage(room): %v", err)
	}
	global := ChatMessage{ID: "global", UserID: "system", UserName: "System", Content: "全局", Timestamp: 2, Type: "SYSTEM"}
	if err := chat.AppendMessage(ctx, nil, global); err != nil {
		t.Fatalf("AppendMessage(global): %v", err)
	}
	roomMessages, err := chat.FetchMessages(ctx, &roomID, 0, 10)
	if err != nil || len(roomMessages) != 1 || roomMessages[0].ID != "old" {
		t.Fatalf("room messages = %+v, %v", roomMessages, err)
	}
	globalMessages, err := chat.FetchMessages(ctx, nil, 0, 10)
	if err != nil || len(globalMessages) != 1 || globalMessages[0].ID != "global" {
		t.Fatalf("global messages = %+v, %v", globalMessages, err)
	}
	duplicate := ChatMessage{ID: "duplicate", UserID: "u", UserName: "U", Content: "x", Timestamp: 3, Type: "CHAT"}
	err = chat.ReplaceMessages(ctx, &roomID, []ChatMessage{duplicate, duplicate})
	if err == nil {
		t.Fatal("ReplaceMessages with duplicate IDs unexpectedly succeeded")
	}
	roomMessages, err = chat.FetchMessages(ctx, &roomID, 0, 10)
	if err != nil || len(roomMessages) != 1 || roomMessages[0].ID != "old" {
		t.Fatalf("chat replacement rollback failed: %+v, %v", roomMessages, err)
	}
}

func TestPlaylistRepositoriesMatchOwnershipDedupAndReorderSemantics(t *testing.T) {
	store := openContractStore(t)
	defer store.Close()
	ctx := context.Background()
	seedRoomAndUsers(t, store, "playlist-room", "u-playlist")
	ids := idSequence("rp", "rt1", "rt2", "rt3", "up", "ut1", "ut2", "system")
	clock := func() time.Time { return time.UnixMilli(7000) }
	rooms := NewRoomPlaylistRepository(store, clock, ids)
	roomPlaylist, err := rooms.CreatePlaylist(ctx, "playlist-room", "房间列表")
	if err != nil {
		t.Fatalf("CreatePlaylist(room): %v", err)
	}
	musicA, musicB, musicC := musicFixture("a"), musicFixture("b"), musicFixture("c")
	trackA, _ := rooms.AddTrack(ctx, "playlist-room", roomPlaylist.ID, &musicA)
	trackB, _ := rooms.AddTrack(ctx, "playlist-room", roomPlaylist.ID, &musicB)
	trackC, _ := rooms.AddTrack(ctx, "playlist-room", roomPlaylist.ID, &musicC)
	if trackA == nil || trackB == nil || trackC == nil {
		t.Fatalf("room playlist tracks were not created: %+v %+v %+v", trackA, trackB, trackC)
	}
	if err := rooms.ReorderTracks(ctx, "playlist-room", roomPlaylist.ID, []string{trackC.ID, trackA.ID}); err != nil {
		t.Fatalf("ReorderTracks(room): %v", err)
	}
	roomTracks, err := rooms.ListTracks(ctx, "playlist-room", roomPlaylist.ID, 0, 500)
	if err != nil {
		t.Fatalf("ListTracks(room): %v", err)
	}
	if got := playlistMusicIDs(roomTracks); !cmp.Equal(got, []string{"c", "a", "b"}) {
		t.Fatalf("room track order mismatch (-got +want):\n%s", cmp.Diff(got, []string{"c", "a", "b"}))
	}
	if deleted, err := rooms.DeleteTrack(ctx, "wrong-room", roomPlaylist.ID, trackA.ID); err != nil || deleted {
		t.Fatalf("cross-room DeleteTrack() = %v, %v; want false, nil", deleted, err)
	}

	users := NewUserPlaylistRepository(store, clock, ids)
	userPlaylist, err := users.CreatePlaylist(ctx, "u-playlist", "用户列表")
	if err != nil {
		t.Fatalf("CreatePlaylist(user): %v", err)
	}
	userTrackA, err := users.AddTrackIfAbsent(ctx, "u-playlist", userPlaylist.ID, &musicA)
	if err != nil || userTrackA == nil {
		t.Fatalf("AddTrackIfAbsent(first) = %+v, %v", userTrackA, err)
	}
	duplicateTrack, err := users.AddTrackIfAbsent(ctx, "u-playlist", userPlaylist.ID, &musicA)
	if err != nil || duplicateTrack != nil {
		t.Fatalf("AddTrackIfAbsent(duplicate) = %+v, %v; want nil, nil", duplicateTrack, err)
	}
	userTrackB, err := users.AddTrackIfAbsent(ctx, "u-playlist", userPlaylist.ID, &musicB)
	if err != nil || userTrackB == nil {
		t.Fatalf("AddTrackIfAbsent(second) = %+v, %v", userTrackB, err)
	}
	systemPlaylist, err := users.CreateSystemPlaylist(ctx, "u-playlist", "收藏", "favorites")
	if err != nil {
		t.Fatalf("CreateSystemPlaylist(): %v", err)
	}
	if _, err := users.CreateSystemPlaylist(ctx, "u-playlist", "收藏更新", "favorites"); err != nil {
		t.Fatalf("CreateSystemPlaylist(upsert): %v", err)
	}
	playlists, err := users.ListPlaylists(ctx, "u-playlist")
	if err != nil || len(playlists) != 2 || playlists[0].ID != systemPlaylist.ID || playlists[0].Name != "收藏更新" {
		t.Fatalf("user playlist ordering/upsert = %+v, %v", playlists, err)
	}
	if deleted, err := users.DeleteTrackByMusicKey(ctx, "u-playlist", userPlaylist.ID, "local:a"); err != nil || !deleted {
		t.Fatalf("DeleteTrackByMusicKey() = %v, %v; want true, nil", deleted, err)
	}
}

func TestSettingsSubsonicAndLocalRepositories(t *testing.T) {
	store := openContractStore(t)
	defer store.Close()
	ctx := context.Background()
	seedRoomAndUsers(t, store, "media-room", "u-media")
	settings := NewSiteSettingRepository(store)
	secret := "秘密值"
	if err := settings.Upsert(ctx, SiteSetting{Key: "secret", Value: &secret, Secret: true, UpdatedAt: 10}); err != nil {
		t.Fatalf("site setting Upsert(): %v", err)
	}
	setting, err := settings.Find(ctx, "secret")
	if err != nil || setting.Value == nil || *setting.Value != secret || !setting.Secret {
		t.Fatalf("site setting = %+v, %v", setting, err)
	}

	subsonic := NewSubsonicSourceRepository(store)
	ownerRoom, allowed := "media-room", "u-media"
	sources := []SubsonicSource{
		{ID: "user-source", OwnerRoomID: &ownerRoom, Label: "Zeta", BaseURL: "https://example", Username: "u", Password: "p", Client: "musicparty", APIVersion: "1.16.1", AllowedUsers: &allowed, Enabled: true, CreatedAt: 1, UpdatedAt: 1},
		{ID: "system-source", Label: "Alpha", BaseURL: "https://system", Username: "u", Password: "p", Client: "musicparty", APIVersion: "1.16.1", Enabled: true, System: true, CreatedAt: 2, UpdatedAt: 2},
	}
	for _, source := range sources {
		if err := subsonic.Upsert(ctx, source); err != nil {
			t.Fatalf("Subsonic Upsert(%s): %v", source.ID, err)
		}
	}
	allSources, err := subsonic.FindAll(ctx)
	if err != nil || len(allSources) != 2 || allSources[0].ID != "system-source" {
		t.Fatalf("Subsonic source order = %+v, %v", allSources, err)
	}
	display := "房间音乐"
	if err := subsonic.UpsertRoomBinding(ctx, RoomSubsonicSource{RoomID: "media-room", SourceID: "user-source", Enabled: true, DisplayLabel: &display, SortOrder: 2, CreatedAt: 3, UpdatedAt: 3}); err != nil {
		t.Fatalf("UpsertRoomBinding(): %v", err)
	}
	binding, err := subsonic.FindRoomBinding(ctx, "media-room", "user-source")
	if err != nil || binding.DisplayLabel == nil || *binding.DisplayLabel != display {
		t.Fatalf("Subsonic binding = %+v, %v", binding, err)
	}
	if err := subsonic.Delete(ctx, "system-source"); err != nil {
		t.Fatalf("Delete(system source): %v", err)
	}
	if _, err := subsonic.FindByID(ctx, "system-source"); err != nil {
		t.Fatalf("system source should not be deleted: %v", err)
	}

	local := NewLocalTrackRepository(store)
	hash, sourcePath, uploadedBy := "hash", "source.flac", "u-media"
	track := LocalTrack{ID: "local-1", OriginalHash: &hash, SourcePath: &sourcePath, SourceSizeBytes: 10, Title: "夜曲 Unicode", Artists: []string{"甲", "乙"}, DurationMS: 123, Status: "PROCESSING", UploadedBy: &uploadedBy, CreatedAt: 10, UpdatedAt: 10}
	if err := local.Upsert(ctx, track); err != nil {
		t.Fatalf("local Upsert(): %v", err)
	}
	ogg := "track.ogg"
	progress := 100
	completed := int64(20)
	if err := local.UpdateStatus(ctx, track.ID, LocalTrackStatusUpdate{Status: "COMPLETED", OGGPath: &ogg, ProgressPercent: &progress, CompletedAt: &completed, UpdatedAt: 20}); err != nil {
		t.Fatalf("local UpdateStatus(): %v", err)
	}
	gotTrack, err := local.FindActiveByOriginalHash(ctx, hash)
	if err != nil || gotTrack.SourcePath != nil || gotTrack.OGGPath == nil || *gotTrack.OGGPath != ogg || !cmp.Equal(gotTrack.Artists, []string{"甲", "乙"}) {
		t.Fatalf("updated local track = %+v, %v", gotTrack, err)
	}
	search, err := local.SearchCompleted(ctx, "unicode", -1, 0)
	if err != nil || len(search) != 1 || search[0].ID != track.ID {
		t.Fatalf("SearchCompleted() = %+v, %v", search, err)
	}
	if err := local.GrantUploadUser(ctx, "alice", 1); err != nil {
		t.Fatalf("GrantUploadUser(): %v", err)
	}
	if err := local.GrantUploadUser(ctx, "alice", 2); err != nil {
		t.Fatalf("GrantUploadUser(update): %v", err)
	}
	users, err := local.FindAllowedUploadUsers(ctx)
	if err != nil {
		t.Fatalf("FindAllowedUploadUsers(): %v", err)
	}
	if _, ok := users["alice"]; !ok {
		t.Fatalf("allowed upload users = %v", users)
	}
	assertLocalReferenceCleanup(t, store, local, track.ID)
}

func assertLocalReferenceCleanup(t *testing.T, store *Store, local *LocalTrackRepository, trackID string) {
	t.Helper()
	ctx := context.Background()
	queue := NewQueueRepository(store, nil)
	item := queueFixture("local-ref", "Local", "READY")
	item.Music.ID = trackID
	if err := queue.ReplaceQueue(ctx, "media-room", []QueueItem{item}); err != nil {
		t.Fatalf("seed local queue reference: %v", err)
	}
	if err := queue.AppendHistory(ctx, HistoryEntry{ID: "local-history", RoomID: "media-room", Music: item.Music, PlayedAt: 1}); err != nil {
		t.Fatalf("seed local history reference: %v", err)
	}
	playbackMusic := PlayableMusic{ID: trackID, Name: "Local", Artists: []string{"A"}, Platform: "local"}
	if err := NewPlaybackStateRepository(store).Upsert(ctx, PlaybackState{RoomID: "media-room", CurrentMusic: &playbackMusic, LikedUserIDs: map[string]struct{}{}, LikeMarkers: []int64{}}); err != nil {
		t.Fatalf("seed playback reference: %v", err)
	}
	roomPlaylists := NewRoomPlaylistRepository(store, nil, idSequence("cleanup-rp", "cleanup-rt"))
	rp, err := roomPlaylists.CreatePlaylist(ctx, "media-room", "Cleanup")
	if err != nil {
		t.Fatalf("seed room playlist: %v", err)
	}
	if _, err := roomPlaylists.AddTrack(ctx, "media-room", rp.ID, &item.Music); err != nil {
		t.Fatalf("seed room playlist track: %v", err)
	}
	userPlaylists := NewUserPlaylistRepository(store, nil, idSequence("cleanup-up", "cleanup-ut"))
	up, err := userPlaylists.CreatePlaylist(ctx, "u-media", "Cleanup")
	if err != nil {
		t.Fatalf("seed user playlist: %v", err)
	}
	if _, err := userPlaylists.AddTrackIfAbsent(ctx, "u-media", up.ID, &item.Music); err != nil {
		t.Fatalf("seed user playlist track: %v", err)
	}
	if err := local.DeleteLocalReferences(ctx, trackID); err != nil {
		t.Fatalf("DeleteLocalReferences(): %v", err)
	}
	checks := map[string]string{
		"room_queue":          `select count(*) from room_queue where id = 'local-ref'`,
		"room_history":        `select count(*) from room_history where id = 'local-history'`,
		"room_history_track":  `select count(*) from room_history_track where platform = 'local' and music_id = 'local-1'`,
		"room_playlist_track": `select count(*) from room_playlist_track where playlist_id = 'cleanup-rp'`,
		"user_playlist_track": `select count(*) from user_playlist_track where playlist_id = 'cleanup-up'`,
	}
	for name, query := range checks {
		var count int
		if err := store.Reader().QueryRowContext(ctx, query).Scan(&count); err != nil || count != 0 {
			t.Fatalf("%s references after cleanup = %d, %v", name, count, err)
		}
	}
	var currentMusic sql.NullString
	if err := store.Reader().QueryRowContext(ctx, `select current_music_json from room_playback_state where room_id = 'media-room'`).Scan(&currentMusic); err != nil || currentMusic.Valid {
		t.Fatalf("playback reference after cleanup = %+v, %v", currentMusic, err)
	}
}

func seedRoomAndUsers(t *testing.T, store *Store, roomID, publicID string) {
	t.Helper()
	ctx := context.Background()
	profiles := NewUserProfileRepository(store, nil)
	if err := profiles.UpsertProfile(ctx, UserProfile{PublicID: publicID, DisplayName: publicID, CurrentRoomID: roomID, CreatedAt: 1, LastSeenAt: 1}); err != nil {
		t.Fatalf("seed profile: %v", err)
	}
	if err := NewRoomRepository(store).Upsert(ctx, Room{ID: roomID, Name: roomID, OwnerPublicID: publicID, Visibility: "PUBLIC", CreatedAt: 1, LastActiveAt: 1}); err != nil {
		t.Fatalf("seed room: %v", err)
	}
}

func queueFixture(id, name, status string) QueueItem {
	return QueueItem{QueueID: id, Music: Music{ID: "music-" + id, Name: name, Artists: []string{"艺术家"}, Duration: 1234, Platform: "local", CoverURL: "封面"}, EnqueuedBy: UserSummary{PublicID: "u-queue", Name: "用户"}, Status: status}
}

func musicFixture(id string) Music {
	return Music{ID: id, Name: "Music " + strings.ToUpper(id), Artists: []string{"A"}, Duration: 1, Platform: "local", CoverURL: "cover"}
}

func assertQueueOrders(t *testing.T, store *Store, roomID string, want map[string]int64) {
	t.Helper()
	rows, err := store.Reader().Query(`select id, sort_order from room_queue where room_id = ?`, roomID)
	if err != nil {
		t.Fatalf("read queue orders: %v", err)
	}
	defer rows.Close()
	got := map[string]int64{}
	for rows.Next() {
		var id string
		var order int64
		if err := rows.Scan(&id, &order); err != nil {
			t.Fatalf("scan queue order: %v", err)
		}
		got[id] = order
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("queue orders mismatch (-want +got):\n%s", diff)
	}
}

func playlistMusicIDs(tracks []PlaylistTrack) []string {
	ids := make([]string, len(tracks))
	for index := range tracks {
		ids[index] = tracks[index].Music.ID
	}
	return ids
}

func idSequence(values ...string) func() string {
	index := 0
	return func() string {
		if index >= len(values) {
			return "extra-id"
		}
		value := values[index]
		index++
		return value
	}
}

func TestRemainingRepositoriesReturnSQLNoRows(t *testing.T) {
	store := openContractStore(t)
	defer store.Close()
	ctx := context.Background()
	checks := []func() error{
		func() error { _, err := NewPlaybackStateRepository(store).FindByRoomID(ctx, "missing"); return err },
		func() error { _, err := NewSiteSettingRepository(store).Find(ctx, "missing"); return err },
		func() error { _, err := NewSubsonicSourceRepository(store).FindByID(ctx, "missing"); return err },
		func() error { _, err := NewLocalTrackRepository(store).FindByID(ctx, "missing"); return err },
	}
	for index, check := range checks {
		if err := check(); !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("missing repository check %d error = %v, want sql.ErrNoRows", index, err)
		}
	}
}
