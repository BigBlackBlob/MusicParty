package room

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const PermanentInviteExpiresAt int64 = 253402300799000

var (
	//lint:ignore ST1005 Java-compatible API message.
	ErrUpdateForbidden = errors.New("No permission to update room")
	//lint:ignore ST1005 Java-compatible API message.
	ErrInvalidRoomName = errors.New("Room name must be 1-64 characters")
	//lint:ignore ST1005 Java-compatible API message.
	ErrLastOwner = errors.New("A room must retain an owner")
	//lint:ignore ST1005 Java-compatible API message.
	ErrPrivatePasswordRequired = errors.New("Private room password cannot be empty")
	//lint:ignore ST1005 Java-compatible API message.
	ErrInvalidRoomPassword = errors.New("Invalid room password")
	ErrForbidden           = errors.New("Forbidden")
	//lint:ignore ST1005 Java-compatible API message.
	ErrRoomNameExists = errors.New("Room name already exists")
)

type Info struct {
	RoomID          string `json:"roomId"`
	Name            string `json:"name"`
	CreatorPublicID string `json:"creatorPublicId"`
	CreatedAt       int64  `json:"createdAt"`
	PrivateRoom     bool   `json:"privateRoom"`
	System          bool   `json:"system"`
	OnlineCount     int    `json:"onlineCount"`
	AccessGranted   bool   `json:"accessGranted"`
}

type UpdateInput struct {
	Name                 string
	Private              *bool
	Password             string
	KeepExistingPassword bool
}

type CreateInput struct {
	Name     string
	Private  bool
	Password string
}

type AccessMetadata struct {
	Private         bool
	PasswordHash    string
	PasswordVersion int
}
type Membership struct {
	RoomID    string `json:"roomId"`
	PublicID  string `json:"publicId"`
	Role      string `json:"role"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}
type Invite struct {
	ID        string `json:"id"`
	Secret    string `json:"secret,omitempty"`
	RoomID    string `json:"-"`
	Label     string `json:"label"`
	ExpiresAt int64  `json:"expiresAt"`
	Permanent bool   `json:"permanent"`
	UsedAt    *int64 `json:"usedAt,omitempty"`
	RevokedAt *int64 `json:"revokedAt,omitempty"`
	CreatedAt int64  `json:"-"`
}
type Service struct {
	store         *storesqlite.Store
	accounts      *account.Service
	now           func() time.Time
	onRoomDeleted func(string)
}

func New(store *storesqlite.Store, accounts *account.Service) *Service {
	return &Service{store: store, accounts: accounts, now: time.Now}
}

// SetRoomDeletedHook wires runtime and connection cleanup after the deletion commits.
// Configure it at startup before serving room mutations.
func (s *Service) SetRoomDeletedHook(hook func(string)) { s.onRoomDeleted = hook }

func (s *Service) List(ctx context.Context, token string) ([]Info, error) {
	_ = token
	rows, err := s.store.Reader().QueryContext(ctx, `select id,name,owner_public_id,visibility,system,created_at from room where deleted_at is null order by system desc,last_active_at desc,created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Info{}
	for rows.Next() {
		var value Info
		var visibility string
		if err := rows.Scan(&value.RoomID, &value.Name, &value.CreatorPublicID, &visibility, &value.System, &value.CreatedAt); err != nil {
			return nil, err
		}
		value.PrivateRoom = visibility == "PRIVATE"
		result = append(result, value)
	}
	return result, rows.Err()
}
func (s *Service) CanManage(ctx context.Context, roomID, token string) (account.Session, bool) {
	session, err := s.accounts.Resolve(ctx, token)
	if err != nil {
		return account.Session{}, false
	}
	return session, session.Admin()
}

// CanEdit reports who may rename or delete a room: platform admins, the room owner, or a member
// holding the OWNER role. room.owner_public_id stays the authority (SetOwner keeps it in step with
// room_membership) and the OWNER membership is a fallback for rows written before that linkage.
// Member, owner and invite administration deliberately keep requiring an admin, so an owner can
// clean up their own room without gaining power over anyone else's.
func (s *Service) CanEdit(ctx context.Context, roomID, token string) (account.Session, bool) {
	session, err := s.accounts.Resolve(ctx, token)
	if err != nil {
		return account.Session{}, false
	}
	if session.Admin() {
		return session, true
	}
	var owner string
	var system bool
	if err := s.store.Reader().QueryRowContext(ctx, "select owner_public_id,system from room where id=? and deleted_at is null", roomID).Scan(&owner, &system); err != nil {
		return account.Session{}, false
	}
	if system {
		return session, false
	}
	if owner != "" && owner == session.PublicID {
		return session, true
	}
	var role string
	if err := s.store.Reader().QueryRowContext(ctx, "select role from room_membership where room_id=? and public_id=?", roomID, session.PublicID).Scan(&role); err != nil {
		return session, false
	}
	return session, role == "OWNER"
}

