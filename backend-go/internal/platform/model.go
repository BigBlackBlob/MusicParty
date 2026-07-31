package platform

import "context"

type Music struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Artists  []string `json:"artists"`
	Duration int64    `json:"duration"`
	Platform string   `json:"platform"`
	CoverURL string   `json:"coverUrl"`
}

// PlayableMusic is the canonical queue payload resolved from a platform ID.
// URL always points at a MusicParty proxy rather than an upstream credentialed URL.
type PlayableMusic struct {
	Music
	URL string
}

type PlayableResolver interface {
	ResolvePlayable(context.Context, string) (PlayableMusic, error)
}

type Playlist struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	CoverImgURL string `json:"coverImgUrl"`
	TrackCount  int    `json:"trackCount"`
	Platform    string `json:"platform"`
}

type Album struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	ArtistName string `json:"artistName"`
	CoverURL   string `json:"coverUrl"`
	TrackCount int    `json:"trackCount"`
	Platform   string `json:"platform"`
}

type User struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatarUrl"`
	Platform  string `json:"platform"`
}

type Lyric struct {
	Lyric           string `json:"lyric"`
	TranslatedLyric string `json:"translatedLyric"`
	RomanizedLyric  string `json:"romanizedLyric"`
}

type Service interface {
	Name() string
	Available() bool
	Search(context.Context, string, int, int) ([]Music, error)
	UserPlaylists(context.Context, string) ([]Playlist, error)
	PlaylistSongs(context.Context, string, int, int) ([]Music, error)
	SearchAlbums(context.Context, string) ([]Album, error)
	AlbumSongs(context.Context, string) ([]Music, error)
	SearchUsers(context.Context, string) ([]User, error)
	Lyric(context.Context, string) (Lyric, error)
}

// MediaSource exposes authenticated upstream URLs to the streaming proxy.
type MediaSource interface {
	StreamURL(context.Context, string) (string, error)
	CoverURL(context.Context, string) (string, error)
}

type Unsupported struct{ Platform string }

func (u Unsupported) Name() string                                            { return u.Platform }
func (Unsupported) Available() bool                                           { return true }
func (Unsupported) Search(context.Context, string, int, int) ([]Music, error) { return []Music{}, nil }
func (Unsupported) UserPlaylists(context.Context, string) ([]Playlist, error) {
	return []Playlist{}, nil
}
func (Unsupported) PlaylistSongs(context.Context, string, int, int) ([]Music, error) {
	return []Music{}, nil
}
func (Unsupported) SearchAlbums(context.Context, string) ([]Album, error) { return []Album{}, nil }
func (Unsupported) AlbumSongs(context.Context, string) ([]Music, error)   { return []Music{}, nil }
func (Unsupported) SearchUsers(context.Context, string) ([]User, error)   { return []User{}, nil }
func (Unsupported) Lyric(context.Context, string) (Lyric, error)          { return Lyric{}, nil }
