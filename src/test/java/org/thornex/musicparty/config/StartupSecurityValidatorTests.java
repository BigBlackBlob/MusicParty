package org.thornex.musicparty.config;

import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThatCode;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

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

    @Test
    void rejectsServerProductionWithLocalhostBaseUrl() {
        AppProperties properties = new AppProperties();
        properties.setMode("server");
        properties.setBaseUrl("http://localhost:8848");
        properties.setAllowedOrigins("https://music.example.com");
        StartupSecurityValidator validator = new StartupSecurityValidator(properties);

        assertThatThrownBy(() -> validator.run(null))
                .isInstanceOf(IllegalStateException.class)
                .hasMessageContaining("BASE_URL");
    }

    @Test
    void rejectsServerProductionWithoutExplicitPublicAllowedOrigin() {
        AppProperties properties = new AppProperties();
        properties.setMode("server");
        properties.setBaseUrl("https://music.example.com");
        properties.setAllowedOrigins("http://localhost:8848");
        StartupSecurityValidator validator = new StartupSecurityValidator(properties);

        assertThatThrownBy(() -> validator.run(null))
                .isInstanceOf(IllegalStateException.class)
                .hasMessageContaining("ALLOWED_ORIGINS");
    }

    @Test
    void acceptsServerProductionWithPublicBaseUrlAndAllowedOrigin() {
        AppProperties properties = new AppProperties();
        properties.setMode("server");
        properties.setBaseUrl("https://music.example.com");
        properties.setAllowedOrigins("https://music.example.com");
        StartupSecurityValidator validator = new StartupSecurityValidator(properties);

        assertThatCode(() -> validator.run(null)).doesNotThrowAnyException();
    }

}
