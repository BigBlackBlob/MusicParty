package account

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/security"
	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	"github.com/google/uuid"
)

var (
	//lint:ignore ST1005 Java-compatible API message.
	ErrUnknownSession = errors.New("Unknown session token")
	//lint:ignore ST1005 Java-compatible API message.
	ErrInvalidLogin = errors.New("Invalid username or password")
	//lint:ignore ST1005 Java-compatible API message.
	errBootstrapBusy = errors.New("Administrator bootstrap is being completed by another instance")
	//lint:ignore ST1005 Java-compatible API message.
	errMemberPassword = errors.New("Member passwords are not supported")
	//lint:ignore ST1005 Java-compatible API message.
	errCurrentPassword = errors.New("Current password is incorrect")
	//lint:ignore ST1005 Java-compatible API message.
	errInvalidInvite = errors.New("Invitation is invalid or expired")
	usernamePattern  = regexp.MustCompile(`^[\p{L}\p{N}_.-]+$`)
)

type Session struct {
	SessionToken string `json:"sessionToken,omitempty"`
	PublicID     string `json:"publicId"`
	Username     string `json:"username"`
	DisplayName  string `json:"displayName"`
	Role         string `json:"role"`
	Guest        bool   `json:"guest"`
	Enabled      bool   `json:"enabled"`
	LastLoginAt  *int64 `json:"lastLoginAt"`
}
type InviteMetadata struct {
	RoomID    string `json:"roomId"`
	RoomName  string `json:"roomName"`
	ExpiresAt int64  `json:"expiresAt"`
	Valid     bool   `json:"valid"`
}

func (s Session) Admin() bool { return s.Role == "PLATFORM_ADMIN" || s.Role == "ADMIN" }

type Service struct {
	store *storesqlite.Store
	now   func() time.Time
}

