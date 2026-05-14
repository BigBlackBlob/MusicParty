package org.thornex.musicparty.config;

import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThatCode;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

class StartupSecurityValidatorTests {

    @Test
    void rejectsMissingAdminPassword() {
        AppProperties properties = new AppProperties();
        StartupSecurityValidator validator = new StartupSecurityValidator(properties);

        assertThatThrownBy(() -> validator.run(null))
                .isInstanceOf(IllegalStateException.class)
                .hasMessageContaining("ADMIN_PASSWORD must be set");
    }

    @Test
    void rejectsInsecureDefaultAdminPassword() {
        AppProperties properties = new AppProperties();
        properties.setAdminPassword("admin123");
        StartupSecurityValidator validator = new StartupSecurityValidator(properties);

        assertThatThrownBy(() -> validator.run(null))
                .isInstanceOf(IllegalStateException.class)
                .hasMessageContaining("insecure default");
    }

    @Test
    void acceptsExplicitStrongAdminPassword() {
        AppProperties properties = new AppProperties();
        properties.setAdminPassword("strong-admin-password-12345");
        StartupSecurityValidator validator = new StartupSecurityValidator(properties);

        assertThatCode(() -> validator.run(null)).doesNotThrowAnyException();
    }
}