func (s *Service) Create(ctx context.Context, token string, input CreateInput) (Info, error) {
	session, err := s.accounts.Resolve(ctx, token)
	if err != nil {
		return Info{}, err
	}
	name, err := normalizeRoomName(input.Name)
	if err != nil {
		return Info{}, err
	}
	private := input.Private
	if private && strings.TrimSpace(input.Password) == "" {
		return Info{}, ErrPrivatePasswordRequired
	}
	roomID := "room-" + uuid.NewString()[:8]
	visibility := "PUBLIC"
	passwordVersion := 0
	var passwordHash any
	if private {
		visibility = "PRIVATE"
		hash, hashErr := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
		if hashErr != nil {
			return Info{}, hashErr
		}
		passwordHash = string(hash)
		passwordVersion = 1
	}
	now := s.now().UnixMilli()
	if err := s.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		if err := ensureRoomNameAvailable(ctx, tx, name, ""); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "insert into room(id,name,owner_public_id,visibility,password_hash,password_version,system,created_at,last_active_at) values(?,?,?,?,?,?,0,?,?)", roomID, name, session.PublicID, visibility, passwordHash, passwordVersion, now, now); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, "insert into room_membership(room_id,public_id,role,created_at,updated_at) values(?,?,'OWNER',?,?)", roomID, session.PublicID, now, now)
		return err
	}); err != nil {
		return Info{}, err
	}
	return Info{RoomID: roomID, Name: name, CreatorPublicID: session.PublicID, CreatedAt: now, PrivateRoom: private, AccessGranted: true}, nil
}

// Update renames a room without changing its visibility or password.
func (s *Service) Update(ctx context.Context, roomID, token, name string) (Info, error) {
	return s.UpdateDetails(ctx, roomID, token, UpdateInput{Name: name})
}

func normalizeRoomName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" || len([]rune(name)) > 64 {
		return "", ErrInvalidRoomName
	}
	return name, nil
}

// Check names in the same writer transaction as creation or rename. Existing duplicate
// rows are left intact; new names follow the existing SQLite case-insensitive rule.
func ensureRoomNameAvailable(ctx context.Context, tx *sql.Tx, name, exceptRoomID string) error {
	var exists bool
	if err := tx.QueryRowContext(ctx, "select exists(select 1 from room where lower(name)=lower(?) and deleted_at is null and id<>?)", name, exceptRoomID).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return ErrRoomNameExists
	}
	return nil
}

func (s *Service) UpdateDetails(ctx context.Context, roomID, token string, input UpdateInput) (Info, error) {
	if _, err := s.accounts.Resolve(ctx, token); err != nil {
		return Info{}, err
	}
	session, ok := s.CanEdit(ctx, roomID, token)
	if !ok {
		return Info{}, ErrUpdateForbidden
	}
	name, err := normalizeRoomName(input.Name)
	if err != nil {
		return Info{}, err
	}
	// Hash outside the single-writer transaction; decide which password to keep inside it.
	newHash := ""
	if (input.Private == nil || *input.Private) && strings.TrimSpace(input.Password) != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
		if err != nil {
			return Info{}, err
		}
		newHash = string(hash)
	}
	now := s.now().UnixMilli()
	if err := s.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		allowed, err := canEditRoomTx(ctx, tx, roomID, session)
		if err != nil {
			return err
		}
		if !allowed {
			return ErrUpdateForbidden
		}
		var existingName, visibility, passwordHash string
		var passwordVersion int
		if err := tx.QueryRowContext(ctx, "select name,visibility,coalesce(password_hash,''),password_version from room where id=? and deleted_at is null", roomID).Scan(&existingName, &visibility, &passwordHash, &passwordVersion); err != nil {
			return err
		}
		if name != existingName {
			if err := ensureRoomNameAvailable(ctx, tx, name, roomID); err != nil {
				return err
			}
		}
		private := visibility == "PRIVATE"
		if input.Private != nil {
			private = *input.Private
		}
		if private {
			keep := input.KeepExistingPassword || (input.Private == nil && input.Password == "")
			if !(keep && visibility == "PRIVATE" && passwordHash != "") {
				if newHash == "" {
					return ErrPrivatePasswordRequired
				}
				passwordHash = newHash
				passwordVersion++
			}
			visibility = "PRIVATE"
		} else {
			if visibility == "PRIVATE" {
				passwordVersion++
			}
			visibility, passwordHash = "PUBLIC", ""
		}
		_, err = tx.ExecContext(ctx, "update room set name=?,visibility=?,password_hash=?,password_version=?,last_active_at=? where id=? and deleted_at is null", name, visibility, nullable(passwordHash), passwordVersion, now, roomID)
		return err
	}); err != nil {
		return Info{}, err
	}
	return s.get(ctx, roomID, session.PublicID)
}

