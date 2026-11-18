package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultBaseURL = "https://bandcamp.com/api/subsonic"
	salt           = "musicparty"
	userAgent      = "MusicParty-BandcampProbe/1.0"

	pass = "PASS"
	fail = "FAIL"
	warn = "WARN"
	skip = "SKIP"
)

type probe struct {
	baseURL  *url.URL
	client   *http.Client
	username string
	token    string
}

type song struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Artist   string `json:"artist"`
	Duration int64  `json:"duration"`
	CoverArt string `json:"coverArt"`
}

type subsonicResponse struct {
	Body struct {
		Status string `json:"status"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
		Search struct {
			Songs []song `json:"song"`
		} `json:"searchResult3"`
	} `json:"subsonic-response"`
}

type result struct {
	status string
	detail string
}

type check struct {
	name string
	result
}

func main() {
	username := os.Getenv("BANDCAMP_USERNAME")
	password := os.Getenv("BANDCAMP_PASSWORD")
	if username == "" || password == "" {
		fmt.Fprintln(os.Stderr, "Set BANDCAMP_USERNAME and BANDCAMP_PASSWORD, then run: go -C tools/bandcamp-probe run .")
		fmt.Fprintln(os.Stderr, "Optional: set BANDCAMP_BASE_URL to override "+defaultBaseURL)
		os.Exit(2)
	}

	base := os.Getenv("BANDCAMP_BASE_URL")
	if base == "" {
		base = defaultBaseURL
	}
	baseURL, err := url.Parse(base)
	if err != nil || (baseURL.Scheme != "http" && baseURL.Scheme != "https") || baseURL.Hostname() == "" || baseURL.User != nil {
		fmt.Fprintln(os.Stderr, "BANDCAMP_BASE_URL must be an absolute HTTP or HTTPS URL without user info.")
		os.Exit(2)
	}

	// Match the MusicParty provider's Subsonic token authentication.
	sum := md5.Sum([]byte(password + salt))
	p := probe{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 15 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return fmt.Errorf("too many redirects")
				}
				// Stream redirects can point to a CDN. Keep auth query parameters out of Referer.
				req.Header.Del("Referer")
				return nil
			},
		},
		username: username,
		token:    hex.EncodeToString(sum[:]),
	}
	if !printSummary(p.run()) {
		os.Exit(1)
	}
}

func (p probe) run() []check {
	ping := result{status: pass}
	if _, err := p.getJSON("ping.view", nil); err != nil {
		ping = failed(err)
	}

	songs, search := p.search()
	var songID, coverID string
	for _, song := range songs {
		if songID == "" {
			songID = song.ID
		}
		if coverID == "" {
			coverID = song.CoverArt
		}
	}

	cover := result{skip, "no coverArt ID"}
	if coverID != "" {
		cover = p.cover(coverID)
	}

	stream := result{skip, "no song ID"}
	rangeSupport, seek := stream, stream
	if songID != "" {
		stream = p.stream(songID)
		rangeSupport = p.checkRange(songID, "0-1023", false)
		seek = p.checkRange(songID, "65536-73727", true)
	}

	return []check{
		{"ping", ping},
		{"search3", search},
		{"getCoverArt", cover},
		{"stream", stream},
		{"range-support", rangeSupport},
		{"seek", seek},
	}
}

func (p probe) get(endpoint string, values url.Values, byteRange string) (*http.Response, error) {
	if values == nil {
		values = make(url.Values)
	}
	values.Set("u", p.username)
	values.Set("t", p.token)
	values.Set("s", salt)
	values.Set("v", "1.16.1")
	values.Set("c", "MusicParty")
	values.Set("f", "json")
	requestURL := p.baseURL.JoinPath("rest", endpoint)
	requestURL.RawQuery = values.Encode()
	req, err := http.NewRequest(http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("could not create Subsonic request")
	}
	req.Header.Set("User-Agent", userAgent)
	if byteRange != "" {
		req.Header.Set("Range", "bytes="+byteRange)
	}
	resp, err := p.client.Do(req)
	// The URL in a transport error includes the username and auth token.
	if urlErr, ok := err.(*url.Error); ok {
		err = urlErr.Err
	}
	return resp, err
}

func (p probe) getJSON(endpoint string, values url.Values) (subsonicResponse, error) {
	var data subsonicResponse
	resp, err := p.get(endpoint, values, "")
	if err != nil {
		return data, err
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&data); err != nil {
		return data, fmt.Errorf("HTTP %d: invalid Subsonic JSON: %w", resp.StatusCode, err)
	}
	if apiErr := data.Body.Error; apiErr != nil {
		return data, fmt.Errorf("Subsonic error %d: %s", apiErr.Code, apiErr.Message)
	}
	if resp.StatusCode != http.StatusOK || data.Body.Status != "ok" {
		return data, fmt.Errorf("HTTP %d: Subsonic status %q", resp.StatusCode, data.Body.Status)
	}
	return data, nil
}

func (p probe) search() ([]song, result) {
	data, err := p.getJSON("search3.view", url.Values{
		"query": {"jazz"}, "artistCount": {"3"}, "albumCount": {"3"}, "songCount": {"3"},
	})
	if err != nil {
		return nil, failed(err)
	}
	songs := data.Body.Search.Songs
	if len(songs) == 0 {
		return nil, result{warn, "no songs returned for jazz"}
	}
	first := songs[0]
	fmt.Printf("First song: id=%q title=%q artist=%q duration=%ds", first.ID, first.Title, first.Artist, first.Duration)
	if first.CoverArt != "" {
		fmt.Printf(" coverArt=%q", first.CoverArt)
	}
	fmt.Println()
	if len(songs) == 1 {
		return songs, result{pass, "1 song"}
	}
	return songs, result{pass, fmt.Sprintf("%d songs", len(songs))}
}

func (p probe) cover(id string) result {
	resp, err := p.get("getCoverArt.view", url.Values{"id": {id}, "size": {"300"}}, "")
	if err != nil {
		return failed(err)
	}
	defer resp.Body.Close()
	mediaType, err := sampleMedia(resp, "image/")
	if err != nil {
		return failed(err)
	}
	return result{pass, mediaType}
}

func (p probe) stream(id string) result {
	resp, err := p.getStream(id, "")
	if err != nil {
		return failed(err)
	}
	defer resp.Body.Close()
	mediaType, err := sampleMedia(resp, "audio/")
	if err != nil {
		return failed(err)
	}
	length := "not provided"
	if resp.ContentLength >= 0 {
		length = strconv.FormatInt(resp.ContentLength, 10)
	}
	return result{pass, fmt.Sprintf("%s, Content-Length: %s, Accept-Ranges: %s", mediaType, length, headerValue(resp, "Accept-Ranges"))}
}

// Read just enough to check that a successful media response has a body.
func sampleMedia(resp *http.Response, prefix string) (string, error) {
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	mediaType, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil || !strings.HasPrefix(mediaType, prefix) {
		return "", fmt.Errorf("expected %s*, got Content-Type %q", prefix, resp.Header.Get("Content-Type"))
	}
	var sample [1]byte
	if _, err := io.ReadFull(resp.Body, sample[:]); err != nil {
		return "", fmt.Errorf("could not read media bytes: %w", err)
	}
	return mediaType, nil
}

func (p probe) getStream(id, byteRange string) (*http.Response, error) {
	return p.get("stream.view", url.Values{"id": {id}, "format": {"mp3"}}, byteRange)
}

func (p probe) checkRange(id, byteRange string, seek bool) result {
	resp, err := p.getStream(id, byteRange)
	if err != nil {
		return failed(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK && !seek {
		if _, err := sampleMedia(resp, "audio/"); err != nil {
			return failed(err)
		}
		return result{warn, "HTTP 200, server ignored Range"}
	}
	if resp.StatusCode != http.StatusPartialContent {
		return result{fail, fmt.Sprintf("expected HTTP 206, got %d, Content-Range: %s", resp.StatusCode, headerValue(resp, "Content-Range"))}
	}

	contentRange := resp.Header.Get("Content-Range")
	if contentRange == "" {
		return result{fail, "HTTP 206 without Content-Range"}
	}
	if seek && !matchesContentRange(contentRange, byteRange) {
		return result{fail, fmt.Sprintf("expected Content-Range bytes %s/<length>, got %q", byteRange, contentRange)}
	}
	return result{pass, "206 + Content-Range: " + contentRange}
}

// Check both seek offsets and the reported representation length, if known.
func matchesContentRange(header, byteRange string) bool {
	unit, bounds, _ := strings.Cut(header, " ")
	positions, length, _ := strings.Cut(bounds, "/")
	if !strings.EqualFold(unit, "bytes") || positions != byteRange {
		return false
	}
	if length == "*" {
		return true
	}
	_, end, _ := strings.Cut(byteRange, "-")
	last, endErr := strconv.ParseUint(end, 10, 64)
	total, err := strconv.ParseUint(length, 10, 64)
	return endErr == nil && err == nil && total > last
}

func headerValue(resp *http.Response, name string) string {
	if value := resp.Header.Get(name); value != "" {
		return value
	}
	return "not provided"
}

func failed(err error) result {
	return result{fail, err.Error()}
}

// Print the summary and report whether any check failed.
func printSummary(checks []check) bool {
	fmt.Println("Check          | Result")
	fmt.Println("---------------|--------")
	var failures []string
	for _, c := range checks {
		fmt.Printf("%-14s | %s", c.name, c.status)
		if c.detail != "" {
			fmt.Printf(" (%s)", c.detail)
		}
		fmt.Println()
		if c.status == fail {
			failures = append(failures, c.name)
		}
	}
	if len(failures) > 0 {
		fmt.Printf("\nOVERALL: NOT COMPATIBLE — %s. Fall back to embed/external-link per the plan.\n", strings.Join(failures, ", "))
		return false
	}
	fmt.Println("\nOVERALL: COMPATIBLE — add Bandcamp as a Subsonic source in the admin UI.")
	return true
}
