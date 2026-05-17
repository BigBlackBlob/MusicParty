package org.thornex.musicparty.service;

import jakarta.annotation.PostConstruct;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;
import org.springframework.util.StringUtils;
import org.springframework.web.reactive.function.client.WebClient;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.dto.SubsonicSourceRequest;
import org.thornex.musicparty.persistence.PersistedRoomSubsonicSource;
import org.thornex.musicparty.persistence.PersistedSubsonicSource;
import org.thornex.musicparty.persistence.SubsonicSourceRepository;
import org.thornex.musicparty.service.api.SubsonicClient;
import reactor.core.publisher.Mono;

import java.text.Normalizer;
import java.net.URI;
import java.util.Comparator;
import java.util.List;
import java.util.Locale;
import java.util.Optional;
import java.util.concurrent.ConcurrentHashMap;

@Service
@RequiredArgsConstructor
@Slf4j
public class SubsonicSourceRegistry {
    private static final String DEFAULT_CLIENT = "musicparty";
    private static final String DEFAULT_API_VERSION = "1.16.1";
    private static final int SYSTEM_SORT_ORDER = -1000;

    private final SubsonicSourceRepository repository;
    private final AppProperties appProperties;
    private final WebClient webClient;
    private final SubsonicCredentialCipher credentialCipher;
    private final ConcurrentHashMap<String, SubsonicSource> sources = new ConcurrentHashMap<>();
    private final ConcurrentHashMap<String, RoomSubsonicSource> roomSources = new ConcurrentHashMap<>();
    private final ConcurrentHashMap<String, Object> roomLocks = new ConcurrentHashMap<>();

    @PostConstruct
    public void init() {
        repository.findAll().stream().map(this::fromPersisted).forEach(source -> sources.put(source.id(), source));
        repository.findAllRoomBindings().stream().map(this::fromPersistedBinding).flatMap(Optional::stream)
                .forEach(binding -> roomSources.put(bindingKey(binding.roomId(), binding.id()), binding));
        seedLegacyNavidrome();
        seedSquidify();
        bindLegacyGlobalSourcesToDefaultRoom();
        log.info("Subsonic sources loaded: sources={}, roomBindings={}", sources.keySet(), roomSources.keySet());
    }

    public List<RoomSubsonicSource> list(String roomId) {
        String normalizedRoom = normalizeRoomId(roomId);
        return roomSources.values().stream()
                .filter(binding -> normalizedRoom.equals(binding.roomId()))
                .sorted(Comparator.comparing(RoomSubsonicSource::system).reversed()
                        .thenComparingInt(RoomSubsonicSource::sortOrder)
                        .thenComparing(RoomSubsonicSource::label))
                .toList();
    }

    public List<RoomSubsonicSource> listEnabledConfigured(String roomId) {
        return list(roomId).stream().filter(RoomSubsonicSource::active).toList();
    }

    public List<SubsonicSource> listSources() {
        return sources.values().stream()
                .sorted(Comparator.comparing(SubsonicSource::system).reversed().thenComparing(SubsonicSource::label))
                .toList();
    }

    public Optional<SubsonicSource> find(String sourceId) {
        return Optional.ofNullable(sources.get(normalizeId(sourceId)));
    }

    public Optional<RoomSubsonicSource> findRoomSource(String roomId, String sourceId) {
        return Optional.ofNullable(roomSources.get(bindingKey(normalizeRoomId(roomId), normalizeId(sourceId))));
    }

    public Optional<RoomSubsonicSource> findRoomSourceByPlatformId(String roomId, String platformId) {
        if (!StringUtils.hasText(platformId) || !platformId.startsWith("subsonic-")) {
            return Optional.empty();
        }
        String value = platformId.substring("subsonic-".length());
        int roomMarker = value.indexOf('@');
        String sourceId = roomMarker >= 0 ? value.substring(0, roomMarker) : value;
        String platformRoomId = roomMarker >= 0 ? value.substring(roomMarker + 1) : roomId;
        return findRoomSource(platformRoomId, sourceId);
    }

    public Optional<SubsonicSource> findByPlatformId(String platformId) {
        if (!StringUtils.hasText(platformId) || !platformId.startsWith("subsonic-")) {
            return Optional.empty();
        }
        String value = platformId.substring("subsonic-".length());
        int roomMarker = value.indexOf('@');
        return find(roomMarker >= 0 ? value.substring(0, roomMarker) : value);
    }

