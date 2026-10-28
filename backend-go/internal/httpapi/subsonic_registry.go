package httpapi

import (
	"fmt"
	"strings"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform/subsonic"
	securitycompat "github.com/BigBlackBlob/MusicParty/backend-go/internal/security"
	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
)

func subsonicBindingConfig(source storesqlite.SubsonicSource, binding storesqlite.RoomSubsonicSource, password string) (subsonic.Config, string) {
	label := defaultValue(dereference(binding.DisplayLabel), source.Label)
	allowed := dereference(source.AllowedUsers)
	if binding.AllowedUsers != nil {
		allowed = *binding.AllowedUsers
	}
	return subsonic.Config{ID: "subsonic-" + source.ID + "@" + binding.RoomID, Label: label, RoomID: binding.RoomID,
		BaseURL: source.BaseURL, Username: source.Username, Password: password, Client: source.Client, Version: source.APIVersion,
		ProxyPrefix: "/api/subsonic/" + binding.RoomID + "/" + source.ID + "/cover", Enabled: source.Enabled && binding.Enabled}, allowed
}

// SubsonicServices shares startup and live binding semantics. Validate every saved
// credential, including disabled sources, so lost keys cannot silently hide data.
func SubsonicServices(client *platform.Client, sources []storesqlite.SubsonicSource, bindings []storesqlite.RoomSubsonicSource, secret string) ([]platform.Service, map[string]string, error) {
	byID := make(map[string]storesqlite.SubsonicSource, len(sources))
	passwords := make(map[string]string, len(sources))
	for _, source := range sources {
		password, err := securitycompat.DecryptSubsonicCredential(source.Password, secret)
		if err != nil {
			return nil, nil, fmt.Errorf("decrypt Subsonic source %q: saved credential cannot be read with the site secret", source.ID)
		}
		byID[source.ID], passwords[source.ID] = source, password
	}
	services := []platform.Service{}
	access := map[string]string{}
	for _, binding := range bindings {
		source, ok := byID[binding.SourceID]
		if !ok {
			return nil, nil, fmt.Errorf("Subsonic binding %q references missing source %q", binding.RoomID, binding.SourceID)
		}
		cfg, allowed := subsonicBindingConfig(source, binding, passwords[source.ID])
		// Keep disabled entries to shadow any environment-provided service of the
		// same name when the startup registry is assembled.
		services = append(services, subsonic.New(client, cfg))
		access[cfg.ID] = allowed
	}
	return services, access, nil
}

// replaceSubsonicSource publishes committed services and policies under one lock.
// Source writes are serialized by AdminAPI through this publication step.
func (api *PlatformAPI) replaceSubsonicSource(client *platform.Client, accounts *account.Service, source storesqlite.SubsonicSource, bindings []storesqlite.RoomSubsonicSource, password string) {
	type entry struct {
		service platform.Service
		allowed string
	}
	entries := make([]entry, 0, len(bindings))
	for _, binding := range bindings {
		cfg, allowed := subsonicBindingConfig(source, binding, password)
		entries = append(entries, entry{subsonic.New(client, cfg), allowed})
	}
	prefix := "subsonic-" + source.ID + "@"
	api.mu.Lock()
	for name, service := range api.services {
		if scoped, ok := service.(interface{ RoomID() string }); ok && name == prefix+scoped.RoomID() {
			delete(api.services, name)
			delete(api.access, name)
		}
	}
	for _, entry := range entries {
		name := entry.service.Name()
		api.services[name] = entry.service
		api.access[name] = PlatformAccess(accounts, entry.allowed)
	}
	api.mu.Unlock()
	api.cacheMu.Lock()
	for key := range api.cache {
		if strings.HasPrefix(key, prefix) {
			delete(api.cache, key)
		}
	}
	api.cacheMu.Unlock()
}
