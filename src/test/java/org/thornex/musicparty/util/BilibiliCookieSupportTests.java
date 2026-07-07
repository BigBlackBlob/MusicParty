package org.thornex.musicparty.util;

import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThat;

class BilibiliCookieSupportTests {

    @Test
    void buildsSessdataCookieHeaderFromLegacyValue() {
        assertThat(BilibiliCookieSupport.toCookieHeader("abc123"))
                .isEqualTo("SESSDATA=abc123");
    }

    @Test
    void keepsFullCookieHeaderWhenConfigured() {
        assertThat(BilibiliCookieSupport.toCookieHeader("buvid3=x; SESSDATA=abc123; b_nut=y"))
                .isEqualTo("buvid3=x; SESSDATA=abc123; b_nut=y");
    }
}
