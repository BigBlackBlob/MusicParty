package httpapi

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"syscall"
	"time"
)

type coverPalette struct {
	Accent       string `json:"accent"`
	AccentHover  string `json:"accentHover"`
	AccentMuted  string `json:"accentMuted"`
	AccentSubtle string `json:"accentSubtle"`
}

type cachedCoverColor struct {
	value     coverPalette
	expiresAt time.Time
}

// upstreamCoverURL accepts only the two providers' CDN domains and requests thumbnails.
func upstreamCoverURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", errors.New("invalid cover URL")
	}
	host := strings.ToLower(u.Hostname())
	music := host == "music.126.net" || strings.HasSuffix(host, ".music.126.net")
	bilibili := host == "hdslb.com" || strings.HasSuffix(host, ".hdslb.com")
	if (!music && !bilibili) || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.Fragment != "" || (u.Port() != "" && u.Port() != "443") || strings.Contains(u.Host, ":") && u.Port() == "" {
		return "", errors.New("unsupported cover URL")
	}
	if net.ParseIP(host) != nil {
		return "", errors.New("IP cover hosts are forbidden")
	}
	// Reject malformed DNS labels, trailing dots, empty labels and encoded host tricks.
	for _, label := range strings.Split(host, ".") {
		if label == "" || label[0] == '-' || label[len(label)-1] == '-' {
			return "", errors.New("invalid cover host")
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return "", errors.New("invalid cover host")
			}
		}
	}
	u.Scheme, u.Host = "https", host
	u.RawPath = ""
	if music {
		q := u.Query()
		q.Set("param", "128y128")
		u.RawQuery = q.Encode()
	} else {
		u.Path = strings.SplitN(u.Path, "@", 2)[0] + "@128w_128h_1c.jpg"
		u.RawQuery = ""
	}
	return u.String(), nil
}

// These globally scoped special-use ranges are not public unicast destinations.
var nonPublicCoverRanges = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"), netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"), netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
}

// Control receives the resolved numeric address immediately before connect, including
// every fallback address. No second DNS lookup can replace the checked destination.
func publicCoverControl(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return err
	}
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.Zone() != "" {
		return errors.New("non-public cover address")
	}
	if ip.Is6() {
		// Only currently allocated global unicast, excluding transition and special-use ranges.
		if !netip.MustParsePrefix("2000::/3").Contains(ip) || netip.MustParsePrefix("2001::/23").Contains(ip) || netip.MustParsePrefix("2001:db8::/32").Contains(ip) || netip.MustParsePrefix("2002::/16").Contains(ip) || netip.MustParsePrefix("3fff::/20").Contains(ip) {
			return errors.New("non-public cover address")
		}
	}
	for _, prefix := range nonPublicCoverRanges {
		if prefix.Contains(ip) {
			return errors.New("non-public cover address")
		}
	}
	return nil
}

func noCoverRedirect(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

func newCoverClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// Environment proxies could resolve the hostname elsewhere and bypass Control.
	transport.Proxy = nil
	transport.DialContext = (&net.Dialer{Timeout: 5 * time.Second, Control: publicCoverControl}).DialContext
	return &http.Client{Timeout: 5 * time.Second, Transport: transport, CheckRedirect: noCoverRedirect}
}

func (api *PlatformAPI) cachedColor(target string) (coverPalette, bool) {
	api.colorsMu.Lock()
	defer api.colorsMu.Unlock()
	cached, ok := api.colors[target]
	return cached.value, ok && time.Now().Before(cached.expiresAt)
}