    public SubsonicSource upsert(String roomId, SubsonicSourceRequest request) {
        String normalizedRoom = normalizeRoomId(roomId);
        synchronized (lockForRoom(normalizedRoom)) {
            String id = normalizeId(request.id());
            if (!StringUtils.hasText(id)) {
                throw new IllegalArgumentException("Source id is required");
            }
            boolean enabled = request.enabled() == null || request.enabled();
            long now = System.currentTimeMillis();
            SubsonicSource existing = sources.get(id);
            if (existing != null && existing.system()) {
                bindRoomSource(normalizedRoom, existing, enabled, request.label(), request.allowedUsers(), SYSTEM_SORT_ORDER);
                return existing;
            }
            if (existing != null && StringUtils.hasText(existing.ownerRoomId()) && !normalizedRoom.equals(existing.ownerRoomId())) {
                throw new IllegalArgumentException("Source id already belongs to another Lounge");
            }
            SubsonicSource source = existing == null
                    ? new SubsonicSource(
                    id,
                    normalizedRoom,
                    StringUtils.hasText(request.label()) ? request.label().trim() : id,
                    normalizeBaseUrl(requireText(request.baseUrl(), "baseUrl")),
                    requireText(request.username(), "username"),
                    requireText(request.password(), "password"),
                    DEFAULT_CLIENT,
                    DEFAULT_API_VERSION,
                    StringUtils.hasText(request.allowedUsers()) ? request.allowedUsers().trim() : "*",
                    true,
                    false,
                    now,
                    now
            )
                    : existing.withUpdatedConfig(request.label(), normalizeOptionalBaseUrl(request.baseUrl()), request.username(), request.password(), request.allowedUsers(), true);
            saveSource(source);
            bindRoomSource(normalizedRoom, source, enabled, request.label(), request.allowedUsers(), nextSortOrder(normalizedRoom));
            return source;
        }
    }

    public boolean remove(String roomId, String sourceId) {
        String normalizedRoom = normalizeRoomId(roomId);
        synchronized (lockForRoom(normalizedRoom)) {
            String id = sourceIdFromPlatformId(sourceId);
            RoomSubsonicSource binding = roomSources.get(bindingKey(normalizedRoom, id));
            if (binding == null) {
                return false;
            }
            SubsonicSource source = sources.get(id);
            if (source != null && source.system()) {
                PersistedRoomSubsonicSource disabledBinding = new PersistedRoomSubsonicSource(
                        normalizedRoom,
                        id,
                        false,
                        binding.displayLabel(),
                        binding.allowedUsers(),
                        binding.sortOrder(),
                        binding.createdAt(),
                        System.currentTimeMillis()
                );
                repository.upsertRoomBinding(disabledBinding);
                fromPersistedBinding(disabledBinding).ifPresent(roomSource -> roomSources.put(bindingKey(normalizedRoom, id), roomSource));
                return true;
            }
            repository.deleteRoomBinding(normalizedRoom, id);
            roomSources.remove(bindingKey(normalizedRoom, id));
            if (source != null && normalizedRoom.equals(source.ownerRoomId())) {
                repository.delete(id);
                sources.remove(id);
            }
            return true;
        }
    }

    public RoomSubsonicSource updateSortOrder(String roomId, String sourceId, int sortOrder) {
        String normalizedRoom = normalizeRoomId(roomId);
        synchronized (lockForRoom(normalizedRoom)) {
            RoomSubsonicSource existing = findRoomSource(normalizedRoom, sourceIdFromPlatformId(sourceId))
                    .orElseThrow(() -> new IllegalArgumentException("Source not found in this Lounge"));
            PersistedRoomSubsonicSource binding = new PersistedRoomSubsonicSource(
                    normalizedRoom,
                    existing.id(),
                    existing.enabled(),
                    existing.displayLabel(),
                    existing.allowedUsers(),
                    sortOrder,
                    existing.createdAt(),
                    System.currentTimeMillis()
            );
            repository.upsertRoomBinding(binding);
            RoomSubsonicSource updated = fromPersistedBinding(binding)
                    .orElseThrow(() -> new IllegalArgumentException("Source not found in this Lounge"));
            roomSources.put(bindingKey(normalizedRoom, existing.id()), updated);
            return updated;
        }
    }

