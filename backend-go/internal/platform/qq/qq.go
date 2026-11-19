package qq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform"
)

const maxCachedTracks = 512

type Service struct {
	client     *platform.Client
	sidecarURL string
	enabled    bool
	mu         sync.RWMutex
	cookie     string
	cacheMu    sync.Mutex
	trackCache map[string]platform.Music
}

func New(client *platform.Client, enabled bool, sidecarURL string) *Service {
	return &Service{
		client: client, sidecarURL: strings.TrimRight(strings.TrimSpace(sidecarURL), "/"),
		enabled: enabled, trackCache: make(map[string]platform.Music),
	}
}

func (*Service) Name() string      { return "qq" }
func (s *Service) Available() bool { return s.enabled && s.sidecarURL != "" }

// StreamCacheable keeps short-lived signed URLs out of the media cache.
func (*Service) StreamCacheable() bool { return false }

func (s *Service) UpdateCredential(value string) {
	s.mu.Lock()
	s.cookie = value
	s.mu.Unlock()
}

func (s *Service) request(ctx context.Context, path string, params url.Values) (*http.Response, error) {
	if !s.Available() {
		return nil, errors.New("qq: sidecar is disabled or not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.sidecarURL+path+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", s.client.UserAgent)
	s.mu.RLock()
	req.Header.Set("X-QQ-Cookie", s.cookie)
	s.mu.RUnlock()

	// Keep the custom credential header on the configured sidecar origin.
	client := *s.client.HTTP
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return client.Do(req)
}

func (s *Service) decode(response *http.Response, target any) error {
	body, err := io.ReadAll(io.LimitReader(response.Body, s.client.MaxBody+1))
	if err != nil {
		return err
	}
	if int64(len(body)) > s.client.MaxBody {
		return errors.New("qq: sidecar response is too large")
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("qq: decode sidecar response: %w", err)
	}
	return nil
}

func (s *Service) Search(ctx context.Context, keyword string, offset, limit int) ([]platform.Music, error) {
	if err := platform.ValidateSearch(keyword, offset, limit); err != nil {
		return nil, err
	}
	response, err := s.request(ctx, "/search", url.Values{
		"keyword": {keyword}, "offset": {strconv.Itoa(offset)}, "limit": {strconv.Itoa(limit)},
	})
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, &platform.UpstreamError{Status: response.StatusCode}
	}

	tracks := []platform.Music{}
	if err := s.decode(response, &tracks); err != nil {
		return nil, err
	}
	return s.cacheTracks(tracks), nil
}

func (s *Service) cacheTracks(tracks []platform.Music) []platform.Music {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	results := tracks[:0]
	for _, track := range tracks {
		if track.ID == "" {
			continue
		}
		track.Platform = s.Name()
		results = append(results, track)
		if _, exists := s.trackCache[track.ID]; !exists && len(s.trackCache) >= maxCachedTracks {
			for oldID := range s.trackCache {
				delete(s.trackCache, oldID)
				break
			}
		}
		s.trackCache[track.ID] = track
	}
	return results
}

func (s *Service) cachedTrack(id string) (platform.Music, bool) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	track, ok := s.trackCache[id]
	return track, ok
}

func (s *Service) ResolvePlayable(ctx context.Context, id string) (platform.PlayableMusic, error) {
	if strings.TrimSpace(id) == "" {
		return platform.PlayableMusic{}, &platform.InputError{Message: "QQ Music track ID is required"}
	}
	music, ok := s.cachedTrack(id)
	if !ok {
		tracks, err := s.Search(ctx, id, 0, 10)
		if err != nil {
			return platform.PlayableMusic{}, err
		}
		// Keyword search may miss a MID. Keep a playable ID usable after a cache miss.
		music = platform.Music{ID: id, Name: id, Artists: []string{}, Platform: s.Name()}
		for _, track := range tracks {
			if track.ID == id {
				music = track
				break
			}
		}
	}
	if _, err := s.StreamURL(ctx, id); err != nil {
		return platform.PlayableMusic{}, err
	}
	return platform.PlayableMusic{Music: music, URL: "/api/qq/stream/" + url.PathEscape(id)}, nil
}

