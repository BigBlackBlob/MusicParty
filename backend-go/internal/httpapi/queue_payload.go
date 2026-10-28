package httpapi

import (
	"encoding/json"
	"strings"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/realtime"
)

func queueMutationID(session account.Session, id string) string {
	encoded, _ := json.Marshal([2]string{session.PublicID, id})
	return string(encoded)
}

func isQueueCommand(kind string) bool {
	switch kind {
	case "enqueue", "enqueue.playlist", "enqueue.album", "enqueue.room-playlist", "queue.remove", "queue.top", "queue.batch-remove", "queue.batch-top", "queue.reorder", "queue.clear":
		return true
	}
	return false
}

// Reject malformed mutations before provider requests or runtime writes.
func validateQueuePayload(kind string, raw json.RawMessage) error {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return realtime.ErrMutationInvalid
	}
	var id string
	if json.Unmarshal(fields["mutationId"], &id) != nil || strings.TrimSpace(id) == "" || len(id) > 128 {
		return realtime.ErrMutationInvalid
	}
	required := []string{}
	switch kind {
	case "enqueue":
		required = []string{"platform", "musicId"}
	case "enqueue.playlist":
		required = []string{"platform", "playlistId"}
	case "enqueue.album":
		required = []string{"platform", "albumId"}
	case "enqueue.room-playlist":
		required = []string{"playlistId"}
	case "queue.remove", "queue.top":
		required = []string{"queueId"}
	case "queue.batch-remove", "queue.batch-top":
		var ids []string
		if json.Unmarshal(fields["queueIds"], &ids) != nil || len(ids) == 0 || len(ids) > 200 {
			return realtime.ErrMutationInvalid
		}
		for _, id := range ids {
			if strings.TrimSpace(id) == "" {
				return realtime.ErrMutationInvalid
			}
		}
	case "queue.reorder":
		if fields["queueId"] != nil || fields["targetQueueId"] != nil {
			required = []string{"queueId", "targetQueueId"}
			var placement string
			if json.Unmarshal(fields["position"], &placement) != nil || (placement != "before" && placement != "after") {
				return realtime.ErrMutationInvalid
			}
		} else {
			for _, key := range []string{"oldIndex", "newIndex"} {
				var index *int
				if json.Unmarshal(fields[key], &index) != nil || index == nil || *index < 0 {
					return realtime.ErrMutationInvalid
				}
			}
		}
	}
	for _, key := range required {
		var value string
		if json.Unmarshal(fields[key], &value) != nil || strings.TrimSpace(value) == "" {
			return realtime.ErrMutationInvalid
		}
	}
	return nil
}
