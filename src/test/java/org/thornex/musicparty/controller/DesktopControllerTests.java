package org.thornex.musicparty.controller;

import org.junit.jupiter.api.Test;
import org.thornex.musicparty.config.AppProperties;

import static org.assertj.core.api.Assertions.assertThat;

class DesktopControllerTests {

    @Test
    void reportsDesktopHostRuntimeStatusFromConfiguration() {
        AppProperties properties = new AppProperties();
        properties.setMode("desktop-host");
        properties.setBaseUrl("http://127.0.0.1:48120");
        properties.getDesktop().setProfileDir("C:/Users/me/AppData/Roaming/MusicParty");
        properties.getDesktop().setLanBaseUrl("http://192.168.1.10:48120");
        properties.getDesktop().setNeteaseApiPort(39201);
        properties.getDesktop().setAdminInitialized(true);

        DesktopController controller = new DesktopController(properties);

        DesktopController.DesktopStatus status = controller.status();

        assertThat(status.mode()).isEqualTo("desktop-host");
        assertThat(status.hosting()).isTrue();
        assertThat(status.baseUrl()).isEqualTo("http://127.0.0.1:48120");
        assertThat(status.lanBaseUrl()).isEqualTo("http://192.168.1.10:48120");
        assertThat(status.inviteUrl()).isEqualTo("http://192.168.1.10:48120/room/lobby");
        assertThat(status.profileDir()).isEqualTo("C:/Users/me/AppData/Roaming/MusicParty");
        assertThat(status.neteaseApiUrl()).isEqualTo("http://127.0.0.1:39201");
        assertThat(status.adminInitialized()).isTrue();
    }

    @Test
    void fallsBackToBaseUrlForInviteWhenLanBaseUrlIsMissing() {
        AppProperties properties = new AppProperties();
        properties.setMode("desktop-host");
        properties.setBaseUrl("http://127.0.0.1:48120");
        properties.getDesktop().setLanBaseUrl("");

        DesktopController controller = new DesktopController(properties);

        assertThat(controller.status().inviteUrl()).isEqualTo("http://127.0.0.1:48120/room/lobby");
    }
}
