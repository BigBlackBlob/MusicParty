package org.thornex.musicparty.service;

import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.dto.User;
import org.thornex.musicparty.dto.UserSummary;
import org.thornex.musicparty.persistence.PersistedRoom;
import org.thornex.musicparty.persistence.PersistedSession;
import org.thornex.musicparty.persistence.PersistedUserProfile;
import org.thornex.musicparty.persistence.InMemoryMigrationStateRepository;
import org.thornex.musicparty.persistence.RoomRepository;
import org.thornex.musicparty.persistence.UserProfileRepository;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.concurrent.ConcurrentHashMap;

import static org.assertj.core.api.Assertions.assertThat;

class UserServiceIdentityTests {

    @Test
    void createsServerIssuedSessionTokenAndPublicId() {
        UserService service = createService();

        User user = service.handleConnect("session-1", null, "Alice");

        assertThat(user.getSessionToken()).isNotBlank();
        assertThat(user.getPublicId()).startsWith("u_");
        assertThat(user.getSessionToken()).isNotEqualTo(user.getPublicId());
    }

    @Test
    void reconnectsOnlyWithKnownSessionToken() {
        UserService service = createService();
        User first = service.handleConnect("session-1", null, "Alice");

        User reconnected = service.handleConnect("session-2", first.getSessionToken(), "Ignored");
        User forged = service.handleConnect("session-3", "client-forged-token", "Mallory");

        assertThat(reconnected.getPublicId()).isEqualTo(first.getPublicId());
        assertThat(forged.getPublicId()).isNotEqualTo(first.getPublicId());
        assertThat(forged.getSessionToken()).isNotEqualTo("client-forged-token");
    }

    @Test
    void onlineSummariesExposeOnlyPublicIdentity() {
        UserService service = createService();
        User user = service.handleConnect("session-1", null, "Alice");

        List<UserSummary> summaries = service.getOnlineUserSummaries();

        assertThat(summaries).containsExactly(new UserSummary(user.getPublicId(), "Alice", false));
        assertThat(summaries.getFirst().publicId()).isNotEqualTo(user.getSessionToken());
    }

    @Test
    void restoresPersistedIdentityFromSessionToken() {
        InMemoryUserProfileRepository userProfiles = new InMemoryUserProfileRepository();
        UserService service = createService(userProfiles);

        userProfiles.upsertProfile(new PersistedUserProfile("u_known", "Alice", false, 1L, 1L));
        userProfiles.upsertSession(new PersistedSession(sha256("server-issued-token"), "u_known", 1L, 1L));

        User restored = service.handleConnect("session-2", "server-issued-token", "Ignored");

        assertThat(restored.getPublicId()).isEqualTo("u_known");
        assertThat(restored.getName()).isEqualTo("Alice");
        assertThat(restored.getSessionToken()).isEqualTo("server-issued-token");
    }

    private UserService createService() {
        return createService(new InMemoryUserProfileRepository());
    }

    private UserService createService(UserProfileRepository userProfileRepository) {
        return new UserService(
                event -> {},
                new RoomService(new ObjectMapper(), event -> {}, new AppProperties(), new InMemoryRoomRepository(), new InMemoryMigrationStateRepository()),
                userProfileRepository
        );
    }

    private static String sha256(String value) {
        try {
            MessageDigest digest = MessageDigest.getInstance("SHA-256");
            byte[] bytes = digest.digest(value.getBytes(StandardCharsets.UTF_8));
            StringBuilder builder = new StringBuilder(bytes.length * 2);
            for (byte current : bytes) {
                builder.append(String.format("%02x", current));
            }
            return builder.toString();
        } catch (Exception e) {
            throw new IllegalStateException(e);
        }
    }

    private static final class InMemoryRoomRepository implements RoomRepository {
        private final Map<String, PersistedRoom> rooms = new ConcurrentHashMap<>();

        @Override
        public List<PersistedRoom> findAllActive() {
            return rooms.values().stream().filter(room -> room.deletedAt() == null).toList();
        }

        @Override
        public Optional<PersistedRoom> findById(String roomId) {
            PersistedRoom room = rooms.get(roomId);
            return room == null || room.deletedAt() != null ? Optional.empty() : Optional.of(room);
        }

        @Override
        public void upsert(PersistedRoom room) {
            rooms.put(room.id(), room);
        }

        @Override
        public void touch(String roomId, long lastActiveAt) {
            rooms.computeIfPresent(roomId, (key, room) -> new PersistedRoom(
                    room.id(),
                    room.name(),
                    room.ownerPublicId(),
                    room.visibility(),
                    room.passwordHash(),
                    room.passwordVersion(),
                    room.system(),
                    room.createdAt(),
                    lastActiveAt,
                    room.deletedAt()
            ));
        }

        @Override
        public void softDelete(String roomId, long deletedAt) {
            rooms.computeIfPresent(roomId, (key, room) -> new PersistedRoom(
                    room.id(),
                    room.name(),
                    room.ownerPublicId(),
                    room.visibility(),
                    room.passwordHash(),
                    room.passwordVersion(),
                    room.system(),
                    room.createdAt(),
                    room.lastActiveAt(),
                    deletedAt
            ));
        }
    }

    private static final class InMemoryUserProfileRepository implements UserProfileRepository {
        private final Map<String, PersistedUserProfile> profiles = new ConcurrentHashMap<>();
        private final Map<String, PersistedSession> sessions = new ConcurrentHashMap<>();

        @Override
        public void upsertProfile(PersistedUserProfile profile) {
            profiles.put(profile.publicId(), profile);
        }

        @Override
        public Optional<PersistedUserProfile> findByPublicId(String publicId) {
            return Optional.ofNullable(profiles.get(publicId));
        }

        @Override
        public void upsertSession(PersistedSession session) {
            sessions.put(session.sessionTokenHash(), session);
        }

        @Override
        public Optional<PersistedSession> findSessionByHash(String sessionTokenHash) {
            return Optional.ofNullable(sessions.get(sessionTokenHash));
        }
    }
}
