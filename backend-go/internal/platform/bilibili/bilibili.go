package bilibili

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform"
)

type Service struct {
	client          *platform.Client
	baseURL, cookie string
	now             func() time.Time
	mu              sync.Mutex
	mixin           string
	mixinAt         time.Time
}

func New(client *platform.Client, baseURL, cookie string) *Service {
	return &Service{client: client, baseURL: strings.TrimRight(baseURL, "/"), cookie: cookie, now: time.Now}
}
func (*Service) Name() string    { return "bilibili" }
func (*Service) Available() bool { return true }

func (s *Service) Search(ctx context.Context, keyword string, offset, limit int) ([]platform.Music, error) {
	if strings.TrimSpace(s.cookie) == "" {
		return nil, errors.New("bilibili SESSDATA is not configured")
	}
	page := offset/max(1, limit) + 1
	query, err := s.sign(ctx, url.Values{"search_type": {"video"}, "keyword": {keyword}, "page": {strconv.Itoa(page)}, "page_size": {strconv.Itoa(limit)}})
	if err != nil {
		return nil, err
	}
	var response struct {
		Code int `json:"code"`
		Data struct {
			Result []struct {
				BVID     string `json:"bvid"`
				Title    string `json:"title"`
				Author   string `json:"author"`
				Duration string `json:"duration"`
				Pic      string `json:"pic"`
			} `json:"result"`
		} `json:"data"`
	}
	headers := map[string]string{"Referer": "https://www.bilibili.com/", "Cookie": cookieHeader(s.cookie), "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120 Safari/537.36"}
	if err := s.client.JSON(ctx, s.baseURL+"/x/web-interface/wbi/search/type?"+query, headers, &response); err != nil {
		return nil, err
	}
	if response.Code != 0 {
		return []platform.Music{}, nil
	}
	result := make([]platform.Music, 0, len(response.Data.Result))
	for _, item := range response.Data.Result {
		result = append(result, platform.Music{ID: item.BVID, Name: stripHTML(item.Title), Artists: []string{item.Author}, Duration: duration(item.Duration), Platform: "bilibili", CoverURL: normalizeImage(item.Pic)})
	}
	return result, nil
}
func (s *Service) ResolvePlayable(ctx context.Context, bvid string) (platform.PlayableMusic, error) {
	var response struct {
		Code int `json:"code"`
		Data struct {
			BVID, Title, Pic string
			Duration         int64
			Owner            struct {
				Name string `json:"name"`
			} `json:"owner"`
		} `json:"data"`
	}
	if err := s.client.JSON(ctx, s.baseURL+"/x/web-interface/view?"+url.Values{"bvid": {bvid}}.Encode(), s.headers(), &response); err != nil {
		return platform.PlayableMusic{}, err
	}
	if response.Code != 0 || response.Data.BVID == "" {
		return platform.PlayableMusic{}, fmt.Errorf("bilibili video %s was not found", bvid)
	}
	music := platform.Music{ID: response.Data.BVID, Name: response.Data.Title, Artists: []string{response.Data.Owner.Name}, Duration: response.Data.Duration * 1000, Platform: "bilibili", CoverURL: normalizeImage(response.Data.Pic)}
	return platform.PlayableMusic{Music: music, URL: "/api/bilibili/stream/" + url.PathEscape(bvid)}, nil
}

var mixinTable = []int{46, 47, 18, 2, 53, 8, 23, 32, 15, 50, 10, 31, 58, 3, 45, 35, 27, 43, 5, 49, 33, 9, 42, 19, 29, 28, 14, 39, 12, 38, 41, 13, 37, 48, 7, 16, 24, 55, 40, 61, 26, 17, 0, 1, 60, 51, 30, 4, 22, 25, 54, 21, 56, 59, 6, 63, 57, 62, 11, 36, 20, 34, 44, 52}

