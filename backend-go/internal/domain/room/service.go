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
	Private              bool
	Password             string
	KeepExistingPassword bool
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
	store    *storesqlite.Store
	accounts *account.Service
	now      func() time.Time
}

func New(store *storesqlite.Store, accounts *account.Service) *Service {
	return &Service{store: store, accounts: accounts, now: time.Now}
}
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
func (s *Service) Update(ctx context.Context, roomID, token, name string) (Info, error) {
	metadata, err := s.Access(ctx, roomID)
	if err != nil {
		return Info{}, err
	}
	return s.UpdateDetails(ctx, roomID, token, UpdateInput{Name: name, Private: metadata.Private, KeepExistingPassword: metadata.Private})
}

func (s *Service) UpdateDetails(ctx context.Context, roomID, token string, input UpdateInput) (Info, error) {
	if _, err := s.accounts.Resolve(ctx, token); err != nil {
		return Info{}, err
	}
	session, ok := s.CanManage(ctx, roomID, token)
	if !ok {
		return Info{}, ErrUpdateForbidden
	}
	name := strings.TrimSpace(input.Name)
	if name == "" || len([]rune(name)) > 64 {
		return Info{}, ErrInvalidRoomName
	}
	var existingVisibility, existingHash string
	var existingVersion int
	if err := s.store.Reader().QueryRowContext(ctx, "select visibility,coalesce(password_hash,''),password_version from room where id=? and deleted_at is null", roomID).Scan(&existingVisibility, &existingHash, &existingVersion); err != nil {
		return Info{}, err
	}
	visibility := "PUBLIC"
	passwordHash := ""
	passwordVersion := existingVersion
	if input.Private {
		visibility = "PRIVATE"
		if input.KeepExistingPassword && existingVisibility == "PRIVATE" && existingHash != "" {
			passwordHash = existingHash
		} else {
			if strings.TrimSpace(input.Password) == "" {
				return Info{}, ErrPrivatePasswordRequired
			}
			hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
			if err != nil {
				return Info{}, err
			}
			passwordHash = string(hash)
			passwordVersion++
		}
	} else if existingVisibility == "PRIVATE" {
		passwordVersion++
	}
	now := s.now().UnixMilli()
	if err := s.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, "update room set name=?,visibility=?,password_hash=?,password_version=?,last_active_at=? where id=? and deleted_at is null", name, visibility, nullable(passwordHash), passwordVersion, now, roomID)
		if err != nil {
			return err
		}
		count, _ := result.RowsAffected()
		if count == 0 {
			return sql.ErrNoRows
		}
		return nil
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
	if _, err := s.accounts.Resolve(ctx, token); err != nil {
		return false, err
	}
	_, ok := s.CanManage(ctx, roomID, token)
	if !ok {
		return false, nil
	}
	var system bool
	if err := s.store.Reader().QueryRowContext(ctx, "select system from room where id=? and deleted_at is null", roomID).Scan(&system); err != nil {
		return false, nil
	}
	if system {
		return false, nil
	}
	now := s.now().UnixMilli()
	err := s.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "update user_profile set current_room_id='lounge',last_seen_at=? where current_room_id=?", now, roomID); err != nil {
			return err
		}
		tables := []string{"room_queue", "room_history", "room_history_track", "room_playback_state", "chat_message", "room_subsonic_source", "room_invite", "room_membership"}
		for _, table := range tables {
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
		result, err := tx.ExecContext(ctx, "update room set deleted_at=? where id=? and deleted_at is null", now, roomID)
		if err != nil {
			return err
		}
		count, _ := result.RowsAffected()
		if count != 1 {
			return sql.ErrNoRows
		}
		return nil
	})
	return err == nil, err
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
	role := "MEMBER"
	if owner {
		role = "OWNER"
	}
	return s.changeMembership(ctx, roomID, publicID, role, !owner)
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
			_, err := tx.ExecContext(ctx, "delete from room_membership where room_id=? and public_id=?", roomID, publicID)
			changed = err == nil
			return err
		}
		_, err := tx.ExecContext(ctx, "update room_membership set role=?,updated_at=? where room_id=? and public_id=?", role, s.now().UnixMilli(), roomID, publicID)
		changed = err == nil
		return err
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
