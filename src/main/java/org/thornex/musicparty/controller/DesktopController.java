package org.thornex.musicparty.controller;

import lombok.RequiredArgsConstructor;
import org.springframework.util.StringUtils;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;
import org.thornex.musicparty.config.AppProperties;

@RestController
@RequestMapping("/api/desktop")
@RequiredArgsConstructor
public class DesktopController {

    private final AppProperties appProperties;

    @GetMapping("/status")
    public DesktopStatus status() {
        String baseUrl = trimTrailingSlash(appProperties.getBaseUrl());
        String lanBaseUrl = trimTrailingSlash(appProperties.getDesktop().getLanBaseUrl());
        String inviteBaseUrl = StringUtils.hasText(lanBaseUrl) ? lanBaseUrl : baseUrl;
        int neteaseApiPort = appProperties.getDesktop().getNeteaseApiPort();
        return new DesktopStatus(
                appProperties.getMode(),
                "desktop-host".equals(appProperties.getMode()),
                baseUrl,
                lanBaseUrl,
                StringUtils.hasText(inviteBaseUrl) ? inviteBaseUrl + "/room/lobby" : "",
                appProperties.getDesktop().getProfileDir(),
                "http://127.0.0.1:" + neteaseApiPort,
                appProperties.getDesktop().isAdminInitialized()
        );
    }

    private String trimTrailingSlash(String value) {
        if (!StringUtils.hasText(value)) return "";
        String trimmed = value.trim();
        while (trimmed.endsWith("/")) {
            trimmed = trimmed.substring(0, trimmed.length() - 1);
        }
        return trimmed;
    }

    public record DesktopStatus(
            String mode,
            boolean hosting,
            String baseUrl,
            String lanBaseUrl,
            String inviteUrl,
            String profileDir,
            String neteaseApiUrl,
            boolean adminInitialized
    ) {
    }
}
