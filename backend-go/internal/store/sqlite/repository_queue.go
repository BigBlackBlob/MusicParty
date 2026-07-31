package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const queueSortOrderStep int64 = 1024

type QueueRepository struct {
	store *Store
	now   func() time.Time
}

type persistedQueueRow struct {
	ID                   string
	MusicJSON            string
	EnqueuerPublicID     string
	EnqueuerNameSnapshot string
	Status               string
	SortOrder            int64
}

func NewQueueRepository(store *Store, now func() time.Time) *QueueRepository {
	if now == nil {
		now = time.Now
	}
	return &QueueRepository{store: store, now: now}
}

func (repository *QueueRepository) LoadQueue(ctx context.Context, roomID string) ([]QueueItem, error) {
	rows, err := repository.store.reader.QueryContext(ctx, `
		select music_json from room_queue where room_id = ? order by sort_order asc
	`, roomID)
	if err != nil {
		return nil, fmt.Errorf("load room queue: %w", err)
	}
	defer rows.Close()
	items := []QueueItem{}
	for rows.Next() {
		var encoded string
		if err := rows.Scan(&encoded); err != nil {
			return nil, fmt.Errorf("scan room queue: %w", err)
		}
		var item QueueItem
		if err := unmarshalJSON(encoded, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (repository *QueueRepository) ReplaceQueue(ctx context.Context, roomID string, items []QueueItem) error {
	encoded, err := encodeQueueItems(items)
	if err != nil {
		return err
	}
	return repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "delete from room_queue where room_id = ?", roomID); err != nil {
			return err
		}
		for index, item := range items {
			if _, err := tx.ExecContext(ctx, `
				insert into room_queue(id, room_id, music_json, enqueuer_public_id, enqueuer_name_snapshot, status, sort_order, created_at)
				values (?, ?, ?, ?, ?, ?, ?, ?)
			`, item.QueueID, roomID, encoded[index], item.EnqueuedBy.PublicID, item.EnqueuedBy.Name, item.Status,
				int64(index+1)*queueSortOrderStep, repository.now().UnixMilli()); err != nil {
				return err
			}
		}
		return nil
	})
}

func (repository *QueueRepository) SynchronizeQueue(ctx context.Context, roomID string, items []QueueItem) error {
	encoded, err := encodeQueueItems(items)
	if err != nil {
		return err
	}
	return repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		return synchronizeQueueTx(ctx, tx, repository.now, roomID, items, encoded)
	})
}

