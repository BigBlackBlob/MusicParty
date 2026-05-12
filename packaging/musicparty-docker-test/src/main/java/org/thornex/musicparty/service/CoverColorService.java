package org.thornex.musicparty.service;

import lombok.extern.slf4j.Slf4j;
import org.springframework.http.HttpStatusCode;
import org.springframework.stereotype.Service;
import org.springframework.util.StringUtils;
import org.springframework.web.reactive.function.client.WebClient;
import org.thornex.musicparty.dto.CoverColorResponse;
import reactor.core.publisher.Mono;

import javax.imageio.ImageIO;
import java.awt.*;
import java.awt.image.BufferedImage;
import java.io.ByteArrayInputStream;
import java.util.HashMap;
import java.util.Map;

@Service
@Slf4j
public class CoverColorService {

    private final WebClient webClient;

    public CoverColorService(WebClient webClient) {
        this.webClient = webClient;
    }

    public Mono<CoverColorResponse> extract(String coverUrl) {
        if (!StringUtils.hasText(coverUrl)) {
            return Mono.empty();
        }

        return webClient.get()
                .uri(coverUrl)
                .retrieve()
                .onStatus(HttpStatusCode::isError, response -> response.createException().flatMap(Mono::error))
                .bodyToMono(byte[].class)
                .flatMap(bytes -> {
                    try {
                        BufferedImage image = ImageIO.read(new ByteArrayInputStream(bytes));
                        if (image == null) {
                            return Mono.empty();
                        }

                        Color dominant = extractDominantColor(image);
                        return Mono.just(toResponse(dominant));
                    } catch (Exception e) {
                        log.warn("Failed to extract cover color from {}", coverUrl, e);
                        return Mono.empty();
                    }
                })
                .onErrorResume(error -> {
                    log.warn("Failed to fetch cover for color extraction: {}", coverUrl, error);
                    return Mono.empty();
                });
    }

    private Color extractDominantColor(BufferedImage image) {
        BufferedImage scaled = new BufferedImage(48, 48, BufferedImage.TYPE_INT_RGB);
        Graphics2D graphics = scaled.createGraphics();
        graphics.setRenderingHint(RenderingHints.KEY_INTERPOLATION, RenderingHints.VALUE_INTERPOLATION_BILINEAR);
        graphics.drawImage(image, 0, 0, 48, 48, null);
        graphics.dispose();

        Map<Integer, Integer> buckets = new HashMap<>();
        int bestBucket = 0;
        int bestWeight = -1;
        Color bestColor = new Color(211, 194, 243);

        for (int y = 0; y < scaled.getHeight(); y++) {
            for (int x = 0; x < scaled.getWidth(); x++) {
                Color color = new Color(scaled.getRGB(x, y));
                float[] hsb = Color.RGBtoHSB(color.getRed(), color.getGreen(), color.getBlue(), null);
                float saturation = hsb[1];
                float brightness = hsb[2];

                if (brightness < 0.14f || brightness > 0.92f || saturation < 0.18f) {
                    continue;
                }

                int quantizedRed = (color.getRed() / 24) * 24;
                int quantizedGreen = (color.getGreen() / 24) * 24;
                int quantizedBlue = (color.getBlue() / 24) * 24;
                int bucket = (quantizedRed << 16) | (quantizedGreen << 8) | quantizedBlue;

                int weight = 1 + Math.round(saturation * 6) + Math.round((1.0f - Math.abs(brightness - 0.58f)) * 3);
                int totalWeight = buckets.merge(bucket, weight, Integer::sum);
                if (totalWeight > bestWeight) {
                    bestWeight = totalWeight;
                    bestBucket = bucket;
                    bestColor = new Color(quantizedRed, quantizedGreen, quantizedBlue);
                }
            }
        }

        if (bestWeight < 0) {
            return bestColor;
        }

        return new Color((bestBucket >> 16) & 0xFF, (bestBucket >> 8) & 0xFF, bestBucket & 0xFF);
    }

    private CoverColorResponse toResponse(Color base) {
        Color hover = shift(base, 0.06f, 0.10f);
        String accent = toHex(base);
        String accentHover = toHex(hover);
        String accentMuted = toRgba(base, 0.16f);
        String accentSubtle = toRgba(base, 0.08f);
        return new CoverColorResponse(accent, accentHover, accentMuted, accentSubtle);
    }

    private Color shift(Color color, float saturationDelta, float brightnessDelta) {
        float[] hsb = Color.RGBtoHSB(color.getRed(), color.getGreen(), color.getBlue(), null);
        float saturation = Math.max(0.18f, Math.min(0.88f, hsb[1] + saturationDelta));
        float brightness = Math.max(0.26f, Math.min(0.96f, hsb[2] + brightnessDelta));
        return Color.getHSBColor(hsb[0], saturation, brightness);
    }

    private String toHex(Color color) {
        return String.format("#%02x%02x%02x", color.getRed(), color.getGreen(), color.getBlue());
    }

    private String toRgba(Color color, float alpha) {
        return String.format("rgba(%d, %d, %d, %.2f)", color.getRed(), color.getGreen(), color.getBlue(), alpha);
    }
}
