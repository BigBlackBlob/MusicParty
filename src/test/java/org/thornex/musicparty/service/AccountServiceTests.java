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

    @Test
    void changePasswordRequiresCurrentPasswordAndInvalidatesOldPassword() {
        AccountService service = createService();
        AccountSession registered = service.register("Alice", "correct-horse-battery-staple");

        assertThatThrownBy(() -> service.changePassword(registered.sessionToken(), "wrong-password", "new-correct-horse"))
                .isInstanceOf(IllegalArgumentException.class)
                .hasMessageContaining("Current password is incorrect");

        service.changePassword(registered.sessionToken(), "correct-horse-battery-staple", "new-correct-horse");

        assertThatThrownBy(() -> service.login("Alice", "correct-horse-battery-staple"))
                .isInstanceOf(IllegalArgumentException.class)
                .hasMessageContaining("Invalid username or password");
        assertThat(service.login("Alice", "new-correct-horse").publicId()).isEqualTo(registered.publicId());
    }

    @Test
    void logoutRevokesCurrentSession() {
        AccountService service = createService();
        AccountSession registered = service.register("Alice", "correct-horse-battery-staple");

        service.logout(registered.sessionToken());

        assertThat(service.resolveSession(registered.sessionToken())).isEmpty();
    }

    @Test
    void updateProfileChangesDisplayNameWithoutChangingPublicId() {
        AccountService service = createService();
        AccountSession registered = service.register("Alice", "correct-horse-battery-staple");

        AccountSession updated = service.updateProfile(registered.sessionToken(), "Alice Cooper");

        assertThat(updated.publicId()).isEqualTo(registered.publicId());
        assertThat(updated.displayName()).isEqualTo("Alice Cooper");
        assertThat(service.resolveSession(registered.sessionToken()).map(AccountSession::displayName))
                .contains("Alice Cooper");
    }

    private AccountService createService() {
        return new AccountService(new InMemoryUserAccountRepository(), new InMemoryUserProfileRepository());
    }
}
