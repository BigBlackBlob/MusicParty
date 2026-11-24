package soundcloud

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/media"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform"
)

// apiBaseURL is the public v2 API. It accepts a client_id alone, so SoundCloud
// audio can be resolved without an OAuth token or a client secret.
const apiBaseURL = "https://api-v2.soundcloud.com"

// widgetTrackBaseURL is the canonical track address that the public oEmbed
// endpoint and the player widget both understand.
const widgetTrackBaseURL = "https://api.soundcloud.com"

type Service struct {
	client      *platform.Client
	enabled     bool
	clientID    string
	ytdlpPath   string
	ffmpegPath  string
	mu          sync.RWMutex
	accessToken string
}

// New uses an externally provisioned token, so the client secret is not retained.
func New(client *platform.Client, enabled bool, clientID, _ string) *Service {
	return &Service{client: client, enabled: enabled, clientID: clientID}
}

// SetDownloadTools configures the external binaries used to materialize audio.
// Without them the service still resolves tracks, but only the widget can play them.
func (s *Service) SetDownloadTools(ytdlpPath, ffmpegPath string) {
	s.mu.Lock()
	s.ytdlpPath, s.ffmpegPath = ytdlpPath, ffmpegPath
	s.mu.Unlock()
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
	// Policy classifies the upstream rights: ALLOW, SNIP or MONETIZE.
	Policy string `json:"policy"`
	User   struct {
		Username string `json:"username"`
	} `json:"user"`
	Publisher struct {
		Artist string `json:"artist"`
	} `json:"publisher_metadata"`
	Media struct {
		Transcodings []transcoding `json:"transcodings"`
	} `json:"media"`
}

// transcoding is one upstream rendition. The protocol tells us whether the
// audio is plain HLS or DRM-protected.
type transcoding struct {
	URL    string `json:"url"`
	Preset string `json:"preset"`
	Format struct {
		Protocol string `json:"protocol"`
		MimeType string `json:"mime_type"`
	} `json:"format"`
}

// access describes how the host may play a track.
type access int

const (
	// accessNative means the host can fetch and serve the full audio.
	accessNative access = iota
	// accessTrial means only a 30 second preview exists.
	accessTrial
	// accessDRM means the audio is encrypted and only the widget can play it.
	accessDRM
)

func (t track) artist() string {
	if name := strings.TrimSpace(t.Publisher.Artist); name != "" {
		return name
	}
	return t.User.Username
}

func (t track) music() platform.Music {
	id := t.URN
	if t.ID != 0 {
		id = strconv.FormatInt(t.ID, 10)
	}
	return platform.Music{
		ID: id, Name: t.Title,
		Artists: []string{t.artist()}, Duration: t.Duration,
		Platform: "soundcloud", CoverURL: t.ArtworkURL,
	}
}

// classify decides how the host may play the track. SNIP tracks only ever
// expose a short preview, and encrypted renditions cannot be remuxed at all.
func (t track) classify() access {
	for _, rendition := range t.Media.Transcodings {
		if strings.Contains(strings.ToLower(rendition.Format.Protocol), "encrypted") {
			return accessDRM
		}
	}
	if strings.EqualFold(t.Policy, "SNIP") {
		return accessTrial
	}
	for _, rendition := range t.Media.Transcodings {
		if strings.EqualFold(rendition.Format.Protocol, "hls") {
			return accessNative
		}
	}
	return accessDRM
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
	}
	var response searchResponse
	if err := s.getJSON(ctx, "/search/tracks", query, &response); err != nil {
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
		return s.publicTrack(ctx, widgetTrackBaseURL+"/tracks/"+url.PathEscape(id))
	}
	var item track
	if err := s.getJSON(ctx, "/tracks/"+url.PathEscape(id), url.Values{}, &item); err != nil {
		if authorizationDenied(err) {
			return s.publicTrack(ctx, widgetTrackBaseURL+"/tracks/"+url.PathEscape(id))
		}
		return platform.Music{}, err
	}
	return item.music(), nil
}