    public Mono<Void> test(RoomSubsonicSource roomSource) {
        return client(roomSource.source()).ping();
    }

    public SubsonicClient client(SubsonicSource source) {
        return new SubsonicClient(webClient, source);
    }

    public static String normalizeId(String raw) {
        if (!StringUtils.hasText(raw)) return "";
        String normalized = Normalizer.normalize(raw.trim().toLowerCase(Locale.ROOT), Normalizer.Form.NFKD)
                .replaceAll("[^a-z0-9_-]+", "-")
                .replaceAll("^-+|-+$", "");
        return normalized.length() > 48 ? normalized.substring(0, 48) : normalized;
    }

    public static String normalizeRoomId(String raw) {
        return StringUtils.hasText(raw) ? raw.trim() : RoomService.DEFAULT_ROOM_ID;
    }

    private void bindRoomSource(String roomId, SubsonicSource source, boolean enabled, String displayLabel, String allowedUsers, int sortOrder) {
        long now = System.currentTimeMillis();
        PersistedRoomSubsonicSource existing = repository.findRoomBinding(roomId, source.id()).orElse(null);
        PersistedRoomSubsonicSource binding = new PersistedRoomSubsonicSource(
                roomId,
                source.id(),
                enabled,
                StringUtils.hasText(displayLabel) ? displayLabel.trim() : null,
                StringUtils.hasText(allowedUsers) ? allowedUsers.trim() : source.allowedUsers(),
                existing == null ? sortOrder : existing.sortOrder(),
                existing == null ? now : existing.createdAt(),
                now
        );
        repository.upsertRoomBinding(binding);
        fromPersistedBinding(binding).ifPresent(roomSource -> roomSources.put(bindingKey(roomId, source.id()), roomSource));
    }

    private void bindLegacyGlobalSourcesToDefaultRoom() {
        for (SubsonicSource source : sources.values()) {
            if (!StringUtils.hasText(source.ownerRoomId()) && !roomSources.containsKey(bindingKey(RoomService.DEFAULT_ROOM_ID, source.id()))) {
                bindRoomSource(RoomService.DEFAULT_ROOM_ID, source, source.enabled(), source.label(), source.allowedUsers(), source.system() ? SYSTEM_SORT_ORDER : nextSortOrder(RoomService.DEFAULT_ROOM_ID));
            }
        }
    }

    private void seedLegacyNavidrome() {
        AppProperties.NavidromeApiConfig config = appProperties.getNavidrome();
        if (config == null || !config.isEnabled()) {
            return;
        }
        SubsonicSource source = SubsonicClient.sourceFromLegacyNavidrome(
                config.getBaseUrl(),
                config.getUsername(),
                config.getPassword(),
                config.getClient(),
                config.getApiVersion(),
                config.getAllowedUsers(),
                true
        );
        saveSource(source);
        bindRoomSource(RoomService.DEFAULT_ROOM_ID, source, true, source.label(), source.allowedUsers(), SYSTEM_SORT_ORDER);
    }

    private void seedSquidify() {
        AppProperties.SquidifyConfig config = appProperties.getSquidify();
        if (config == null || !config.isEnabled()) {
            return;
        }
        String id = StringUtils.hasText(config.getId()) ? normalizeId(config.getId()) : "squidify";
        long now = System.currentTimeMillis();
        SubsonicSource source = new SubsonicSource(
                id,
                null,
                StringUtils.hasText(config.getLabel()) ? config.getLabel() : "Squidify",
                config.getBaseUrl(),
                config.getUsername(),
                config.getPassword(),
                StringUtils.hasText(config.getClient()) ? config.getClient() : DEFAULT_CLIENT,
                StringUtils.hasText(config.getApiVersion()) ? config.getApiVersion() : DEFAULT_API_VERSION,
                StringUtils.hasText(config.getAllowedUsers()) ? config.getAllowedUsers() : "*",
                true,
                true,
                now,
                now
        );
        if (source.isConfigured()) {
            saveSource(source);
            bindRoomSource(RoomService.DEFAULT_ROOM_ID, source, true, source.label(), source.allowedUsers(), SYSTEM_SORT_ORDER + 1);
        } else {
            log.info("Squidify source seed skipped because base URL or credentials are missing");
        }
    }

