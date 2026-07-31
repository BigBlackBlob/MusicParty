package sqlite

import (
	"context"
	"database/sql"
	"fmt"
)

type PlaybackStateRepository struct{ store *Store }

func NewPlaybackStateRepository(store *Store) *PlaybackStateRepository {
	return &PlaybackStateRepository{store: store}
}

func (repository *PlaybackStateRepository) FindByRoomID(ctx context.Context, roomID string) (*PlaybackState, error) {
	row := repository.store.reader.QueryRowContext(ctx, `
		select room_id, current_music_json, current_enqueuer_id, current_enqueuer_name,
		       position_anchor, timestamp_anchor, position_updated_at,
		       is_shuffle, is_paused, is_pause_locked, is_skip_locked, is_shuffle_locked,
		       is_loading, liked_user_ids_json, like_markers_json, play_epoch, state_version, last_persisted_at
		from room_playback_state where room_id = ?
	`, roomID)
	state, err := scanPlaybackState(row)
	if err != nil {
		return nil, fmt.Errorf("find playback state: %w", err)
	}
	return &state, nil
}

func (repository *PlaybackStateRepository) Upsert(ctx context.Context, state PlaybackState) error {
	values, err := encodePlaybackState(state)
	if err != nil {
		return err
	}
	return repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		return upsertPlaybackStateTx(ctx, tx, state, values)
	})
}

type encodedPlaybackState struct {
	currentMusic    any
	likedUserIDs    string
	likeMarkersJSON string
}

func encodePlaybackState(state PlaybackState) (encodedPlaybackState, error) {
	var values encodedPlaybackState
	var currentMusic any
	if state.CurrentMusic != nil {
		encoded, err := marshalJavaJSON(state.CurrentMusic)
		if err != nil {
			return values, fmt.Errorf("encode current music: %w", err)
		}
		currentMusic = encoded
	}
	likedUserIDs, err := marshalJavaJSON(stringSetForJSON(state.LikedUserIDs))
	if err != nil {
		return values, fmt.Errorf("encode liked user ids: %w", err)
	}
	likeMarkers := state.LikeMarkers
	if likeMarkers == nil {
		likeMarkers = []int64{}
	}
	likeMarkersJSON, err := marshalJavaJSON(likeMarkers)
	if err != nil {
		return values, fmt.Errorf("encode like markers: %w", err)
	}
	values.currentMusic = currentMusic
	values.likedUserIDs = likedUserIDs
	values.likeMarkersJSON = likeMarkersJSON
	return values, nil
}

func upsertPlaybackStateTx(ctx context.Context, tx *sql.Tx, state PlaybackState, values encodedPlaybackState) error {
	_, err := tx.ExecContext(ctx, `
			insert into room_playback_state(
				room_id, current_music_json, current_enqueuer_id, current_enqueuer_name,
				position_anchor, timestamp_anchor, position_updated_at,
				is_shuffle, is_paused, is_pause_locked, is_skip_locked, is_shuffle_locked,
				is_loading, liked_user_ids_json, like_markers_json, play_epoch, state_version, last_persisted_at
			) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			on conflict(room_id) do update set
				current_music_json = excluded.current_music_json,
				current_enqueuer_id = excluded.current_enqueuer_id,
				current_enqueuer_name = excluded.current_enqueuer_name,
				position_anchor = excluded.position_anchor,
				timestamp_anchor = excluded.timestamp_anchor,
				position_updated_at = excluded.position_updated_at,
				is_shuffle = excluded.is_shuffle,
				is_paused = excluded.is_paused,
				is_pause_locked = excluded.is_pause_locked,
				is_skip_locked = excluded.is_skip_locked,
				is_shuffle_locked = excluded.is_shuffle_locked,
				is_loading = excluded.is_loading,
				liked_user_ids_json = excluded.liked_user_ids_json,
				like_markers_json = excluded.like_markers_json,
				play_epoch = excluded.play_epoch,
				state_version = excluded.state_version,
				last_persisted_at = excluded.last_persisted_at
		`, state.RoomID, values.currentMusic, nullableString(state.CurrentEnqueuerID), nullableString(state.CurrentEnqueuerName),
		state.PositionAnchor, state.TimestampAnchor, state.PositionUpdatedAt, state.Shuffle, state.Paused,
		state.PauseLocked, state.SkipLocked, state.ShuffleLocked, state.Loading, values.likedUserIDs, values.likeMarkersJSON,
		state.PlayEpoch, state.StateVersion, state.LastPersistedAt)
	return err
}

