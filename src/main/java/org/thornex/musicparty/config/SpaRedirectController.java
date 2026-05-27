package org.thornex.musicparty.config;

import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.core.io.ClassPathResource;
import org.springframework.http.CacheControl;
import org.springframework.web.reactive.function.server.RouterFunction;
import org.springframework.web.reactive.function.server.ServerResponse;

import static org.springframework.web.reactive.function.server.RequestPredicates.GET;
import static org.springframework.web.reactive.function.server.RouterFunctions.route;

@Configuration
public class SpaRedirectController {

    @Bean
    RouterFunction<ServerResponse> spaFallbackRoutes() {
        ClassPathResource index = new ClassPathResource("static/index.html");
        return route(GET("/{*path}").and(SpaRedirectController::isSpaFallbackPath),
                request -> ServerResponse.ok()
                        .cacheControl(CacheControl.noStore())
                        .bodyValue(index));
    }

    private static boolean isSpaFallbackPath(org.springframework.web.reactive.function.server.ServerRequest request) {
        String path = request.path();
        String normalized = path.startsWith("/") ? path.substring(1) : path;
        return !normalized.isBlank()
                && !normalized.contains(".")
                && !normalized.equals("radio")
                && !normalized.startsWith("api/")
                && !normalized.startsWith("ws/")
                && !normalized.startsWith("proxy/")
                && !normalized.startsWith("media/");
    }
}
