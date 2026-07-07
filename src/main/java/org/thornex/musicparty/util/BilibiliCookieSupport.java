package org.thornex.musicparty.util;

import org.springframework.util.StringUtils;

public final class BilibiliCookieSupport {

    private BilibiliCookieSupport() {
    }

    public static String toCookieHeader(String configuredCookie) {
        if (!StringUtils.hasText(configuredCookie)) {
            return "";
        }
        String trimmed = configuredCookie.trim();
        if (trimmed.contains("=")) {
            return trimmed;
        }
        return "SESSDATA=" + trimmed;
    }
}