func New(store *storesqlite.Store) *Service { return &Service{store: store, now: time.Now} }
func (s *Service) Status(ctx context.Context) (map[string]bool, error) {
	var count int
	err := s.store.Reader().QueryRowContext(ctx, "select count(1) from user_account where role in ('PLATFORM_ADMIN','ADMIN') and enabled = 1").Scan(&count)
	return map[string]bool{"requiresSetup": count == 0}, err
}
func (s *Service) Bootstrap(ctx context.Context, username, password string) error {
	username, err := normalizeUsername(username)
	if err != nil {
		return err
	}
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	passwordHash, err := security.HashPassword(password)
	if err != nil {
		return err
	}
	now := s.now().UnixMilli()
	return s.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		var admins int
		if err := tx.QueryRowContext(ctx, "select count(1) from user_account where role in ('PLATFORM_ADMIN','ADMIN') and enabled=1").Scan(&admins); err != nil || admins > 0 {
			return err
		}
		result, err := tx.ExecContext(ctx, "insert or ignore into admin_bootstrap_claim(claim_key,claimed_at) values('initial-admin',?)", now)
		if err != nil {
			return err
		}
		claimed, _ := result.RowsAffected()
		if claimed != 1 {
			return errBootstrapBusy
		}
		var exists int
		if err := tx.QueryRowContext(ctx, "select count(1) from user_account where username=?", username).Scan(&exists); err != nil {
			return err
		}
		if exists > 0 {
			return errors.New("Bootstrap administrator username already exists")
		}
		publicID := "u_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
		if _, err = tx.ExecContext(ctx, "insert into user_profile(public_id,display_name,is_guest,current_room_id,created_at,last_seen_at) values(?,?,0,'lounge',?,?)", publicID, username, now, now); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, "insert into user_account(username,public_id,password_hash,role,enabled,created_at,updated_at,last_login_at) values(?,?,?,'PLATFORM_ADMIN',1,?,?,?)", username, publicID, passwordHash, now, now, now)
		return err
	})
}
func (s *Service) Resolve(ctx context.Context, token string) (Session, error) {
	if strings.TrimSpace(token) == "" {
		return Session{}, ErrUnknownSession
	}
	hash := tokenHash(token)

	// Try to resolve as registered user (with user_account)
	row := s.store.Reader().QueryRowContext(ctx, `select a.public_id,a.username,p.display_name,a.role,a.enabled,a.last_login_at,p.is_guest from user_session ss join user_account a on a.public_id=ss.public_id join user_profile p on p.public_id=a.public_id where ss.session_token_hash=? and a.enabled=1`, hash)
	var result Session
	var last sql.NullInt64
	var isGuest int
	err := row.Scan(&result.PublicID, &result.Username, &result.DisplayName, &result.Role, &result.Enabled, &last, &isGuest)
	if err == nil {
		result.SessionToken = token
		result.Guest = isGuest == 1
		if last.Valid {
			result.LastLoginAt = &last.Int64
		}
		return result, nil
	}

	// Try to resolve as guest (no user_account)
	row = s.store.Reader().QueryRowContext(ctx, `select p.public_id,p.display_name,p.is_guest from user_session ss join user_profile p on p.public_id=ss.public_id where ss.session_token_hash=?`, hash)
	err = row.Scan(&result.PublicID, &result.DisplayName, &isGuest)
	if err != nil {
		return Session{}, ErrUnknownSession
	}
	result.SessionToken = token
	result.Username = ""
	result.Role = "GUEST"
	result.Guest = true
	result.Enabled = true
	return result, nil
}
func (s *Service) Login(ctx context.Context, username, password string) (Session, error) {
	username, err := normalizeUsername(username)
	if err != nil {
		return Session{}, ErrInvalidLogin
	}
	var account storesqlite.UserAccount
	var last sql.NullInt64
	err = s.store.Reader().QueryRowContext(ctx, `select username,public_id,password_hash,role,enabled,created_at,updated_at,last_login_at from user_account where username=?`, username).Scan(&account.Username, &account.PublicID, &account.PasswordHash, &account.Role, &account.Enabled, &account.CreatedAt, &account.UpdatedAt, &last)
	if err != nil || !account.Enabled || !security.CheckPassword(account.PasswordHash, password) {
		return Session{}, ErrInvalidLogin
	}
	now := s.now().UnixMilli()
	token := uuid.NewString()
	hash := tokenHash(token)
	var display string
	err = s.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "update user_account set last_login_at=?,updated_at=? where username=?", now, now, username); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "update user_profile set is_guest=0,last_seen_at=? where public_id=?", now, account.PublicID); err != nil {
			return err
		}
		if err := tx.QueryRowContext(ctx, "select display_name from user_profile where public_id=?", account.PublicID).Scan(&display); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `insert into user_session(session_token_hash,public_id,created_at,last_seen_at) values(?,?,?,?) on conflict(session_token_hash) do update set public_id=excluded.public_id,last_seen_at=excluded.last_seen_at`, hash, account.PublicID, now, now)
		return err
	})
	if err != nil {
		return Session{}, err
	}
	return Session{SessionToken: token, PublicID: account.PublicID, Username: account.Username, DisplayName: display, Role: account.Role, Enabled: true, LastLoginAt: &now}, nil
}
func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "delete from user_session where session_token_hash=?", tokenHash(token))
		return err
	})
}
func (s *Service) UpdateProfile(ctx context.Context, token, displayName string) (Session, error) {
	displayName, err := normalizeDisplayName(displayName)
	if err != nil {
		return Session{}, err
	}
	session, err := s.Resolve(ctx, token)
	if err != nil {
		return Session{}, err
	}
	now := s.now().UnixMilli()
	if err := s.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "update user_profile set display_name=?,is_guest=0,last_seen_at=? where public_id=?", displayName, now, session.PublicID)
		return err
	}); err != nil {
		return Session{}, err
	}
	session.DisplayName = displayName
	return session, nil
}
func (s *Service) ChangePassword(ctx context.Context, token, current, next string) error {
	if len(next) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	session, err := s.Resolve(ctx, token)
	if err != nil {
		return err
	}
	if !session.Admin() {
		return errMemberPassword
	}
	var currentHash string
	if err := s.store.Reader().QueryRowContext(ctx, "select password_hash from user_account where public_id=?", session.PublicID).Scan(&currentHash); err != nil {
		return ErrUnknownSession
	}
	if !security.CheckPassword(currentHash, current) {
		return errCurrentPassword
	}
	nextHash, err := security.HashPassword(next)
	if err != nil {
		return err
	}
	now := s.now().UnixMilli()
	return s.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "update user_account set password_hash=?,updated_at=? where public_id=?", nextHash, now, session.PublicID)
		return err
	})
}
func (s *Service) UpgradeGuestToUser(ctx context.Context, guestToken, inviteSecret string) (Session, error) {
	guestSession, err := s.Resolve(ctx, guestToken)
	if err != nil {
		return Session{}, err
	}
	if !guestSession.Guest {
		return Session{}, errors.New("session is already a registered user")
	}
	if strings.TrimSpace(inviteSecret) == "" {
		return Session{}, errInvalidInvite
	}

	now := s.now().UnixMilli()
	newToken := uuid.NewString()
	username := "member_" + strings.TrimPrefix(guestSession.PublicID, "u_")
	passwordBytes := make([]byte, 32)
	if _, err := rand.Read(passwordBytes); err != nil {
		return Session{}, err
	}
	passwordHash, err := security.HashPassword(base64.RawURLEncoding.EncodeToString(passwordBytes))
	if err != nil {
		return Session{}, err
	}

	err = s.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		var inviteID, roomID string
		err := tx.QueryRowContext(ctx, "select id,room_id from room_invite where secret_hash=? and used_at is null and revoked_at is null and expires_at>=?", tokenHash(inviteSecret), now).Scan(&inviteID, &roomID)
		if err != nil {
			return errInvalidInvite
		}
		result, err := tx.ExecContext(ctx, "update room_invite set used_at=?,used_by_public_id=?,revoked_at=? where id=? and used_at is null and revoked_at is null and expires_at>=?", now, guestSession.PublicID, now, inviteID, now)
		if err != nil {
			return err
		}
		rows, _ := result.RowsAffected()
		if rows != 1 {
			return errInvalidInvite
		}

		// Create user_account for the guest
		if _, err = tx.ExecContext(ctx, "insert into user_account(username,public_id,password_hash,role,enabled,created_at,updated_at,last_login_at) values(?,?,?,'MEMBER',1,?,?,?)", username, guestSession.PublicID, passwordHash, now, now, now); err != nil {
			return err
		}

		// Update profile to mark as non-guest and set current room
		if _, err = tx.ExecContext(ctx, "update user_profile set is_guest=0,current_room_id=?,last_seen_at=? where public_id=?", roomID, now, guestSession.PublicID); err != nil {
			return err
		}

		// Create room membership
		if _, err = tx.ExecContext(ctx, "insert into room_membership(room_id,public_id,role,created_at,updated_at) values(?,?,'MEMBER',?,?)", roomID, guestSession.PublicID, now, now); err != nil {
			return err
		}

		// Create new session token
		_, err = tx.ExecContext(ctx, "insert into user_session(session_token_hash,public_id,created_at,last_seen_at) values(?,?,?,?)", tokenHash(newToken), guestSession.PublicID, now, now)
		return err
	})
	if err != nil {
		return Session{}, err
	}

	return Session{SessionToken: newToken, PublicID: guestSession.PublicID, Username: username, DisplayName: guestSession.DisplayName, Role: "MEMBER", Guest: false, Enabled: true, LastLoginAt: &now}, nil
}

