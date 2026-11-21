package soundcloud

import (
	"context"
	"encoding/json"
	"encoding/xml"
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

func (s *Service) Available() bool { return s.enabled }

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
	if permalink, err := url.Parse(keyword); err == nil && permalink.Scheme == "https" && strings.EqualFold(permalink.Host, "soundcloud.com") {
		segments := strings.Split(strings.TrimPrefix(permalink.Path, "/"), "/")
		if len(segments) == 2 && segments[0] != "" && segments[1] != "" {
			if offset != 0 {
				return []platform.Music{}, nil
			}
			return s.resolvePermalink(ctx, keyword)
		}
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

func (s *Service) resolvePermalink(ctx context.Context, permalink string) ([]platform.Music, error) {
	if s.clientID == "" {
		return s.publicSearch(ctx, permalink)
	}
	var item track
	if err := s.getJSON(ctx, "/resolve", url.Values{"url": {permalink}}, &item); err != nil {
		if authorizationDenied(err) {
			return s.publicSearch(ctx, permalink)
		}
		var upstream *platform.UpstreamError
		var syntaxError *json.SyntaxError
		var typeError *json.UnmarshalTypeError
		if (errors.As(err, &upstream) && upstream.Status == http.StatusNotFound) ||
			errors.As(err, &syntaxError) || errors.As(err, &typeError) {
			return []platform.Music{}, nil
		}
		return nil, err
	}
	if !item.Streamable || item.ID < 0 || (item.ID == 0 && strings.TrimSpace(item.URN) == "") {
		return []platform.Music{}, nil
	}
	return []platform.Music{item.music()}, nil
}

func (s *Service) trackInfo(ctx context.Context, id string) (platform.Music, error) {
	if strings.TrimSpace(id) == "" {
		return platform.Music{}, errors.New("soundcloud track ID is required")
	}
	if s.clientID == "" {
		return s.publicTrack(ctx, apiBaseURL+"/tracks/"+url.PathEscape(id))
	}
	var item track
	if err := s.getJSON(ctx, "/tracks/"+url.PathEscape(id), url.Values{}, &item); err != nil {
		if authorizationDenied(err) {
			return s.publicTrack(ctx, apiBaseURL+"/tracks/"+url.PathEscape(id))
		}
		return platform.Music{}, err
	}
	return item.music(), nil
}

func authorizationDenied(err error) bool {
	var upstream *platform.UpstreamError
	return errors.As(err, &upstream) && (upstream.Status == http.StatusUnauthorized || upstream.Status == http.StatusForbidden)
}

func (s *Service) publicSearch(ctx context.Context, permalink string) ([]platform.Music, error) {
	music, err := s.publicTrack(ctx, permalink)
	var upstream *platform.UpstreamError
	if errors.As(err, &upstream) && upstream.Status == http.StatusNotFound {
		return []platform.Music{}, nil
	}
	if err != nil {
		return nil, err
	}
	return []platform.Music{music}, nil
}

// Public oEmbed supplies a durable numeric Widget ID without API credentials.
// Duration is unknown here and is reported by the Widget when playback starts.
func (s *Service) publicTrack(ctx context.Context, source string) (platform.Music, error) {
	var response struct {
		Title     string `json:"title"`
		Author    string `json:"author_name"`
		Thumbnail string `json:"thumbnail_url"`
		HTML      string `json:"html"`
	}
	query := url.Values{"url": {source}, "format": {"json"}}
	if err := withoutRequestURL(s.client.JSON(ctx, "https://soundcloud.com/oembed?"+query.Encode(), nil, &response)); err != nil {
		return platform.Music{}, err
	}
	decoder := xml.NewDecoder(strings.NewReader(response.HTML))
	decoder.Strict = false // oEmbed returns HTML, including unescaped ampersands.
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		element, ok := token.(xml.StartElement)
		if !ok || element.Name.Local != "iframe" {
			continue
		}
		for _, attr := range element.Attr {
			if attr.Name.Local != "src" {
				continue
			}
			widget, err := url.Parse(attr.Value)
			if err != nil || widget.Scheme != "https" || widget.Host != "w.soundcloud.com" || widget.Path != "/player/" || widget.User != nil {
				continue
			}
			trackURL, err := url.Parse(widget.Query().Get("url"))
			if err != nil || trackURL.Scheme != "https" || trackURL.Host != "api.soundcloud.com" || trackURL.User != nil {
				continue
			}
			id := strings.TrimPrefix(trackURL.Path, "/tracks/")
			numeric, err := strconv.ParseInt(id, 10, 64)
			if !strings.HasPrefix(trackURL.Path, "/tracks/") || err != nil || numeric <= 0 || strconv.FormatInt(numeric, 10) != id {
				continue
			}
			if strings.TrimSpace(response.Title) == "" {
				continue
			}
			return platform.Music{ID: id, Name: strings.TrimSuffix(response.Title, " by "+response.Author), Artists: []string{response.Author}, Platform: "soundcloud", CoverURL: response.Thumbnail}, nil
		}
	}
	return platform.Music{}, errors.New("soundcloud oEmbed did not return a track Widget")
}

func (s *Service) ResolvePlayable(ctx context.Context, id string) (platform.PlayableMusic, error) {
	music, err := s.trackInfo(ctx, id)
	if err != nil {
		return platform.PlayableMusic{}, err
	}
	return platform.PlayableMusic{Music: music, PlayerType: "soundcloud-widget", EmbedID: music.ID}, nil
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