func (api *PlatformAPI) upstreamColor(ctx context.Context, target string) (coverPalette, error) {
	if value, ok := api.cachedColor(target); ok {
		return value, nil
	}
	result := api.colorFlights.DoChan(target, func() (any, error) {
		if value, ok := api.cachedColor(target); ok {
			return value, nil
		}
		value, err := api.fetchCoverColor(ctx, target, api.coverClient)
		if err != nil {
			return coverPalette{}, err
		}
		api.colorsMu.Lock()
		defer api.colorsMu.Unlock()
		// Evict expired entries first, then the oldest success at the fixed capacity.
		now := time.Now()
		oldestKey := ""
		var oldest time.Time
		for key, entry := range api.colors {
			if !now.Before(entry.expiresAt) {
				delete(api.colors, key)
				continue
			}
			if oldestKey == "" || entry.expiresAt.Before(oldest) {
				oldestKey, oldest = key, entry.expiresAt
			}
		}
		if len(api.colors) >= 256 {
			delete(api.colors, oldestKey)
		}
		api.colors[target] = cachedCoverColor{value, now.Add(10 * time.Minute)}
		return value, nil
	})
	select {
	case <-ctx.Done():
		return coverPalette{}, ctx.Err()
	case result := <-result:
		if result.Err != nil {
			return coverPalette{}, result.Err
		}
		return result.Val.(coverPalette), nil
	}
}

func (api *PlatformAPI) fetchCoverColor(ctx context.Context, target string, client *http.Client) (coverPalette, error) {
	select {
	case api.coverSlots <- struct{}{}:
		defer func() { <-api.coverSlots }()
	default:
		return coverPalette{}, &APIError{Status: http.StatusServiceUnavailable, Message: "Cover processing capacity reached"}
	}
	return api.readCoverColor(ctx, target, client)
}

// readCoverColor requires the caller to hold a coverSlots token through decoding.
func (api *PlatformAPI) readCoverColor(ctx context.Context, target string, client *http.Client) (coverPalette, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return coverPalette{}, err
	}
	request.Header.Set("User-Agent", "Mozilla/5.0 (compatible; MusicParty/1.0)")
	response, err := client.Do(request)
	if err != nil {
		return coverPalette{}, mapPlatformError(err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return coverPalette{}, BadGateway("Cover proxy request failed")
	}
	if !strings.HasPrefix(strings.ToLower(response.Header.Get("Content-Type")), "image/") {
		return coverPalette{}, BadGateway("Cover response is not an image")
	}
	encoded, err := io.ReadAll(io.LimitReader(response.Body, (3<<20)+1))
	if err != nil || len(encoded) > 3<<20 {
		return coverPalette{}, BadGateway("Cover image exceeds byte limit")
	}
	decoded, err := decodeBoundedCover(encoded)
	if err != nil {
		return coverPalette{}, BadGateway("Cover image could not be decoded")
	}
	return dominantCoverColor(decoded)
}

// Pick the most prominent saturated bucket, retaining original pixel averages
// within that bucket. Iterating the fixed array makes ties deterministic.
func dominantCoverColor(img image.Image) (coverPalette, error) {
	type bucket struct{ r, g, b, count, weight uint64 }
	var buckets [4096]bucket
	var total bucket
	bounds := img.Bounds()
	stepX, stepY := max(1, (bounds.Dx()+63)/64), max(1, (bounds.Dy()+63)/64)
	for y := bounds.Min.Y; y < bounds.Max.Y; y += stepY {
		for x := bounds.Min.X; x < bounds.Max.X; x += stepX {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			if c.A < 64 {
				continue
			}
			r, g, b := uint64(c.R), uint64(c.G), uint64(c.B)
			total.r += r
			total.g += g
			total.b += b
			total.count++
			high, low := max(r, g, b), min(r, g, b)
			if high < 32 || low > 230 || (high-low)*100 < high*20 {
				continue
			}
			key := (r>>4)<<8 | (g>>4)<<4 | b>>4
			v := &buckets[key]
			v.r += r
			v.g += g
			v.b += b
			v.count++
			v.weight += (high - low) * 255 / high
		}
	}
	if total.count == 0 {
		return coverPalette{}, BadGateway("Cover image has no visible pixels")
	}
	var selected bucket
	for _, v := range buckets {
		if v.weight > selected.weight {
			selected = v
		}
	}
	if selected.count == 0 {
		selected = total
	}
	r, g, b := selected.r/selected.count, selected.g/selected.count, selected.b/selected.count
	accent := fmt.Sprintf("#%02x%02x%02x", r, g, b)
	return coverPalette{accent, accent, fmt.Sprintf("rgba(%d, %d, %d, 0.16)", r, g, b), fmt.Sprintf("rgba(%d, %d, %d, 0.08)", r, g, b)}, nil
}