func (s *Service) RedeemInvite(ctx context.Context, secret, displayName string) (Session, error) {
	displayName, err := normalizeDisplayName(displayName)
	if err != nil {
		return Session{}, err
	}
	if strings.TrimSpace(secret) == "" {
		return Session{}, errInvalidInvite
	}
	now := s.now().UnixMilli()
	token := uuid.NewString()
	publicID := "u_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	username := "member_" + strings.TrimPrefix(publicID, "u_")
	passwordBytes := make([]byte, 32)
	if _, err := rand.Read(passwordBytes); err != nil {
		return Session{}, err
	}
	passwordHash, err := security.HashPassword(base64.RawURLEncoding.EncodeToString(passwordBytes))
	if err != nil {
		return Session{}, err
	}
	err = s.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		var inviteID, roomID string
		err := tx.QueryRowContext(ctx, "select id,room_id from room_invite where secret_hash=? and used_at is null and revoked_at is null and expires_at>=?", tokenHash(secret), now).Scan(&inviteID, &roomID)
		if err != nil {
			return errInvalidInvite
		}
		result, err := tx.ExecContext(ctx, "update room_invite set used_at=?,used_by_public_id=?,revoked_at=? where id=? and used_at is null and revoked_at is null and expires_at>=?", now, publicID, now, inviteID, now)
		if err != nil {
			return err
		}
		rows, _ := result.RowsAffected()
		if rows != 1 {
			return errInvalidInvite
		}
		if _, err = tx.ExecContext(ctx, "insert into user_profile(public_id,display_name,is_guest,current_room_id,created_at,last_seen_at) values(?,?,0,?,?,?)", publicID, displayName, roomID, now, now); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "insert into user_account(username,public_id,password_hash,role,enabled,created_at,updated_at,last_login_at) values(?,?,?,'MEMBER',1,?,?,?)", username, publicID, passwordHash, now, now, now); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "insert into room_membership(room_id,public_id,role,created_at,updated_at) values(?,?,'MEMBER',?,?)", roomID, publicID, now, now); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, "insert into user_session(session_token_hash,public_id,created_at,last_seen_at) values(?,?,?,?)", tokenHash(token), publicID, now, now)
		return err
	})
	if err != nil {
		return Session{}, err
	}
	return Session{SessionToken: token, PublicID: publicID, Username: "", DisplayName: displayName, Role: "MEMBER", Enabled: true, LastLoginAt: &now}, nil
}
func (s *Service) CreateGuestSession(ctx context.Context, displayName string) (Session, error) {
	displayName, err := normalizeDisplayName(displayName)
	if err != nil {
		return Session{}, err
	}
	now := s.now().UnixMilli()
	token := uuid.NewString()
	publicID := "u_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]

	err = s.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "insert into user_profile(public_id,display_name,is_guest,current_room_id,created_at,last_seen_at) values(?,?,1,'lounge',?,?)", publicID, displayName, now, now); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, "insert into user_session(session_token_hash,public_id,created_at,last_seen_at) values(?,?,?,?)", tokenHash(token), publicID, now, now)
		return err
	})
	if err != nil {
		return Session{}, err
	}
	return Session{SessionToken: token, PublicID: publicID, Username: "", DisplayName: displayName, Role: "GUEST", Guest: true, Enabled: true, LastLoginAt: &now}, nil
}
func (s *Service) InviteMetadata(ctx context.Context, secret string) InviteMetadata {
	var value InviteMetadata
	err := s.store.Reader().QueryRowContext(ctx, `select i.room_id,r.name,i.expires_at from room_invite i join room r on r.id=i.room_id where i.secret_hash=? and i.used_at is null and i.revoked_at is null and i.expires_at>=?`, tokenHash(secret), s.now().UnixMilli()).Scan(&value.RoomID, &value.RoomName, &value.ExpiresAt)
	value.Valid = err == nil
	return value
}
func tokenHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
func normalizeUsername(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" || len([]rune(value)) > 32 || !usernamePattern.MatchString(value) {
		return "", errors.New("username must be 1-32 characters and contain only letters, numbers, dot, dash or underscore")
	}
	return value, nil
}
func normalizeDisplayName(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || len([]rune(value)) > 32 {
		return "", errors.New("display name must be 1-32 characters")
	}
	return value, nil
}
