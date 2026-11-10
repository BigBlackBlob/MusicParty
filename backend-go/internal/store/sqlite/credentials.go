package sqlite

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	securitycompat "github.com/BigBlackBlob/MusicParty/backend-go/internal/security"
)

const externalCredentialMigration = "security.external-key.v1"

// CredentialKey loads a key outside SQLite and atomically migrates legacy credentials.
// A missing key after migration is an error, never a reason to replace the key.
func (s *Store) CredentialKey(ctx context.Context) (string, error) {
	s.credentialMu.Lock()
	defer s.credentialMu.Unlock()
	if s.credentialKey != "" {
		return s.credentialKey, nil
	}
	var migrated bool
	if err := s.reader.QueryRowContext(ctx, "select exists(select 1 from migration_state where migration_key=?)", externalCredentialMigration).Scan(&migrated); err != nil {
		return "", err
	}
	key, err := loadCredentialKey(s.credentialKeyPath, !migrated)
	if err != nil {
		return "", fmt.Errorf("load external credential key: %w", err)
	}
	err = s.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "pragma secure_delete=ON"); err != nil {
			return err
		}
		// Another Store may have completed migration while this one loaded the file.
		if err := tx.QueryRowContext(ctx, "select exists(select 1 from migration_state where migration_key=?)", externalCredentialMigration).Scan(&migrated); err != nil {
			return err
		}
		var old sql.NullString
		if !migrated {
			err := tx.QueryRowContext(ctx, "select setting_value from site_setting where setting_key='security.site-secret.v1'").Scan(&old)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
		}
		oldKey := old.String
		if migrated {
			oldKey = key
		}
		rows, err := tx.QueryContext(ctx, "select id,password from subsonic_source")
		if err != nil {
			return err
		}
		type credential struct{ id, value string }
		sources := []credential{}
		for rows.Next() {
			var v credential
			if err := rows.Scan(&v.id, &v.value); err != nil {
				rows.Close()
				return err
			}
			sources = append(sources, v)
		}
		rowErr := rows.Err()
		rows.Close()
		if rowErr != nil {
			return rowErr
		}
		for _, source := range sources {
			plain, err := securitycompat.DecryptSubsonicCredential(source.value, oldKey)
			if err != nil {
				return fmt.Errorf("decrypt saved Subsonic credential: %w", err)
			}
			if migrated {
				continue
			}
			sealed := plain
			if strings.TrimSpace(plain) != "" {
				sealed, err = securitycompat.EncryptCredential(plain, key)
				if err != nil {
					return err
				}
			}
			if _, err := tx.ExecContext(ctx, "update subsonic_source set password=? where id=?", sealed, source.id); err != nil {
				return err
			}
		}
		for _, name := range []string{"netease.cookie", "bilibili.sessdata"} {
			var value sql.NullString
			err := tx.QueryRowContext(ctx, "select setting_value from site_setting where setting_key=?", name).Scan(&value)
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			if err != nil {
				return err
			}
			if !value.Valid {
				continue
			}
			if migrated {
				if _, err := securitycompat.DecryptSubsonicCredential(value.String, key); err != nil {
					return fmt.Errorf("decrypt saved platform credential: %w", err)
				}
				continue
			}
			sealed, err := securitycompat.EncryptCredential(value.String, key)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, "update site_setting set setting_value=?,secret=1 where setting_key=?", sealed, name); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, "delete from site_setting where setting_key='security.site-secret.v1'"); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, "insert into migration_state(migration_key,completed_at) values(?,unixepoch()*1000) on conflict(migration_key) do nothing", externalCredentialMigration)
		return err
	})
	if err != nil {
		return "", err
	}
	// Drop WAL pages that still contain legacy plaintext or the retired key.
	var busy, pages, checkpointed int
	if err := s.writer.QueryRowContext(ctx, "pragma wal_checkpoint(TRUNCATE)").Scan(&busy, &pages, &checkpointed); err != nil {
		return "", err
	}
	if busy != 0 {
		return "", errors.New("credential migration checkpoint is busy; retry initialization")
	}
	s.credentialKey = key
	return key, nil
}

func loadCredentialKey(path string, create bool) (string, error) {
	read := func() (string, error) {
		info, err := os.Lstat(path)
		if err != nil {
			return "", err
		}
		if !info.Mode().IsRegular() {
			return "", errors.New("credential key must be a regular file")
		}
		if runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
			return "", errors.New("credential key permissions must exclude group and other users")
		}
		if info.Size() > 128 {
			return "", errors.New("invalid credential key size")
		}
		bytes, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		value := strings.TrimSpace(string(bytes))
		raw, err := base64.StdEncoding.DecodeString(value)
		if err != nil || len(raw) != 32 {
			return "", errors.New("credential key must contain a base64 encoded 32-byte key")
		}
		return value, nil
	}
	value, err := read()
	if err == nil || !errors.Is(err, os.ErrNotExist) || !create {
		return value, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", err
	}
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".credential-key-*")
	if err != nil {
		return "", err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	_, writeErr := file.WriteString(base64.StdEncoding.EncodeToString(random))
	syncErr := file.Sync()
	closeErr := file.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		return "", err
	}
	// Link publishes a complete file without overwriting another initializer's key.
	if err := os.Link(temporary, path); err != nil && !errors.Is(err, os.ErrExist) {
		return "", err
	}
	return read()
}

func isPlatformCredential(key string) bool {
	return key == "netease.cookie" || key == "bilibili.sessdata"
}
