package org.thornex.musicparty.service;

import org.junit.jupiter.api.Test;
import org.thornex.musicparty.persistence.InMemoryUserAccountRepository;
import org.thornex.musicparty.persistence.InMemoryUserProfileRepository;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

class AccountServiceTests {

    @Test
    void emptyAccountStoreRequiresBootstrapAndFirstRegisteredUserIsAdmin() {
        AccountService service = createService();

        assertThat(service.status().requiresSetup()).isTrue();

        AccountSession session = service.register("Alice", "correct-horse-battery-staple");

        assertThat(session.role()).isEqualTo("ADMIN");
        assertThat(session.guest()).isFalse();
        assertThat(service.status().requiresSetup()).isFalse();
        assertThat(service.isAdminSession(session.sessionToken())).isTrue();
    }

    @Test
    void loginRestoresExistingAccountSessionWithoutChangingPublicId() {
        AccountService service = createService();
        AccountSession registered = service.register("Alice", "correct-horse-battery-staple");

        AccountSession loggedIn = service.login("alice", "correct-horse-battery-staple");

        assertThat(loggedIn.sessionToken()).isNotEqualTo(registered.sessionToken());
        assertThat(loggedIn.publicId()).isEqualTo(registered.publicId());
        assertThat(loggedIn.role()).isEqualTo("ADMIN");
        assertThat(service.resolveSession(loggedIn.sessionToken()).map(AccountSession::publicId))
                .contains(registered.publicId());
    }

    @Test
    void subsequentRegisteredUsersAreNotAdmins() {
        AccountService service = createService();
        service.register("Alice", "correct-horse-battery-staple");

        AccountSession bob = service.register("Bob", "another-correct-horse");

        assertThat(bob.role()).isEqualTo("USER");
        assertThat(service.isAdminSession(bob.sessionToken())).isFalse();
    }

    @Test
    void rejectsDuplicateUsernameAndBadPassword() {
        AccountService service = createService();
        service.register("Alice", "correct-horse-battery-staple");

        assertThatThrownBy(() -> service.register(" alice ", "another-correct-horse"))
                .isInstanceOf(IllegalArgumentException.class)
                .hasMessageContaining("username already exists");
        assertThatThrownBy(() -> service.login("Alice", "wrong-password"))
                .isInstanceOf(IllegalArgumentException.class)
                .hasMessageContaining("Invalid username or password");
    }

    private AccountService createService() {
        return new AccountService(new InMemoryUserAccountRepository(), new InMemoryUserProfileRepository());
    }
}
