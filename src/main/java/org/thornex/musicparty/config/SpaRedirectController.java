package org.thornex.musicparty.config;

import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.core.io.ClassPathResource;
import org.springframework.web.reactive.function.server.RouterFunction;
import org.springframework.web.reactive.function.server.ServerResponse;

import static org.springframework.web.reactive.function.server.RequestPredicates.GET;
import static org.springframework.web.reactive.function.server.RouterFunctions.route;

@Configuration
public class SpaRedirectController {

    @Bean
    RouterFunction<ServerResponse> spaFallbackRoutes() {
        ClassPathResource index = new ClassPathResource("static/index.html");
        return route(GET("/{path:^(?!ws|api|proxy|radio$)[^.]*$}"),
                request -> ServerResponse.ok().bodyValue(index));
    }
}
