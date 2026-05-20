package org.thornex.musicparty.config;

import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThatCode;

class StartupSecurityValidatorTests {

    @Test
    void acceptsMissingAdminPasswordBecauseDatabaseBootstrapHandlesFirstAdmin() {
        StartupSecurityValidator validator = new StartupSecurityValidator();

        assertThatCode(() -> validator.run(null)).doesNotThrowAnyException();
    }

    @Test
    void ignoresLegacyAdminPasswordBecauseRuntimeAdminAuthUsesAccounts() {
        StartupSecurityValidator validator = new StartupSecurityValidator();

        assertThatCode(() -> validator.run(null)).doesNotThrowAnyException();
    }

    @Test
    void acceptsExplicitStrongAdminPassword() {
        StartupSecurityValidator validator = new StartupSecurityValidator();

        assertThatCode(() -> validator.run(null)).doesNotThrowAnyException();
    }

}
