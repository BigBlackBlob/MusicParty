package org.thornex.musicparty.config;

import org.springframework.context.annotation.Configuration;
import org.springframework.messaging.simp.config.MessageBrokerRegistry;
import org.springframework.scheduling.concurrent.ThreadPoolTaskScheduler;
import org.springframework.util.StringUtils;
import org.springframework.web.socket.config.annotation.EnableWebSocketMessageBroker;
import org.springframework.web.socket.config.annotation.StompEndpointRegistry;
import org.springframework.web.socket.config.annotation.WebSocketMessageBrokerConfigurer;

import java.net.URI;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;

@Configuration
@EnableWebSocketMessageBroker
public class WebSocketConfig implements WebSocketMessageBrokerConfigurer {

    private final WebSocketAuthInterceptor authInterceptor;
    private final AppProperties appProperties;

    public WebSocketConfig(WebSocketAuthInterceptor authInterceptor, AppProperties appProperties) {
        this.authInterceptor = authInterceptor;
        this.appProperties = appProperties;
    }

    @Override
    public void configureMessageBroker(MessageBrokerRegistry config) {
        ThreadPoolTaskScheduler te = new ThreadPoolTaskScheduler();
        te.setPoolSize(1);
        te.setThreadNamePrefix("wss-heartbeat-");
        te.initialize();

        config.enableSimpleBroker("/topic", "/queue")
                .setTaskScheduler(te) // 🟢 绑定调度器
                .setHeartbeatValue(new long[]{10000, 10000}); // 🟢 设置心跳：[发, 收] 均为 10秒

        config.setApplicationDestinationPrefixes("/app");
        config.setUserDestinationPrefix("/user");
    }

    @Override
    public void registerStompEndpoints(StompEndpointRegistry registry) {
        registry.addEndpoint("/ws")
                .setAllowedOriginPatterns(resolveAllowedOrigins());
    }

    @Override
    public void configureClientInboundChannel(org.springframework.messaging.simp.config.ChannelRegistration registration) {
        // 注册拦截器
        registration.interceptors(authInterceptor);
    }

    private String[] resolveAllowedOrigins() {
        List<String> origins = new ArrayList<>();
        if (StringUtils.hasText(appProperties.getAllowedOrigins())) {
            origins.addAll(Arrays.stream(appProperties.getAllowedOrigins().split(","))
                    .map(String::trim)
                    .filter(StringUtils::hasText)
                    .toList());
        }

        addOriginFromBaseUrl(origins, appProperties.getBaseUrl());
        origins.add("http://localhost:5173");
        origins.add("http://127.0.0.1:5173");
        origins.add("http://localhost:8080");
        origins.add("http://127.0.0.1:8080");

        return origins.stream().distinct().toArray(String[]::new);
    }

    private void addOriginFromBaseUrl(List<String> origins, String baseUrl) {
        if (!StringUtils.hasText(baseUrl)) {
            return;
        }
        try {
            URI uri = URI.create(baseUrl);
            if (StringUtils.hasText(uri.getScheme()) && StringUtils.hasText(uri.getHost())) {
                int port = uri.getPort();
                origins.add(uri.getScheme() + "://" + uri.getHost() + (port > -1 ? ":" + port : ""));
            }
        } catch (IllegalArgumentException ignored) {
        }
    }
}
