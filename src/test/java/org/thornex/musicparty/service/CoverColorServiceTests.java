package org.thornex.musicparty.service;

import org.junit.jupiter.api.Test;
import org.thornex.musicparty.config.AppProperties;

import java.lang.reflect.Method;

import static org.assertj.core.api.Assertions.assertThat;

class CoverColorServiceTests {

    @Test
    void rejectsLocalAndNonHttpCoverUrls() throws Exception {
        CoverColorService service = new CoverColorService(null, new AppProperties());
        Method method = CoverColorService.class.getDeclaredMethod("isSafeCoverUrl", String.class);
        method.setAccessible(true);

        assertThat((boolean) method.invoke(service, "file:///etc/passwd")).isFalse();
        assertThat((boolean) method.invoke(service, "http://localhost:8080/private.png")).isFalse();
        assertThat((boolean) method.invoke(service, "http://127.0.0.1/private.png")).isFalse();
        assertThat((boolean) method.invoke(service, "http://169.254.169.254/latest/meta-data")).isFalse();
    }

    @Test
    void allowsKnownLocalCoverPaths() throws Exception {
        CoverColorService service = new CoverColorService(null, new AppProperties());
        Method method = CoverColorService.class.getDeclaredMethod("isTrustedLocalCoverPath", String.class);
        method.setAccessible(true);

        assertThat((boolean) method.invoke(service, "/api/navidrome/cover/abc?token=user-token")).isTrue();
        assertThat((boolean) method.invoke(service, "/media/song.mp3")).isTrue();
        assertThat((boolean) method.invoke(service, "/actuator/env")).isFalse();
    }
}