func (s *Service) StreamURL(ctx context.Context, id string) (string, error) {
	if strings.TrimSpace(id) == "" {
		return "", &platform.InputError{Message: "QQ Music track ID is required"}
	}
	response, err := s.request(ctx, "/resolve", url.Values{"id": {id}})
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	var result struct {
		URL   string `json:"url"`
		Error string `json:"error"`
	}
	switch response.StatusCode {
	case http.StatusUnprocessableEntity:
		if err := s.decode(response, &result); err == nil && result.Error == "no_playable_url" {
			return "", &platform.StreamResolutionError{Kind: platform.StreamUnavailable, Detail: "qq: no playable URL"}
		}
		return "", &platform.UpstreamError{Status: response.StatusCode}
	case http.StatusOK:
		if err := s.decode(response, &result); err != nil {
			return "", err
		}
	default:
		return "", &platform.UpstreamError{Status: response.StatusCode}
	}
	target, err := url.Parse(result.URL)
	if err != nil || target.Scheme != "https" || target.Hostname() == "" || target.User != nil {
		return "", errors.New("qq: sidecar did not return a valid HTTPS stream URL")
	}
	return result.URL, nil
}

func (s *Service) CoverURL(_ context.Context, id string) (string, error) {
	track, _ := s.cachedTrack(id)
	return track.CoverURL, nil
}

func (s *Service) UserPlaylists(ctx context.Context, uid string) ([]platform.Playlist, error) {
	if strings.TrimSpace(uid) == "" {
		return nil, &platform.InputError{Message: "QQ Music user ID is required"}
	}
	response, err := s.request(ctx, "/playlists", url.Values{"uid": {uid}})
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, &platform.UpstreamError{Status: response.StatusCode}
	}
	var values []struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		CoverURL   string `json:"coverUrl"`
		TrackCount int    `json:"trackCount"`
	}
	if err := s.decode(response, &values); err != nil {
		return nil, err
	}
	playlists := make([]platform.Playlist, 0, len(values))
	for _, value := range values {
		playlists = append(playlists, platform.Playlist{ID: value.ID, Name: value.Name, CoverImgURL: value.CoverURL, TrackCount: value.TrackCount, Platform: s.Name()})
	}
	return playlists, nil
}

func (s *Service) PlaylistSongs(ctx context.Context, id string, offset, limit int) ([]platform.Music, error) {
	if strings.TrimSpace(id) == "" {
		return nil, &platform.InputError{Message: "QQ Music playlist ID is required"}
	}
	if err := platform.ValidatePage(offset, limit, platform.MaxPlaylistLimit); err != nil {
		return nil, err
	}
	response, err := s.request(ctx, "/playlist/tracks", url.Values{
		"id": {id}, "offset": {strconv.Itoa(offset)}, "limit": {strconv.Itoa(limit)},
	})
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, &platform.UpstreamError{Status: response.StatusCode}
	}
	tracks := []platform.Music{}
	if err := s.decode(response, &tracks); err != nil {
		return nil, err
	}
	if len(tracks) > limit {
		return nil, errors.New("qq: sidecar returned more tracks than requested")
	}
	return s.cacheTracks(tracks), nil
}

func (*Service) SearchAlbums(context.Context, string, int, int) (platform.AlbumSearchResult, error) {
	return platform.AlbumSearchResult{Items: []platform.Album{}}, nil
}
func (*Service) AlbumSongs(context.Context, string) ([]platform.Music, error) {
	return []platform.Music{}, nil
}
func (*Service) SearchUsers(context.Context, string) ([]platform.User, error) {
	return []platform.User{}, nil
}
func (*Service) Lyric(context.Context, string) (platform.Lyric, error) {
	return platform.Lyric{}, nil
}

var (
	_ platform.Service           = (*Service)(nil)
	_ platform.PlayableResolver  = (*Service)(nil)
	_ platform.MediaSource       = (*Service)(nil)
	_ platform.CredentialUpdater = (*Service)(nil)
	_ platform.StreamCacheable   = (*Service)(nil)
)
