package sqlite

type Room struct {
	ID              string
	Name            string
	OwnerPublicID   string
	Visibility      string
	PasswordHash    *string
	PasswordVersion int
	System          bool
	CreatedAt       int64
	LastActiveAt    int64
	DeletedAt       *int64
}

type UserProfile struct {
	PublicID      string
	DisplayName   string
	Guest         bool
	CurrentRoomID string
	CreatedAt     int64
	LastSeenAt    int64
}

type Session struct {
	SessionTokenHash string
	PublicID         string
	CreatedAt        int64
	LastSeenAt       int64
}

type UserAccount struct {
	Username     string
	PublicID     string
	PasswordHash string
	Role         string
	Enabled      bool
	CreatedAt    int64
	UpdatedAt    int64
	LastLoginAt  *int64
}

type RoomMembership struct {
	RoomID    string
	PublicID  string
	Role      string
	CreatedAt int64
	UpdatedAt int64
}

func (membership RoomMembership) Owner() bool {
	return membership.Role == "OWNER"
}

type RoomInvite struct {
	ID                string
	RoomID            string
	CreatedByPublicID string
	SecretHash        string
	Label             *string
	ExpiresAt         int64
	MaxUses           int
	UsedAt            *int64
	UsedByPublicID    *string
	RevokedAt         *int64
	CreatedAt         int64
}

func (invite RoomInvite) Active(now int64) bool {
	return invite.RevokedAt == nil && invite.UsedAt == nil && invite.ExpiresAt >= now
}

type Music struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Artists  []string `json:"artists"`
	Duration int64    `json:"duration"`
	Platform string   `json:"platform"`
	CoverURL string   `json:"coverUrl"`
}

type PlayableMusic struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Artists    []string `json:"artists"`
	Duration   int64    `json:"duration"`
	Platform   string   `json:"platform"`
	URL        string   `json:"url"`
	CoverURL   string   `json:"coverUrl"`
	NeedsProxy bool     `json:"needsProxy"`
}

type UserSummary struct {
	PublicID string `json:"publicId"`
	Name     string `json:"name"`
	Guest    bool   `json:"isGuest"`
}

type QueueItem struct {
	QueueID    string      `json:"queueId"`
	Music      Music       `json:"music"`
	EnqueuedBy UserSummary `json:"enqueuedBy"`
	Status     string      `json:"status"`
}

type HistoryEntry struct {
	ID               string
	RoomID           string
	Music            Music
	EnqueuerPublicID *string
	PlayedAt         int64
}

type PlaylistTrack struct {
	ID         string
	PlaylistID string
	Music      Music
	SortOrder  int
	CreatedAt  int64
}

type PlaybackState struct {
	RoomID              string
	CurrentMusic        *PlayableMusic
	CurrentEnqueuerID   *string
	CurrentEnqueuerName *string
	PositionAnchor      int64
	TimestampAnchor     int64
	PositionUpdatedAt   int64
	Shuffle             bool
	Paused              bool
	PauseLocked         bool
	SkipLocked          bool
	ShuffleLocked       bool
	Loading             bool
	LikedUserIDs        map[string]struct{}
	LikeMarkers         []int64
	PlayEpoch           int64
	StateVersion        int64
	LastPersistedAt     int64
}

type ChatMessage struct {
	ID        string `json:"id"`
	UserID    string `json:"userId"`
	UserName  string `json:"userName"`
	Content   string `json:"content"`
	Timestamp int64  `json:"timestamp"`
	Type      string `json:"type"`
}

type Playlist struct {
	ID         string
	OwnerID    string
	Name       string
	SystemKey  *string
	TrackCount int
	CreatedAt  int64
	UpdatedAt  int64
}

type SiteSetting struct {
	Key       string
	Value     *string
	Secret    bool
	UpdatedAt int64
}

type SubsonicSource struct {
	ID           string
	OwnerRoomID  *string
	Label        string
	BaseURL      string
	Username     string
	Password     string
	Client       string
	APIVersion   string
	AllowedUsers *string
	Enabled      bool
	System       bool
	CreatedAt    int64
	UpdatedAt    int64
}

type RoomSubsonicSource struct {
	RoomID       string
	SourceID     string
	Enabled      bool
	DisplayLabel *string
	AllowedUsers *string
	SortOrder    int
	CreatedAt    int64
	UpdatedAt    int64
}

type LocalTrack struct {
	ID               string   `json:"id"`
	OriginalHash     *string  `json:"originalHash"`
	OriginalFileName *string  `json:"originalFileName"`
	SourcePath       *string  `json:"sourcePath"`
	SourceMIMEType   *string  `json:"sourceMimeType"`
	SourceSizeBytes  int64    `json:"sourceSizeBytes"`
	Title            string   `json:"title"`
	Artists          []string `json:"artists"`
	Album            *string  `json:"album"`
	DurationMS       int64    `json:"durationMs"`
	CoverPath        *string  `json:"coverPath"`
	CoverMIMEType    *string  `json:"coverMimeType"`
	OGGPath          *string  `json:"oggPath"`
	Status           string   `json:"status"`
	ErrorMessage     *string  `json:"errorMessage"`
	StatusMessage    *string  `json:"statusMessage"`
	ProgressPercent  *int     `json:"progressPercent"`
	UploadedBy       *string  `json:"uploadedBy"`
	CreatedAt        int64    `json:"createdAt"`
	UpdatedAt        int64    `json:"updatedAt"`
	StartedAt        *int64   `json:"startedAt"`
	CompletedAt      *int64   `json:"completedAt"`
}

type LocalTrackStatusUpdate struct {
	Status          string
	OGGPath         *string
	ErrorMessage    *string
	StatusMessage   *string
	ProgressPercent *int
	StartedAt       *int64
	CompletedAt     *int64
	UpdatedAt       int64
}

func nullableString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}
