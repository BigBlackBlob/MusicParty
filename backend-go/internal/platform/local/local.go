package local

import (
	"context"
	"fmt"
	"net/url"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform"
	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
)

type Service struct {
	repository *storesqlite.LocalTrackRepository
}

func New(repository *storesqlite.LocalTrackRepository) *Service {
	return &Service{repository: repository}
}
func (*Service) Name() string      { return "local" }
func (s *Service) Available() bool { return s.repository != nil }
func (s *Service) Search(ctx context.Context, keyword string, offset, limit int) ([]platform.Music, error) {
	tracks, err := s.repository.SearchCompleted(ctx, keyword, offset, limit)
	result := make([]platform.Music, 0, len(tracks))
	for _, track := range tracks {
		cover := ""
		if track.CoverPath != nil {
			cover = "/api/local/cover/" + track.ID
		}
		result = append(result, platform.Music{ID: track.ID, Name: track.Title, Artists: track.Artists, Duration: track.DurationMS, Platform: "local", CoverURL: cover})
	}
	return result, err
}
func (s *Service) ResolvePlayable(ctx context.Context, id string) (platform.PlayableMusic, error) {
	track, err := s.repository.FindByID(ctx, id)
	if err != nil {
		return platform.PlayableMusic{}, err
	}
	if track.Status != "COMPLETED" {
		return platform.PlayableMusic{}, fmt.Errorf("local track %s is not ready", id)
	}
	cover := ""
	if track.CoverPath != nil {
		cover = "/api/local/cover/" + url.PathEscape(track.ID)
	}
	music := platform.Music{ID: track.ID, Name: track.Title, Artists: track.Artists, Duration: track.DurationMS, Platform: "local", CoverURL: cover}
	return platform.PlayableMusic{Music: music, URL: "/api/local/media/" + url.PathEscape(track.ID)}, nil
}
func (*Service) UserPlaylists(context.Context, string) ([]platform.Playlist, error) {
	return []platform.Playlist{}, nil
}
func (*Service) PlaylistSongs(context.Context, string, int, int) ([]platform.Music, error) {
	return []platform.Music{}, nil
}
func (*Service) SearchAlbums(context.Context, string) ([]platform.Album, error) {
	return []platform.Album{}, nil
}
func (*Service) AlbumSongs(context.Context, string) ([]platform.Music, error) {
	return []platform.Music{}, nil
}
func (*Service) SearchUsers(context.Context, string) ([]platform.User, error) {
	return []platform.User{}, nil
}
func (*Service) Lyric(context.Context, string) (platform.Lyric, error) { return platform.Lyric{}, nil }