// trackDetails loads the full track, including the renditions that decide
// whether the host may play it natively.
func (s *Service) trackDetails(ctx context.Context, id string) (track, error) {
	if strings.TrimSpace(id) == "" {
		return track{}, errors.New("soundcloud track ID is required")
	}
	if s.clientID == "" {
		return track{}, errors.New("soundcloud client ID is required to inspect a track")
	}
	var item track
	if err := s.getJSON(ctx, "/tracks/"+url.PathEscape(id), url.Values{}, &item); err != nil {
		return track{}, err
	}
	return item, nil
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

// ResolvePlayable picks the playback engine for a track. The widget plays every
// track the host cannot fetch itself, and preview-only tracks are rejected so
// the room never queues a track it cannot play in full.
//
// The native engine is chosen later, once the host has cached the audio, so
// the queue item keeps the widget as its fallback.
func (s *Service) ResolvePlayable(ctx context.Context, id string) (platform.PlayableMusic, error) {
	music, err := s.trackInfo(ctx, id)
	if err != nil {
		return platform.PlayableMusic{}, err
	}
	details, detailErr := s.trackDetails(ctx, id)
	if detailErr == nil && details.classify() == accessTrial {
		return platform.PlayableMusic{}, &platform.StreamResolutionError{
			Kind: platform.StreamTrialOnly, Reason: http.StatusUnprocessableEntity,
			Detail: "soundcloud track " + id + " only offers a preview",
		}
	}
	return platform.PlayableMusic{Music: music, PlayerType: "soundcloud-widget", EmbedID: music.ID}, nil
}

// StreamURL resolves the upstream media location for a track. SoundCloud serves
// HLS segments behind short-lived signed URLs, so the resolved location is only
// valid for the lifetime of its signature.
func (s *Service) StreamURL(ctx context.Context, id string) (string, error) {
	if strings.TrimSpace(id) == "" {
		return "", errors.New("soundcloud track ID is required")
	}
	details, err := s.trackDetails(ctx, id)
	if err != nil {
		return "", err
	}
	switch details.classify() {
	case accessTrial:
		return "", &platform.StreamResolutionError{
			Kind: platform.StreamTrialOnly, Reason: http.StatusUnprocessableEntity,
			Detail: "soundcloud track " + id + " only offers a preview",
		}
	case accessDRM:
		return "", &platform.StreamResolutionError{
			Kind: platform.StreamUnavailable, Reason: http.StatusForbidden,
			Detail: "soundcloud track " + id + " is protected and must use the widget",
		}
	}
	for _, rendition := range details.Media.Transcodings {
		if !strings.EqualFold(rendition.Format.Protocol, "hls") {
			continue
		}
		target, err := s.resolveRendition(ctx, rendition.URL)
		if err == nil && target != "" {
			return target, nil
		}
	}
	return "", errors.New("soundcloud track has no playable rendition")
}

// Download materializes the track audio into a cache-owned file.
//
// yt-dlp resolves the public playback location on its own, so no SoundCloud
// credential is required. The result is a single seekable file that the host
// can serve with HTTP range support.
func (s *Service) Download(ctx context.Context, id, target string, maxBytes int64) error {
	if strings.TrimSpace(id) == "" {
		return errors.New("soundcloud track ID is required")
	}
	details, err := s.trackDetails(ctx, id)
	if err != nil {
		return err
	}
	switch details.classify() {
	case accessTrial:
		return &platform.StreamResolutionError{
			Kind: platform.StreamTrialOnly, Reason: http.StatusUnprocessableEntity,
			Detail: "soundcloud track " + id + " only offers a preview",
		}
	case accessDRM:
		return &platform.StreamResolutionError{
			Kind: platform.StreamUnavailable, Reason: http.StatusForbidden,
			Detail: "soundcloud track " + id + " is protected and must use the widget",
		}
	}
	s.mu.RLock()
	ytdlpPath, ffmpegPath := s.ytdlpPath, s.ffmpegPath
	s.mu.RUnlock()
	return media.DownloadSoundCloud(ctx, ytdlpPath, ffmpegPath, id, target, maxBytes)
}

// resolveRendition exchanges a transcoding location for the signed media URL.
// The media endpoint requires the client_id as a query parameter.
func (s *Service) resolveRendition(ctx context.Context, transcodingURL string) (string, error) {
	if strings.TrimSpace(transcodingURL) == "" {
		return "", errors.New("soundcloud rendition has no location")
	}
	parsed, err := url.Parse(transcodingURL)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	query.Set("client_id", s.clientID)
	parsed.RawQuery = query.Encode()
	var payload struct {
		URL string `json:"url"`
	}
	if err := s.client.JSON(ctx, parsed.String(), nil, &payload); err != nil {
		return "", withoutRequestURL(err)
	}
	if strings.TrimSpace(payload.URL) == "" {
		return "", errors.New("soundcloud rendition did not resolve to a media URL")
	}
	return payload.URL, nil
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