func (s *Service) Access(ctx context.Context, roomID string) (AccessMetadata, error) {
	var visibility string
	var metadata AccessMetadata
	err := s.store.Reader().QueryRowContext(ctx, "select visibility,coalesce(password_hash,''),password_version from room where id=? and deleted_at is null", roomID).Scan(&visibility, &metadata.PasswordHash, &metadata.PasswordVersion)
	metadata.Private = visibility == "PRIVATE"
	return metadata, err
}

func (s *Service) VerifyAccess(ctx context.Context, roomID, token, password string) (AccessMetadata, account.Session, error) {
	session, err := s.accounts.Resolve(ctx, token)
	if err != nil {
		return AccessMetadata{}, account.Session{}, err
	}
	metadata, err := s.Access(ctx, roomID)
	if err != nil {
		return AccessMetadata{}, account.Session{}, err
	}
	if !metadata.Private {
		return metadata, session, nil
	}
	if metadata.PasswordHash == "" || bcrypt.CompareHashAndPassword([]byte(metadata.PasswordHash), []byte(password)) != nil {
		return AccessMetadata{}, account.Session{}, ErrInvalidRoomPassword
	}
	return metadata, session, nil
}
func (s *Service) Delete(ctx context.Context, roomID, token string) (bool, error) {
	session, err := s.accounts.Resolve(ctx, token)
	if err != nil {
		return false, err
	}
	deleted := false
	now := s.now().UnixMilli()
	// Once accepted, deletion and its post-commit cleanup complete even if the HTTP/WS
	// request ends. Otherwise cancellation can hide a committed write from the caller.
	ctx = context.WithoutCancel(ctx)
	err = s.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		allowed, err := canEditRoomTx(ctx, tx, roomID, session)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil || !allowed {
			return err
		}
		var system bool
		if err := tx.QueryRowContext(ctx, "select system from room where id=? and deleted_at is null", roomID).Scan(&system); err != nil {
			return err
		}
		if system {
			return nil
		}
		if _, err := tx.ExecContext(ctx, "update room set deleted_at=? where id=? and deleted_at is null", now, roomID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "update user_profile set current_room_id='lounge',last_seen_at=? where current_room_id=?", now, roomID); err != nil {
			return err
		}
		for _, table := range []string{"room_queue", "room_history", "room_history_track", "room_playback_state", "chat_message", "room_subsonic_source", "room_invite", "room_membership"} {
			if _, err := tx.ExecContext(ctx, "delete from "+table+" where room_id=?", roomID); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, "delete from room_playlist_track where playlist_id in (select id from room_playlist where room_id=?)", roomID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "delete from room_playlist where room_id=?", roomID); err != nil {
			return err
		}
		deleted = true
		return nil
	})
	if err == nil && deleted && s.onRoomDeleted != nil {
		s.onRoomDeleted(roomID)
	}
	return deleted && err == nil, err
}