func (repository *PlaybackStateRepository) Delete(ctx context.Context, roomID string) error {
	return repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "delete from room_playback_state where room_id = ?", roomID)
		return err
	})
}

func scanPlaybackState(row rowScanner) (PlaybackState, error) {
	var state PlaybackState
	var currentMusic, currentEnqueuerID, currentEnqueuerName, likedUserIDs, likeMarkers sql.NullString
	if err := row.Scan(&state.RoomID, &currentMusic, &currentEnqueuerID, &currentEnqueuerName,
		&state.PositionAnchor, &state.TimestampAnchor, &state.PositionUpdatedAt,
		&state.Shuffle, &state.Paused, &state.PauseLocked, &state.SkipLocked, &state.ShuffleLocked,
		&state.Loading, &likedUserIDs, &likeMarkers, &state.PlayEpoch, &state.StateVersion, &state.LastPersistedAt); err != nil {
		return PlaybackState{}, err
	}
	if currentMusic.Valid && currentMusic.String != "" {
		var music PlayableMusic
		if err := unmarshalJSON(currentMusic.String, &music); err != nil {
			return PlaybackState{}, err
		}
		state.CurrentMusic = &music
	}
	if currentEnqueuerID.Valid {
		state.CurrentEnqueuerID = &currentEnqueuerID.String
	}
	if currentEnqueuerName.Valid {
		state.CurrentEnqueuerName = &currentEnqueuerName.String
	}
	state.LikedUserIDs = map[string]struct{}{}
	if likedUserIDs.Valid && likedUserIDs.String != "" {
		var values []string
		if err := unmarshalJSON(likedUserIDs.String, &values); err != nil {
			return PlaybackState{}, err
		}
		for _, value := range values {
			state.LikedUserIDs[value] = struct{}{}
		}
	}
	state.LikeMarkers = []int64{}
	if likeMarkers.Valid && likeMarkers.String != "" {
		if err := unmarshalJSON(likeMarkers.String, &state.LikeMarkers); err != nil {
			return PlaybackState{}, err
		}
	}
	return state, nil
}

type ChatRepository struct{ store *Store }

func NewChatRepository(store *Store) *ChatRepository { return &ChatRepository{store: store} }

func (repository *ChatRepository) AppendMessage(ctx context.Context, roomID *string, message ChatMessage) error {
	return repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		return appendChatMessage(ctx, tx, roomID, message)
	})
}

func (repository *ChatRepository) FetchMessages(ctx context.Context, roomID *string, offset, limit int) ([]ChatMessage, error) {
	var room any
	if roomID != nil {
		room = *roomID
	}
	rows, err := repository.store.reader.QueryContext(ctx, `
		select id, user_id, user_name, content, type, created_at
		from chat_message
		where ((? is null and room_id is null) or room_id = ?)
		order by created_at desc limit ? offset ?
	`, room, room, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("fetch chat messages: %w", err)
	}
	defer rows.Close()
	messages := []ChatMessage{}
	for rows.Next() {
		var message ChatMessage
		if err := rows.Scan(&message.ID, &message.UserID, &message.UserName, &message.Content, &message.Type, &message.Timestamp); err != nil {
			return nil, fmt.Errorf("scan chat message: %w", err)
		}
		messages = append(messages, message)
	}
	return messages, rows.Err()
}

func (repository *ChatRepository) ReplaceMessages(ctx context.Context, roomID *string, messages []ChatMessage) error {
	return repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		var err error
		if roomID == nil {
			_, err = tx.ExecContext(ctx, "delete from chat_message where room_id is null")
		} else {
			_, err = tx.ExecContext(ctx, "delete from chat_message where room_id = ?", *roomID)
		}
		if err != nil {
			return err
		}
		for _, message := range messages {
			if err := appendChatMessage(ctx, tx, roomID, message); err != nil {
				return err
			}
		}
		return nil
	})
}

func (repository *ChatRepository) DeleteRoomHistory(ctx context.Context, roomID string) error {
	return repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "delete from chat_message where room_id = ?", roomID)
		return err
	})
}

func appendChatMessage(ctx context.Context, tx *sql.Tx, roomID *string, message ChatMessage) error {
	var room any
	if roomID != nil {
		room = *roomID
	}
	_, err := tx.ExecContext(ctx, `
		insert into chat_message(id, room_id, user_id, user_name, content, type, created_at)
		values (?, ?, ?, ?, ?, ?, ?)
	`, message.ID, room, message.UserID, message.UserName, message.Content, message.Type, message.Timestamp)
	return err
}
