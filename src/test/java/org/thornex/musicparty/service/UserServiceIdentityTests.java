package org.thornex.musicparty.service;

import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.dto.User;
import org.thornex.musicparty.dto.UserSummary;

import java.util.List;

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

    private UserService createService() {
        return new UserService(
                event -> {},
                new RoomService(new ObjectMapper(), event -> {}, new AppProperties())
        );
    }
}