func synchronizeQueueTx(ctx context.Context, tx *sql.Tx, now func() time.Time, roomID string, items []QueueItem, encoded []string) error {
	existing, err := loadPersistedQueueRows(ctx, tx, roomID)
	if err != nil {
		return err
	}
	desiredIDs := make(map[string]struct{}, len(items))
	for _, item := range items {
		desiredIDs[item.QueueID] = struct{}{}
	}
	for id := range existing {
		if _, keep := desiredIDs[id]; !keep {
			if _, err := tx.ExecContext(ctx, `delete from room_queue where room_id = ? and id = ?`, roomID, id); err != nil {
				return err
			}
		}
	}

	sortOrders := allocateSparseSortOrders(items, existing)
	for index, item := range items {
		persisted, found := existing[item.QueueID]
		if !found {
			if _, err := tx.ExecContext(ctx, `
					insert into room_queue(id, room_id, music_json, enqueuer_public_id, enqueuer_name_snapshot, status, sort_order, created_at)
					values (?, ?, ?, ?, ?, ?, ?, ?)
				`, item.QueueID, roomID, encoded[index], item.EnqueuedBy.PublicID, item.EnqueuedBy.Name, item.Status,
				sortOrders[item.QueueID], now().UnixMilli()); err != nil {
				return err
			}
			continue
		}
		if persisted.SortOrder == sortOrders[item.QueueID] && persisted.MusicJSON == encoded[index] &&
			persisted.EnqueuerPublicID == item.EnqueuedBy.PublicID && persisted.EnqueuerNameSnapshot == item.EnqueuedBy.Name &&
			persisted.Status == item.Status {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
				update room_queue set music_json = ?, enqueuer_public_id = ?, enqueuer_name_snapshot = ?, status = ?, sort_order = ?
				where room_id = ? and id = ?
			`, encoded[index], item.EnqueuedBy.PublicID, item.EnqueuedBy.Name, item.Status, sortOrders[item.QueueID], roomID, item.QueueID); err != nil {
			return err
		}
	}
	return nil
}

func (repository *QueueRepository) LoadHistory(ctx context.Context, roomID string, limit int) ([]HistoryEntry, error) {
	rows, err := repository.store.reader.QueryContext(ctx, `
		select id, room_id, music_json, enqueuer_public_id, played_at
		from room_history where room_id = ? order by played_at desc limit ?
	`, roomID, limit)
	if err != nil {
		return nil, fmt.Errorf("load room history: %w", err)
	}
	defer rows.Close()
	entries := []HistoryEntry{}
	for rows.Next() {
		entry, err := scanHistoryEntry(rows)
		if err != nil {
			return nil, fmt.Errorf("scan room history: %w", err)
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (repository *QueueRepository) CountHistoryTracks(ctx context.Context, roomID string) (int, error) {
	var count int
	err := repository.store.reader.QueryRowContext(ctx, `select count(1) from room_history_track where room_id = ?`, roomID).Scan(&count)
	return count, err
}

func (repository *QueueRepository) ListHistoryTracks(ctx context.Context, roomID string, offset, limit int) ([]PlaylistTrack, error) {
	safeOffset, safeLimit := max(0, offset), min(500, max(1, limit))
	rows, err := repository.store.reader.QueryContext(ctx, `
		select platform, music_id, music_json, last_played_at
		from room_history_track where room_id = ? order by last_played_at desc limit ? offset ?
	`, roomID, safeLimit, safeOffset)
	if err != nil {
		return nil, fmt.Errorf("list room history tracks: %w", err)
	}
	defer rows.Close()
	tracks := []PlaylistTrack{}
	for rows.Next() {
		var platform, musicID, musicJSON string
		var track PlaylistTrack
		if err := rows.Scan(&platform, &musicID, &musicJSON, &track.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan room history track: %w", err)
		}
		if err := unmarshalJSON(musicJSON, &track.Music); err != nil {
			return nil, err
		}
		track.ID = platform + ":" + musicID
		track.PlaylistID = "__room_history__"
		track.SortOrder = safeOffset + len(tracks)
		tracks = append(tracks, track)
	}
	return tracks, rows.Err()
}

func (repository *QueueRepository) ListHistoryMusic(ctx context.Context, roomID string, limit int) ([]Music, error) {
	tracks, err := repository.ListHistoryTracks(ctx, roomID, 0, limit)
	if err != nil {
		return nil, err
	}
	music := make([]Music, len(tracks))
	for index := range tracks {
		music[index] = tracks[index].Music
	}
	return music, nil
}

func (repository *QueueRepository) AppendHistory(ctx context.Context, entry HistoryEntry) error {
	musicJSON, err := marshalJavaJSON(entry.Music)
	if err != nil {
		return fmt.Errorf("encode room history music: %w", err)
	}
	return repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		return appendHistoryTx(ctx, tx, entry, musicJSON)
	})
}

func appendHistoryTx(ctx context.Context, tx *sql.Tx, entry HistoryEntry, musicJSON string) error {
	if _, err := tx.ExecContext(ctx, `
			insert into room_history(id, room_id, music_json, enqueuer_public_id, played_at) values (?, ?, ?, ?, ?)
		`, entry.ID, entry.RoomID, musicJSON, nullableString(entry.EnqueuerPublicID), entry.PlayedAt); err != nil {
		return err
	}
	if entry.Music.Platform == "" || entry.Music.ID == "" {
		return nil
	}
	_, err := tx.ExecContext(ctx, `
			insert into room_history_track(room_id, platform, music_id, music_json, play_count, first_played_at, last_played_at)
			values (?, ?, ?, ?, 1, ?, ?)
			on conflict(room_id, platform, music_id) do update set
			  music_json = case when excluded.last_played_at >= room_history_track.last_played_at then excluded.music_json else room_history_track.music_json end,
			  play_count = room_history_track.play_count + 1,
			  first_played_at = min(room_history_track.first_played_at, excluded.first_played_at),
			  last_played_at = max(room_history_track.last_played_at, excluded.last_played_at)
		`, entry.RoomID, entry.Music.Platform, entry.Music.ID, musicJSON, entry.PlayedAt, entry.PlayedAt)
	return err
}

func (repository *QueueRepository) ReplaceHistory(ctx context.Context, roomID string, history []Music) error {
	encoded := make([]string, len(history))
	for index := range history {
		value, err := marshalJavaJSON(history[index])
		if err != nil {
			return fmt.Errorf("encode replacement history: %w", err)
		}
		encoded[index] = value
	}
	return repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `delete from room_history where room_id = ?`, roomID); err != nil {
			return err
		}
		for index := range history {
			if _, err := tx.ExecContext(ctx, `
				insert into room_history(id, room_id, music_json, enqueuer_public_id, played_at) values (?, ?, ?, null, ?)
			`, fmt.Sprintf("%s-history-%d", roomID, index), roomID, encoded[index], repository.now().UnixMilli()-int64(index)); err != nil {
				return err
			}
		}
		return nil
	})
}

func (repository *QueueRepository) DeleteRoomData(ctx context.Context, roomID string) error {
	return repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		for _, table := range []string{"room_queue", "room_history", "room_history_track"} {
			if _, err := tx.ExecContext(ctx, "delete from "+table+" where room_id = ?", roomID); err != nil {
				return err
			}
		}
		return nil
	})
}

func encodeQueueItems(items []QueueItem) ([]string, error) {
	encoded := make([]string, len(items))
	for index := range items {
		value, err := marshalJavaJSON(items[index])
		if err != nil {
			return nil, fmt.Errorf("encode queue item %s: %w", items[index].QueueID, err)
		}
		encoded[index] = value
	}
	return encoded, nil
}

func loadPersistedQueueRows(ctx context.Context, tx *sql.Tx, roomID string) (map[string]persistedQueueRow, error) {
	rows, err := tx.QueryContext(ctx, `
		select id, music_json, enqueuer_public_id, enqueuer_name_snapshot, status, sort_order from room_queue where room_id = ?
	`, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]persistedQueueRow{}
	for rows.Next() {
		var row persistedQueueRow
		if err := rows.Scan(&row.ID, &row.MusicJSON, &row.EnqueuerPublicID, &row.EnqueuerNameSnapshot, &row.Status, &row.SortOrder); err != nil {
			return nil, err
		}
		result[row.ID] = row
	}
	return result, rows.Err()
}

func allocateSparseSortOrders(desired []QueueItem, existing map[string]persistedQueueRow) map[string]int64 {
	result := map[string]int64{}
	if len(desired) == 0 {
		return result
	}
	if !hasUsableSparseOrder(existing) {
		return denseSortOrders(desired)
	}
	stableIDs := longestIncreasingQueueSubsequence(desired, existing)
	stable := make(map[string]struct{}, len(stableIDs))
	for _, id := range stableIDs {
		stable[id] = struct{}{}
		result[id] = existing[id].SortOrder
	}

	for index := 0; index < len(desired); {
		if _, ok := stable[desired[index].QueueID]; ok {
			index++
			continue
		}
		start := index
		for index < len(desired) {
			if _, ok := stable[desired[index].QueueID]; ok {
				break
			}
			index++
		}
		end, count := index, index-start
		var left, right *int64
		if start > 0 {
			value := result[desired[start-1].QueueID]
			left = &value
		}
		if end < len(desired) {
			value := existing[desired[end].QueueID].SortOrder
			right = &value
		}
		if left != nil && right != nil && *right-*left <= int64(count) {
			return denseSortOrders(desired)
		}
		for offset := 0; offset < count; offset++ {
			var order int64
			switch {
			case left == nil && right != nil:
				order = *right - int64(count-offset)*queueSortOrderStep
			case right == nil && left != nil:
				order = *left + int64(offset+1)*queueSortOrderStep
			case left != nil && right != nil:
				order = *left + (*right-*left)*int64(offset+1)/int64(count+1)
			default:
				order = int64(offset+1) * queueSortOrderStep
			}
			result[desired[start+offset].QueueID] = order
		}
	}
	return result
}

func hasUsableSparseOrder(existing map[string]persistedQueueRow) bool {
	if len(existing) == 0 {
		return false
	}
	seen := make(map[int64]struct{}, len(existing))
	for _, row := range existing {
		if _, duplicate := seen[row.SortOrder]; duplicate {
			return false
		}
		seen[row.SortOrder] = struct{}{}
	}
	return true
}

func denseSortOrders(desired []QueueItem) map[string]int64 {
	result := make(map[string]int64, len(desired))
	for index, item := range desired {
		result[item.QueueID] = int64(index+1) * queueSortOrderStep
	}
	return result
}

func longestIncreasingQueueSubsequence(desired []QueueItem, existing map[string]persistedQueueRow) []string {
	ids := make([]string, 0, len(desired))
	for _, item := range desired {
		if _, ok := existing[item.QueueID]; ok {
			ids = append(ids, item.QueueID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	tails, previous := make([]int, len(ids)), make([]int, len(ids))
	for index := range previous {
		previous[index] = -1
	}
	length := 0
	for index, id := range ids {
		value := existing[id].SortOrder
		low, high := 0, length
		for low < high {
			middle := (low + high) >> 1
			if existing[ids[tails[middle]]].SortOrder < value {
				low = middle + 1
			} else {
				high = middle
			}
		}
		if low > 0 {
			previous[index] = tails[low-1]
		}
		tails[low] = index
		if low == length {
			length++
		}
	}
	result := make([]string, 0, length)
	for current := tails[length-1]; current >= 0; current = previous[current] {
		result = append(result, ids[current])
	}
	for left, right := 0, len(result)-1; left < right; left, right = left+1, right-1 {
		result[left], result[right] = result[right], result[left]
	}
	return result
}

func scanHistoryEntry(row rowScanner) (HistoryEntry, error) {
	var entry HistoryEntry
	var musicJSON string
	var enqueuer sql.NullString
	if err := row.Scan(&entry.ID, &entry.RoomID, &musicJSON, &enqueuer, &entry.PlayedAt); err != nil {
		return HistoryEntry{}, err
	}
	if err := unmarshalJSON(musicJSON, &entry.Music); err != nil {
		return HistoryEntry{}, err
	}
	if enqueuer.Valid {
		entry.EnqueuerPublicID = &enqueuer.String
	}
	return entry, nil
}
