package soundcloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform"
)

const apiBaseURL = "https://api.soundcloud.com"

type Service struct {
	client      *platform.Client
	enabled     bool
	clientID    string
	mu          sync.RWMutex
	accessToken string
}

// New uses an externally provisioned token, so the client secret is not retained.
func New(client *platform.Client, enabled bool, clientID, _ string) *Service {
	return &Service{client: client, enabled: enabled, clientID: clientID}
}

func (*Service) Name() string { return "soundcloud" }

func (s *Service) Available() bool { return s.enabled && s.clientID != "" }

func (s *Service) UpdateCredential(value string) {
	s.mu.Lock()
	s.accessToken = value
	s.mu.Unlock()
}

func (s *Service) accessTokenValue() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.accessToken
}

func (s *Service) apiURL(path string, values url.Values, token string) string {
	values.Set("client_id", s.clientID)
	if token != "" {
		values.Set("oauth_token", token)
	}
	return apiBaseURL + path + "?" + values.Encode()
}

func (s *Service) getJSON(ctx context.Context, path string, query url.Values, target any) error {
	token := s.accessTokenValue()
	headers := map[string]string{}
	if token != "" {
		headers["Authorization"] = "OAuth " + token
	}
	return withoutRequestURL(s.client.JSON(ctx, s.apiURL(path, query, token), headers, target))
}

// Transport errors include the request URL, which can contain the OAuth token.
func withoutRequestURL(err error) error {
	var requestError *url.Error
	if errors.As(err, &requestError) {
		return &platform.UpstreamError{Cause: errors.New("soundcloud request failed")}
	}
	return err
}

type track struct {
	ID         int64  `json:"id"`
	URN        string `json:"urn"`
	Title      string `json:"title"`
	Duration   int64  `json:"duration"`
	ArtworkURL string `json:"artwork_url"`
	Streamable bool   `json:"streamable"`
	User       struct {
		Username string `json:"username"`
	} `json:"user"`
}

func (t track) music() platform.Music {
	id := t.URN
	if t.ID != 0 {
		id = strconv.FormatInt(t.ID, 10)
	}
	return platform.Music{
		ID: id, Name: t.Title,
		Artists: []string{t.User.Username}, Duration: t.Duration,
		Platform: "soundcloud", CoverURL: t.ArtworkURL,
	}
}

type searchResponse struct {
	Collection []track `json:"collection"`
}

func (r *searchResponse) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '[' {
		return json.Unmarshal(data, &r.Collection)
	}
	type response searchResponse
	return json.Unmarshal(data, (*response)(r))
}

func (s *Service) Search(ctx context.Context, keyword string, offset, limit int) ([]platform.Music, error) {
	if err := platform.ValidateSearch(keyword, offset, limit); err != nil {
		return nil, err
	}
	query := url.Values{
		"q": {keyword}, "limit": {strconv.Itoa(limit)}, "offset": {strconv.Itoa(offset)},
		"linked_partitioning": {"1"},
	}
	var response searchResponse
	if err := s.getJSON(ctx, "/tracks", query, &response); err != nil {
		return nil, err
	}
	result := make([]platform.Music, 0, len(response.Collection))
	for _, item := range response.Collection {
		if item.Streamable {
			result = append(result, item.music())
		}
	}
	return result, nil
}

func (s *Service) trackInfo(ctx context.Context, id string) (platform.Music, error) {
	if strings.TrimSpace(id) == "" {
		return platform.Music{}, errors.New("soundcloud track ID is required")
	}
	var item track
	if err := s.getJSON(ctx, "/tracks/"+url.PathEscape(id), url.Values{}, &item); err != nil {
		return platform.Music{}, err
	}
	return item.music(), nil
}

func (s *Service) ResolvePlayable(ctx context.Context, id string) (platform.PlayableMusic, error) {
	music, err := s.trackInfo(ctx, id)
	if err != nil {
		return platform.PlayableMusic{}, err
	}
	return platform.PlayableMusic{Music: music, PlayerType: "soundcloud-widget", EmbedID: id}, nil
}

func (s *Service) StreamURL(ctx context.Context, id string) (string, error) {
	if strings.TrimSpace(id) == "" {
		return "", errors.New("soundcloud track ID is required")
	}
	token := s.accessTokenValue()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.apiURL("/tracks/"+url.PathEscape(id)+"/stream", url.Values{}, token), nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("User-Agent", s.client.UserAgent)
	if token != "" {
		request.Header.Set("Authorization", "OAuth "+token)
	}
	client := *s.client.HTTP
	client.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		next.Header.Del("Referer")
		if next.URL.Host != request.URL.Host {
			next.Header.Del("Authorization")
		}
		if s.client.HTTP.CheckRedirect != nil {
			return s.client.HTTP.CheckRedirect(next, via)
		}
		if len(via) >= 10 {
			return errors.New("too many SoundCloud stream redirects")
		}
		return nil
	}
	response, err := client.Do(request)
	if err != nil {
		return "", withoutRequestURL(err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("soundcloud stream returned HTTP %d", response.StatusCode)
	}
	if response.Request == nil || response.Request.URL == nil || response.Request.URL.String() == request.URL.String() || response.Request.URL.Query().Has("oauth_token") {
		return "", errors.New("soundcloud stream did not resolve to a URL")
	}
	return response.Request.URL.String(), nil
}

func (s *Service) CoverURL(ctx context.Context, id string) (string, error) {
	music, err := s.trackInfo(ctx, id)
	if err != nil {
		return "", err
	}
	return strings.ReplaceAll(music.CoverURL, "-large", "-t500x500"), nil
}

// StreamCacheable keeps short-lived signed URLs out of the media cache.
func (*Service) StreamCacheable() bool { return false }

func (*Service) UserPlaylists(context.Context, string) ([]platform.Playlist, error) {
	return []platform.Playlist{}, nil
}
func (*Service) PlaylistSongs(context.Context, string, int, int) ([]platform.Music, error) {
	return []platform.Music{}, nil
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

var _ platform.Service = (*Service)(nil)
var _ platform.PlayableResolver = (*Service)(nil)
var _ platform.MediaSource = (*Service)(nil)
var _ platform.CredentialUpdater = (*Service)(nil)
var _ platform.StreamCacheable = (*Service)(nil)
