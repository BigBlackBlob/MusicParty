package sqlite

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	securitycompat "github.com/BigBlackBlob/MusicParty/backend-go/internal/security"
	"github.com/stretchr/testify/require"
)

func TestCredentialsMigrateOutsideDatabaseAndSurviveRestart(t *testing.T) {
	ctx := context.Background()
	cfg := StoreConfig{Path: filepath.Join(t.TempDir(), "credentials.db")}
	store := openTestStore(t, cfg)
	t.Cleanup(func() { _ = store.Close() })
	require.NoError(t, EnsureCompatibleSchema(ctx, store, true))
	oldKey, cookie, password := "legacy-master-key-sentinel", "cookie-private-sentinel", "subsonic-private-sentinel"
	encrypted, err := securitycompat.EncryptSubsonicCredential(password, oldKey)
	require.NoError(t, err)
	require.NoError(t, store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "insert into site_setting(setting_key,setting_value,secret,updated_at) values('security.site-secret.v1',?,1,1),('netease.cookie',?,1,1)", oldKey, cookie); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, "insert into subsonic_source(id,label,base_url,username,password,client,api_version,enabled,system,created_at,updated_at) values('source','Source','https://upstream.test','user',?,'musicparty','1.16.1',1,0,1,1)", encrypted)
		return err
	}))
	key, err := store.CredentialKey(ctx)
	require.NoError(t, err)
	require.NotEqual(t, oldKey, key)
	var count int
	require.NoError(t, store.Reader().QueryRowContext(ctx, "select count(*) from site_setting where setting_key='security.site-secret.v1'").Scan(&count))
	require.Zero(t, count)
	source, err := NewSubsonicSourceRepository(store).FindByID(ctx, "source")
	require.NoError(t, err)
	plain, err := securitycompat.DecryptSubsonicCredential(source.Password, key)
	require.NoError(t, err)
	require.Equal(t, password, plain)
	got, err := NewSiteSettingRepository(store).FindValue(ctx, "netease.cookie")
	require.NoError(t, err)
	require.Equal(t, cookie, *got)
	require.NoError(t, NewSiteSettingRepository(store).Upsert(ctx, SiteSetting{Key: "bilibili.sessdata", Value: &cookie, Secret: true}))
	require.NoError(t, store.Close())
	disk, err := os.ReadFile(cfg.Path)
	require.NoError(t, err)
	for _, sensitive := range []string{oldKey, key, cookie, password} {
		require.False(t, bytes.Contains(disk, []byte(sensitive)), "database file contains retired or plaintext credentials")
	}
	reopened := openTestStore(t, cfg)
	reloadedKey, err := reopened.CredentialKey(ctx)
	require.NoError(t, err)
	require.Equal(t, key, reloadedKey)
	got, err = NewSiteSettingRepository(reopened).FindValue(ctx, "bilibili.sessdata")
	require.NoError(t, err)
	require.Equal(t, cookie, *got)
	require.NoError(t, reopened.Close())
	require.NoError(t, os.Remove(cfg.Path+".key"))
	missing := openTestStore(t, cfg)
	_, err = missing.CredentialKey(ctx)
	require.ErrorIs(t, err, os.ErrNotExist)
	_, err = os.Stat(cfg.Path + ".key")
	require.ErrorIs(t, err, os.ErrNotExist, "a missing migrated key must not be regenerated")
	require.NoError(t, missing.Close())
	require.NoError(t, os.WriteFile(cfg.Path+".key", []byte("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="), 0600))
	wrong := openTestStore(t, cfg)
	_, err = wrong.CredentialKey(ctx)
	require.Error(t, err)
	require.NoError(t, wrong.Close())
}

func TestCredentialMigrationFailureRollsBackAndCanRetry(t *testing.T) {
	ctx := context.Background()
	store := openContractStore(t)
	defer store.Close()
	require.NoError(t, store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `insert into site_setting(setting_key,setting_value,secret,updated_at) values('netease.cookie','unchanged-cookie',1,1);
   create trigger reject_credential_update before update on site_setting begin select raise(abort,'injected failure'); end;`)
		return err
	}))
	_, err := store.CredentialKey(ctx)
	require.Error(t, err)
	var value string
	require.NoError(t, store.Reader().QueryRowContext(ctx, "select setting_value from site_setting where setting_key='netease.cookie'").Scan(&value))
	require.Equal(t, "unchanged-cookie", value)
	require.NoError(t, store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "drop trigger reject_credential_update")
		return err
	}))
	_, err = store.CredentialKey(ctx)
	require.NoError(t, err)
}
