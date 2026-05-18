package org.thornex.musicparty.config;

import org.springframework.context.annotation.Configuration;
import org.springframework.web.servlet.config.annotation.CorsRegistry;
import org.springframework.web.servlet.config.annotation.ResourceHandlerRegistry;
import org.springframework.web.servlet.config.annotation.WebMvcConfigurer;

import java.io.File;

@Configuration
public class LocalResourceConfig implements WebMvcConfigurer {

    // 存储目录
    public static final String CACHE_DIR = "cached_media";

    @Override
    public void addResourceHandlers(ResourceHandlerRegistry registry) {
        // 创建目录（如果不存在）
        File dir = new File(CACHE_DIR);
        if (!dir.exists()) {
            dir.mkdirs();
        }

        // 将 /media/** 映射到本地文件系统
        // file:cached_media/
        registry.addResourceHandler("/media/**")
                .addResourceLocations(dir.toURI().toString());
    }

    @Override
    public void addCorsMappings(CorsRegistry registry) {
        // 让 <audio crossorigin="anonymous"> 能拿到 ACAO 响应头，
        // Keep local media responses playable from the Vite/frontend origin.
        registry.addMapping("/media/**")
                .allowedOriginPatterns("*")
                .allowedMethods("GET", "HEAD")
                .exposedHeaders("Content-Length", "Content-Range", "Accept-Ranges")
                .allowCredentials(false);
    }
}
