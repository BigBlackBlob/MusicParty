package spotify

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform"
)

const (
	searchPageSize  = 10
	maxSearchOffset = 1000
	maxCatalogBody  = 2 << 20
	trackURIPrefix  = "spotify:track:"
)

var trackIDPattern = regexp.MustCompile(`^[a-zA-Z0-9]{22}$`)

type Config struct {
	Enabled                        bool
	ClientID, ClientSecret, Market string
}

type Service struct {
	platform.Unsupported
	enabled  bool
	tokenMgr *tokenManager
	client   *http.Client
	apiBase  string
	market   string
}

var (
	_ platform.Service          = (*Service)(nil)
	_ platform.PlayableResolver = (*Service)(nil)
)

func New(cfg Config) (*Service, bool) {
	id, secret := strings.TrimSpace(cfg.ClientID), strings.TrimSpace(cfg.ClientSecret)
	if !cfg.Enabled || id == "" || secret == "" {
		return nil, false
	}
	client := &http.Client{
		Timeout:       15 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	market := strings.ToUpper(strings.TrimSpace(cfg.Market))
	if market == "" {
		market = "US"
	}
	return &Service{
		Unsupported: platform.Unsupported{Platform: "spotify"}, enabled: cfg.Enabled, client: client,
		apiBase: "https://api.spotify.com", market: market,
		tokenMgr: &tokenManager{clientID: id, clientSecret: secret, baseURL: "https://accounts.spotify.com", client: client, now: time.Now},
	}, true
}

func (*Service) Name() string { return "spotify" }
func (s *Service) Available() bool {
	return s != nil && s.enabled && s.tokenMgr != nil && s.tokenMgr.clientID != "" && s.tokenMgr.clientSecret != ""
}

func (s *Service) get(ctx context.Context, path string, query url.Values, target any) error {
	if !s.Available() {
		return errors.New("spotify: disabled or not configured")
	}
	query.Set("market", s.market)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.apiBase+path+"?"+query.Encode(), nil)
	if err != nil {
		return errors.New("spotify: invalid catalog request")
	}
	for attempt := 0; ; attempt++ {
		token, err := s.tokenMgr.AppToken(ctx)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		err = requestJSON(s.client, req, "catalog", maxCatalogBody, target)
		var upstream *responseError
		if !errors.As(err, &upstream) || upstream.status != http.StatusUnauthorized {
			return err
		}
		s.tokenMgr.Invalidate(token)
		if attempt == 1 {
			return err
		}
	}
}

type track struct {
	ID       string `json:"id"`
	URI      string `json:"uri"`
	Name     string `json:"name"`
	Duration int64  `json:"duration_ms"`
	Artists  []struct {
		Name string `json:"name"`
	} `json:"artists"`
	Album struct {
		Images []struct {
			URL string `json:"url"`
		} `json:"images"`
	} `json:"album"`
}

func (t track) music() platform.Music {
	music := platform.Music{ID: t.ID, Name: t.Name, Duration: t.Duration, Platform: "spotify", Artists: make([]string, 0, len(t.Artists))}
	for _, artist := range t.Artists {
		music.Artists = append(music.Artists, artist.Name)
	}
	if len(t.Album.Images) > 0 {
		music.CoverURL = t.Album.Images[0].URL
	}
	return music
}

func (s *Service) Search(ctx context.Context, keyword string, offset, limit int) ([]platform.Music, error) {
	if err := platform.ValidateSearch(keyword, offset, limit); err != nil {
		return nil, err
	}
	result := make([]platform.Music, 0, limit)
	for limit > 0 && offset <= maxSearchOffset {
		batch := min(limit, searchPageSize)
		query := url.Values{"type": {"track"}, "q": {keyword}, "offset": {strconv.Itoa(offset)}, "limit": {strconv.Itoa(batch)}}
		var page struct {
			Tracks struct {
				Items []track `json:"items"`
				Total int     `json:"total"`
				Next  string  `json:"next"`
			} `json:"tracks"`
		}
		if err := s.get(ctx, "/v1/search", query, &page); err != nil {
			return nil, err
		}
		count := min(batch, len(page.Tracks.Items))
		for _, item := range page.Tracks.Items[:count] {
			if trackIDPattern.MatchString(item.ID) {
				result = append(result, item.music())
			}
		}
		offset += count
		limit -= count
		if count < batch || (page.Tracks.Next == "" && offset >= page.Tracks.Total) {
			break
		}
	}
	return result, nil
}

func (s *Service) ResolvePlayable(ctx context.Context, id string) (platform.PlayableMusic, error) {
	if !trackIDPattern.MatchString(id) {
		return platform.PlayableMusic{}, errors.New("spotify: invalid track ID")
	}
	var item track
	if err := s.get(ctx, "/v1/tracks/"+id, url.Values{}, &item); err != nil {
		return platform.PlayableMusic{}, err
	}
	if !trackIDPattern.MatchString(item.ID) || item.URI != trackURIPrefix+item.ID || strings.TrimSpace(item.Name) == "" || item.Duration <= 0 {
		return platform.PlayableMusic{}, errors.New("spotify: invalid track metadata")
	}
	return platform.PlayableMusic{Music: item.music(), PlayerType: "spotify-embed", EmbedID: item.URI}, nil
}

func (*Service) StreamURL(context.Context, string) (string, error) { return "", nil }

func (*Service) SearchAlbums(context.Context, string, int, int) (platform.AlbumSearchResult, error) {
	return platform.AlbumSearchResult{Items: []platform.Album{}}, nil
}
