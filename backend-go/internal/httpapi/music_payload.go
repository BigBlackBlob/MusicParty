package httpapi

import (
	"strings"

	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
)

// Validate every item before either transport starts a batch write.
func validPlaylistMusics(musics []storesqlite.Music) bool {
	if len(musics) == 0 || len(musics) > playlistMaxItems {
		return false
	}
	for _, music := range musics {
		if !validPlaylistMusic(music) {
			return false
		}
	}
	return true
}

func validPlaylistMusic(music storesqlite.Music) bool {
	bounded := func(value string, min, max int) bool {
		length := len([]rune(value))
		return min <= length && length <= max
	}
	if !bounded(music.ID, 1, 128) || !bounded(music.Platform, 1, 64) || !bounded(music.Name, 1, 256) || !bounded(music.CoverURL, 0, 512) {
		return false
	}
	if music.Duration < 0 || music.Duration > 86_400_000 || len(music.Artists) > 8 {
		return false
	}
	for _, artist := range music.Artists {
		if !bounded(artist, 0, 128) {
			return false
		}
	}
	return strings.TrimSpace(music.ID) != "" && strings.TrimSpace(music.Name) != "" && strings.TrimSpace(music.Platform) != ""
}