func (s *Service) sign(ctx context.Context, values url.Values) (string, error) {
	key, err := s.mixinKey(ctx)
	if err != nil {
		return "", err
	}
	copyValues := url.Values{}
	for k, v := range values {
		copyValues[k] = append([]string(nil), v...)
	}
	copyValues.Set("wts", strconv.FormatInt(s.now().Unix(), 10))
	keys := make([]string, 0, len(copyValues))
	for key := range copyValues {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, encode(key)+"="+encode(copyValues.Get(key)))
	}
	query := strings.Join(parts, "&")
	sum := md5.Sum([]byte(query + key))
	return query + "&w_rid=" + hex.EncodeToString(sum[:]), nil
}
func (s *Service) mixinKey(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.mixin != "" && s.now().Sub(s.mixinAt) < time.Hour {
		return s.mixin, nil
	}
	var response struct {
		Data struct {
			WBI struct {
				ImageURL string `json:"img_url"`
				SubURL   string `json:"sub_url"`
			} `json:"wbi_img"`
		} `json:"data"`
	}
	headers := map[string]string{"Cookie": cookieHeader(s.cookie), "User-Agent": "Mozilla/5.0"}
	if err := s.client.JSON(ctx, s.baseURL+"/x/web-interface/nav", headers, &response); err != nil {
		return "", err
	}
	raw := fileKey(response.Data.WBI.ImageURL) + fileKey(response.Data.WBI.SubURL)
	var builder strings.Builder
	for _, index := range mixinTable {
		if index < len(raw) {
			builder.WriteByte(raw[index])
		}
	}
	key := builder.String()
	if len(key) > 32 {
		key = key[:32]
	}
	if len(key) < 32 {
		return "", errors.New("invalid Bilibili WBI key")
	}
	s.mixin = key
	s.mixinAt = s.now()
	return key, nil
}
func encode(value string) string { return strings.ReplaceAll(url.QueryEscape(value), "+", "%20") }
func fileKey(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	name := path.Base(parsed.Path)
	return strings.TrimSuffix(name, path.Ext(name))
}
func cookieHeader(raw string) string {
	if raw == "" {
		return ""
	}
	if strings.Contains(raw, "=") {
		return raw
	}
	return "SESSDATA=" + raw
}
func stripHTML(value string) string {
	for {
		start := strings.IndexByte(value, '<')
		if start < 0 {
			break
		}
		end := strings.IndexByte(value[start:], '>')
		if end < 0 {
			break
		}
		value = value[:start] + value[start+end+1:]
	}
	return value
}
func normalizeImage(value string) string {
	if strings.HasPrefix(value, "//") {
		return "https:" + value
	}
	return value
}
func duration(value string) int64 {
	parts := strings.Split(value, ":")
	var seconds int64
	for _, part := range parts {
		number, _ := strconv.ParseInt(part, 10, 64)
		seconds = seconds*60 + number
	}
	return seconds * 1000
}
func (s *Service) UserPlaylists(ctx context.Context, userID string) ([]platform.Playlist, error) {
	if strings.TrimSpace(s.cookie) == "" {
		return nil, errors.New("bilibili SESSDATA is not configured")
	}
	var response struct {
		Code int `json:"code"`
		Data struct {
			List []struct {
				ID    jsonID `json:"id"`
				Title string `json:"title"`
				Cover string `json:"cover"`
				Count int    `json:"media_count"`
			} `json:"list"`
		} `json:"data"`
	}
	query := url.Values{"up_mid": {userID}}
	if err := s.client.JSON(ctx, s.baseURL+"/x/v3/fav/folder/created/list-all?"+query.Encode(), s.headers(), &response); err != nil {
		return nil, err
	}
	result := []platform.Playlist{}
	if response.Code != 0 {
		return result, nil
	}
	for _, item := range response.Data.List {
		if item.Count > 0 {
			result = append(result, platform.Playlist{ID: item.ID.String(), Name: item.Title, CoverImgURL: item.Cover, TrackCount: item.Count, Platform: "bilibili"})
		}
	}
	return result, nil
}
func (s *Service) PlaylistSongs(ctx context.Context, playlistID string, offset, limit int) ([]platform.Music, error) {
	if strings.TrimSpace(s.cookie) == "" {
		return nil, errors.New("bilibili SESSDATA is not configured")
	}
	limit = min(limit, 20)
	page := offset/max(1, limit) + 1
	query := url.Values{"media_id": {playlistID}, "ps": {strconv.Itoa(limit)}, "pn": {strconv.Itoa(page)}}
	var response struct {
		Code int `json:"code"`
		Data struct {
			Medias []struct {
				BVID     string `json:"bvid"`
				Title    string `json:"title"`
				Duration int64  `json:"duration"`
				Cover    string `json:"cover"`
				Upper    struct {
					Name string `json:"name"`
				} `json:"upper"`
			} `json:"medias"`
		} `json:"data"`
	}
	if err := s.client.JSON(ctx, s.baseURL+"/x/v3/fav/resource/list?"+query.Encode(), s.headers(), &response); err != nil {
		return nil, err
	}
	result := []platform.Music{}
	if response.Code != 0 {
		return result, nil
	}
	for _, item := range response.Data.Medias {
		if item.Title == "已失效视频" {
			result = append(result, platform.Music{ID: "INVALID_SKIP", Name: item.Title, Artists: []string{"Unknown"}, Platform: "bilibili"})
			continue
		}
		result = append(result, platform.Music{ID: item.BVID, Name: item.Title, Artists: []string{item.Upper.Name}, Duration: item.Duration * 1000, Platform: "bilibili", CoverURL: item.Cover})
	}
	return result, nil
}
func (*Service) SearchAlbums(context.Context, string) ([]platform.Album, error) {
	return []platform.Album{}, nil
}
func (*Service) AlbumSongs(context.Context, string) ([]platform.Music, error) {
	return []platform.Music{}, nil
}
func (s *Service) StreamURL(ctx context.Context, bvid string) (string, error) {
	var view struct {
		Code int `json:"code"`
		Data struct {
			CID int64 `json:"cid"`
		} `json:"data"`
	}
	if err := s.client.JSON(ctx, s.baseURL+"/x/web-interface/view?"+url.Values{"bvid": {bvid}}.Encode(), s.headers(), &view); err != nil {
		return "", err
	}
	if view.Code != 0 || view.Data.CID == 0 {
		return "", errors.New("bilibili video metadata is unavailable")
	}
	var play struct {
		Code int `json:"code"`
		Data struct {
			Dash struct {
				Audio []struct {
					BaseURL string   `json:"baseUrl"`
					Backup  []string `json:"backupUrl"`
				} `json:"audio"`
			} `json:"dash"`
			DURL []struct {
				URL string `json:"url"`
			} `json:"durl"`
		} `json:"data"`
	}
	query := url.Values{"bvid": {bvid}, "cid": {strconv.FormatInt(view.Data.CID, 10)}, "fnval": {"16"}, "qn": {"64"}}
	if err := s.client.JSON(ctx, s.baseURL+"/x/player/playurl?"+query.Encode(), s.headers(), &play); err != nil {
		return "", err
	}
	if len(play.Data.Dash.Audio) > 0 {
		return play.Data.Dash.Audio[0].BaseURL, nil
	}
	if len(play.Data.DURL) > 0 {
		return play.Data.DURL[0].URL, nil
	}
	return "", errors.New("bilibili audio URL is unavailable")
}
func (*Service) CoverURL(context.Context, string) (string, error) { return "", nil }
func (s *Service) SearchUsers(ctx context.Context, keyword string) ([]platform.User, error) {
	if strings.TrimSpace(s.cookie) == "" {
		return nil, errors.New("bilibili SESSDATA is not configured")
	}
	query, err := s.sign(ctx, url.Values{"search_type": {"bili_user"}, "keyword": {keyword}})
	if err != nil {
		return nil, err
	}
	var response struct {
		Code int `json:"code"`
		Data struct {
			Result []struct {
				MID     jsonID `json:"mid"`
				Name    string `json:"uname"`
				Picture string `json:"upic"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := s.client.JSON(ctx, s.baseURL+"/x/web-interface/wbi/search/type?"+query, s.headers(), &response); err != nil {
		return nil, err
	}
	result := []platform.User{}
	if response.Code != 0 {
		return result, nil
	}
	for _, item := range response.Data.Result {
		result = append(result, platform.User{ID: item.MID.String(), Name: item.Name, AvatarURL: normalizeImage(item.Picture), Platform: "bilibili"})
	}
	return result, nil
}
func (*Service) Lyric(context.Context, string) (platform.Lyric, error) { return platform.Lyric{}, nil }
func (s *Service) headers() map[string]string {
	return map[string]string{"Referer": "https://www.bilibili.com/", "Cookie": cookieHeader(s.cookie), "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120 Safari/537.36"}
}

type jsonID string

func (id *jsonID) UnmarshalJSON(data []byte) error {
	*id = jsonID(strings.Trim(string(data), "\""))
	return nil
}
func (id jsonID) String() string { return string(id) }
