package org.thornex.musicparty.config;

import org.junit.jupiter.api.Test;
import org.springframework.test.web.reactive.server.WebTestClient;

class SpaRedirectControllerTests {

    @Test
    void spaFallbackDoesNotAllowLongTermIndexCaching() {
        SpaRedirectController controller = new SpaRedirectController();
        WebTestClient client = WebTestClient.bindToRouterFunction(controller.spaFallbackRoutes()).build();

        client.get()
                .uri("/room/lobby")
                .exchange()
                .expectStatus().isOk()
                .expectHeader().valueEquals("Cache-Control", "no-store");
    }
}