    private int nextSortOrder(String roomId) {
        return list(roomId).stream().mapToInt(RoomSubsonicSource::sortOrder).max().orElse(0) + 10;
    }

    private String requireText(String value, String field) {
        if (!StringUtils.hasText(value)) {
            throw new IllegalArgumentException(field + " is required");
        }
        return value.trim();
    }

    private String normalizeOptionalBaseUrl(String raw) {
        return StringUtils.hasText(raw) ? normalizeBaseUrl(raw) : raw;
    }

    private String normalizeBaseUrl(String raw) {
        String value = raw.trim();
        if (!value.matches("^[a-zA-Z][a-zA-Z0-9+.-]*://.*")) {
            value = (value.endsWith(":443") ? "https://" : "http://") + value;
        }
        try {
            URI uri = URI.create(value);
            if (!StringUtils.hasText(uri.getHost())) {
                throw new IllegalArgumentException("baseUrl must include a host");
            }
            String scheme = uri.getScheme() == null ? "http" : uri.getScheme().toLowerCase(Locale.ROOT);
            if (!"http".equals(scheme) && !"https".equals(scheme)) {
                throw new IllegalArgumentException("baseUrl must use http or https");
            }
            String path = StringUtils.hasText(uri.getRawPath()) ? uri.getRawPath().replaceAll("/+$", "") : "";
            return new URI(scheme, uri.getUserInfo(), uri.getHost(), uri.getPort(), path, null, null).toString();
        } catch (IllegalArgumentException e) {
            throw e;
        } catch (Exception e) {
            throw new IllegalArgumentException("baseUrl is invalid");
        }
    }

    private void saveSource(SubsonicSource source) {
        sources.put(source.id(), source);
        repository.upsert(toPersisted(source));
    }

    private Optional<RoomSubsonicSource> fromPersistedBinding(PersistedRoomSubsonicSource binding) {
        SubsonicSource source = sources.get(binding.sourceId());
        if (source == null) {
            return Optional.empty();
        }
        return Optional.of(new RoomSubsonicSource(
                binding.roomId(),
                source,
                binding.enabled(),
                binding.displayLabel(),
                binding.allowedUsers(),
                binding.sortOrder(),
                binding.createdAt(),
                binding.updatedAt()
        ));
    }

    private SubsonicSource fromPersisted(PersistedSubsonicSource source) {
        String password = credentialCipher.decrypt(source.password());
        boolean needsEncryption = StringUtils.hasText(source.password()) && !credentialCipher.isEncrypted(source.password());
        SubsonicSource hydrated = new SubsonicSource(
                source.id(),
                source.ownerRoomId(),
                source.label(),
                source.baseUrl(),
                source.username(),
                password,
                source.client(),
                source.apiVersion(),
                source.allowedUsers(),
                source.enabled(),
                source.system(),
                source.createdAt(),
                source.updatedAt()
        );
        if (needsEncryption) {
            repository.upsert(toPersisted(hydrated));
        }
        return hydrated;
    }

    private PersistedSubsonicSource toPersisted(SubsonicSource source) {
        return new PersistedSubsonicSource(
                source.id(),
                source.ownerRoomId(),
                source.label(),
                source.baseUrl(),
                source.username(),
                credentialCipher.encrypt(source.password()),
                source.client(),
                source.apiVersion(),
                source.allowedUsers(),
                source.enabled(),
                source.system(),
                source.createdAt(),
                source.updatedAt()
        );
    }

    private String bindingKey(String roomId, String sourceId) {
        return normalizeRoomId(roomId) + "::" + normalizeId(sourceId);
    }

    private String sourceIdFromPlatformId(String value) {
        if (!StringUtils.hasText(value)) return "";
        String raw = value.trim();
        if (raw.startsWith("subsonic-")) {
            raw = raw.substring("subsonic-".length());
        }
        int roomMarker = raw.indexOf('@');
        return normalizeId(roomMarker >= 0 ? raw.substring(0, roomMarker) : raw);
    }

    private Object lockForRoom(String roomId) {
        return roomLocks.computeIfAbsent(normalizeRoomId(roomId), ignored -> new Object());
    }
}