// Recheck ownership with the room mutation so a queued write cannot use a revoked owner role.
func canEditRoomTx(ctx context.Context, tx *sql.Tx, roomID string, session account.Session) (bool, error) {
	var owner string
	var system bool
	if err := tx.QueryRowContext(ctx, "select owner_public_id,system from room where id=? and deleted_at is null", roomID).Scan(&owner, &system); err != nil {
		return false, err
	}
	if session.Admin() {
		return true, nil
	}
	if system {
		return false, nil
	}
	if owner != "" && owner == session.PublicID {
		return true, nil
	}
	var allowed bool
	err := tx.QueryRowContext(ctx, "select exists(select 1 from room_membership where room_id=? and public_id=? and role='OWNER')", roomID, session.PublicID).Scan(&allowed)
	return allowed, err
}

func (s *Service) CreateInvite(ctx context.Context, roomID, token, label string) (Invite, error) {
	session, ok := s.CanManage(ctx, roomID, token)
	if !ok {
		return Invite{}, ErrForbidden
	}
	label = strings.TrimSpace(label)
	if len([]rune(label)) > 64 {
		label = string([]rune(label)[:64])
	}
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return Invite{}, err
	}
	secret := base64.RawURLEncoding.EncodeToString(bytes)
	now := s.now().UnixMilli()
	value := Invite{ID: "inv_" + strings.ReplaceAll(uuid.NewString(), "-", ""), Secret: secret, RoomID: roomID, Label: label, ExpiresAt: PermanentInviteExpiresAt, Permanent: true, CreatedAt: now}
	err := s.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		if err := storesqlite.RequireActiveRoom(ctx, tx, roomID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, "insert into room_invite(id,room_id,created_by_public_id,secret_hash,label,expires_at,max_uses,created_at) values(?,?,?,?,?,?,1,?)", value.ID, roomID, session.PublicID, hash(secret), nullable(label), value.ExpiresAt, now)
		return err
	})
	return value, err
}
func (s *Service) ListInvites(ctx context.Context, roomID, token string) ([]Invite, error) {
	if _, ok := s.CanManage(ctx, roomID, token); !ok {
		return nil, ErrForbidden
	}
	rows, err := s.store.Reader().QueryContext(ctx, "select id,label,expires_at,used_at,revoked_at,created_at from room_invite where room_id=? order by created_at desc", roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Invite{}
	for rows.Next() {
		var value Invite
		var label sql.NullString
		var used, revoked sql.NullInt64
		if err := rows.Scan(&value.ID, &label, &value.ExpiresAt, &used, &revoked, &value.CreatedAt); err != nil {
			return nil, err
		}
		value.RoomID = roomID
		if label.Valid {
			value.Label = label.String
		}
		if used.Valid {
			value.UsedAt = &used.Int64
		}
		if revoked.Valid {
			value.RevokedAt = &revoked.Int64
		}
		value.Permanent = value.ExpiresAt == PermanentInviteExpiresAt
		result = append(result, value)
	}
	return result, rows.Err()
}
func (s *Service) RevokeInvite(ctx context.Context, roomID, inviteID, token string) (bool, error) {
	if _, ok := s.CanManage(ctx, roomID, token); !ok {
		return false, ErrForbidden
	}
	updated := false
	err := s.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, "update room_invite set revoked_at=? where room_id=? and id=? and revoked_at is null", s.now().UnixMilli(), roomID, inviteID)
		if err == nil {
			count, _ := result.RowsAffected()
			updated = count == 1
		}
		return err
	})
	return updated, err
}
func (s *Service) Members(ctx context.Context, roomID, token string) ([]Membership, error) {
	if _, ok := s.CanManage(ctx, roomID, token); !ok {
		return nil, ErrForbidden
	}
	rows, err := s.store.Reader().QueryContext(ctx, "select room_id,public_id,role,created_at,updated_at from room_membership where room_id=? order by created_at", roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Membership{}
	for rows.Next() {
		var value Membership
		if err := rows.Scan(&value.RoomID, &value.PublicID, &value.Role, &value.CreatedAt, &value.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}
func (s *Service) RemoveMember(ctx context.Context, roomID, publicID, token string) (bool, error) {
	if _, ok := s.CanManage(ctx, roomID, token); !ok {
		return false, ErrForbidden
	}
	return s.changeMembership(ctx, roomID, publicID, "DELETE", false)
}
func (s *Service) SetOwner(ctx context.Context, roomID, publicID, token string, owner bool) (bool, error) {
	session, err := s.accounts.Resolve(ctx, token)
	if err != nil || !session.Admin() {
		return false, ErrForbidden
	}
	if !owner {
		return s.changeMembership(ctx, roomID, publicID, "MEMBER", true)
	}
	return s.grantOwner(ctx, roomID, publicID)
}

// grantOwner promotes publicID to OWNER and, in the same write transaction, moves
// room.owner_public_id to them while the previous owner steps down to MEMBER. CanEdit reads that
// pointer, so a promotion or a handover takes effect immediately without a second write path.
func (s *Service) grantOwner(ctx context.Context, roomID, publicID string) (bool, error) {
	changed := false
	err := s.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		var existing string
		if err := tx.QueryRowContext(ctx, "select role from room_membership where room_id=? and public_id=?", roomID, publicID).Scan(&existing); err != nil {
			// A plain sql.ErrNoRows maps to 404 at the HTTP layer; anything else stays an error.
			return err
		}
		var previous string
		if err := tx.QueryRowContext(ctx, "select owner_public_id from room where id=? and deleted_at is null", roomID).Scan(&previous); err != nil {
			return err
		}
		now := s.now().UnixMilli()
		if _, err := tx.ExecContext(ctx, "update room_membership set role='OWNER',updated_at=? where room_id=? and public_id=?", now, roomID, publicID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "update room set owner_public_id=? where id=? and deleted_at is null", publicID, roomID); err != nil {
			return err
		}
		if previous != "" && previous != publicID {
			// The new owner is already recorded, so demoting the previous one never leaves the room
			// without an owner.
			if _, err := tx.ExecContext(ctx, "update room_membership set role='MEMBER',updated_at=? where room_id=? and public_id=? and role='OWNER'", now, roomID, previous); err != nil {
				return err
			}
		}
		changed = true
		return nil
	})
	return changed, err
}

// syncOwnerPointer re-points room.owner_public_id at a remaining OWNER (or clears it) once the
// member it names stops being one, keeping CanEdit and room_membership in agreement.
func (s *Service) syncOwnerPointer(ctx context.Context, tx *sql.Tx, roomID, publicID string) error {
	var owner string
	if err := tx.QueryRowContext(ctx, "select owner_public_id from room where id=? and deleted_at is null", roomID).Scan(&owner); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	if owner == "" || owner != publicID {
		return nil
	}
	var next string
	if err := tx.QueryRowContext(ctx, "select public_id from room_membership where room_id=? and role='OWNER' order by created_at,public_id limit 1", roomID).Scan(&next); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		next = ""
	}
	_, err := tx.ExecContext(ctx, "update room set owner_public_id=? where id=? and deleted_at is null", next, roomID)
	return err
}
func (s *Service) changeMembership(ctx context.Context, roomID, publicID, role string, protectLastOwner bool) (bool, error) {
	changed := false
	err := s.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		var existing string
		if err := tx.QueryRowContext(ctx, "select role from room_membership where room_id=? and public_id=?", roomID, publicID).Scan(&existing); err != nil {
			return sql.ErrNoRows
		}
		if existing == "OWNER" && (role == "DELETE" || protectLastOwner) {
			var owners int
			if err := tx.QueryRowContext(ctx, "select count(*) from room_membership where room_id=? and role='OWNER'", roomID).Scan(&owners); err != nil {
				return err
			}
			if owners <= 1 {
				return ErrLastOwner
			}
		}
		if role == "DELETE" {
			if _, err := tx.ExecContext(ctx, "delete from room_membership where room_id=? and public_id=?", roomID, publicID); err != nil {
				return err
			}
			changed = true
			return s.syncOwnerPointer(ctx, tx, roomID, publicID)
		}
		if _, err := tx.ExecContext(ctx, "update room_membership set role=?,updated_at=? where room_id=? and public_id=?", role, s.now().UnixMilli(), roomID, publicID); err != nil {
			return err
		}
		changed = true
		return s.syncOwnerPointer(ctx, tx, roomID, publicID)
	})
	return changed, err
}
func (s *Service) get(ctx context.Context, roomID, _ string) (Info, error) {
	var value Info
	var visibility string
	err := s.store.Reader().QueryRowContext(ctx, "select id,name,owner_public_id,visibility,system,created_at from room where id=? and deleted_at is null", roomID).Scan(&value.RoomID, &value.Name, &value.CreatorPublicID, &visibility, &value.System, &value.CreatedAt)
	value.PrivateRoom = visibility == "PRIVATE"
	return value, err
}
func hash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}
