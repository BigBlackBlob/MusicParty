package org.thornex.musicparty.service;

import lombok.extern.slf4j.Slf4j;
import org.springframework.http.MediaType;
import org.springframework.http.HttpStatusCode;
import org.springframework.stereotype.Service;
import org.springframework.util.StringUtils;
import org.springframework.web.reactive.function.client.WebClient;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.dto.CoverColorResponse;
import reactor.core.publisher.Mono;

import javax.imageio.ImageIO;
import java.awt.*;
import java.awt.image.BufferedImage;
import java.io.ByteArrayInputStream;
import java.net.InetAddress;
import java.net.URI;
import java.net.UnknownHostException;
import java.util.HashMap;
import java.util.Map;
import java.util.Set;

@Service
@Slf4j
public class CoverColorService {

    private final WebClient webClient;
    private final AppProperties appProperties;
    private static final long MAX_COVER_BYTES = 3 * 1024 * 1024;
    private static final Set<String> ALLOWED_SCHEMES = Set.of("http", "https");

    public CoverColorService(WebClient webClient, AppProperties appProperties) {
        this.webClient = webClient;
        this.appProperties = appProperties;
    }

    public Mono<CoverColorResponse> extract(String coverUrl) {
        if (!StringUtils.hasText(coverUrl)) {
            return Mono.empty();
        }

        boolean trustedLocalPath = isTrustedLocalCoverPath(coverUrl);
        String resolvedUrl = resolveCoverUrl(coverUrl);
        if (!trustedLocalPath && !isSafeCoverUrl(resolvedUrl)) {
            log.warn("Rejected unsafe cover URL for color extraction: {}", resolvedUrl);
            return Mono.empty();
        }

        return webClient.get()
                .uri(resolvedUrl)
                .exchangeToMono(response -> {
                    HttpStatusCode status = response.statusCode();
                    if (status.is3xxRedirection()) {
                        String location = response.headers().header("Location").stream().findFirst().orElse("");
                        String redirectUrl = resolveRedirectUrl(resolvedUrl, location);
                        if (!isSafeCoverUrl(redirectUrl)) {
                            log.warn("Rejected unsafe cover redirect: from={}, to={}", resolvedUrl, redirectUrl);
                        } else {
                            log.warn("Rejected cover redirect for color extraction: from={}, to={}", resolvedUrl, redirectUrl);
                        }
                        return Mono.empty();
                    }
                    if (response.statusCode().isError()) {
                        return response.createException().flatMap(Mono::error);
                    }
                    MediaType contentType = response.headers().contentType().orElse(null);
                    if (contentType == null || !"image".equalsIgnoreCase(contentType.getType())) {
                        log.warn("Rejected non-image cover response: url={}, contentType={}", resolvedUrl, contentType);
                        return Mono.empty();
                    }
                    long contentLength = response.headers().contentLength().orElse(-1);
                    if (contentLength > MAX_COVER_BYTES) {
                        log.warn("Rejected oversized cover response: url={}, bytes={}", resolvedUrl, contentLength);
                        return Mono.empty();
                    }
                    return response.bodyToMono(byte[].class);
                })
                .filter(bytes -> {
                    boolean allowed = bytes.length <= MAX_COVER_BYTES;
                    if (!allowed) {
                        log.warn("Rejected oversized cover body: url={}, bytes={}", resolvedUrl, bytes.length);
                    }
                    return allowed;
                })
                .flatMap(bytes -> {
                    try {
                        BufferedImage image = ImageIO.read(new ByteArrayInputStream(bytes));
                        if (image == null) {
                            return Mono.empty();
                        }

                        Color dominant = extractDominantColor(image);
                        return Mono.just(toResponse(dominant));
                    } catch (Exception e) {
                        log.warn("Failed to extract cover color from {}", resolvedUrl, e);
                        return Mono.empty();
                    }
                })
                .onErrorResume(error -> {
                    log.warn("Failed to fetch cover for color extraction: {}", resolvedUrl, error);
                    return Mono.empty();
                });
    }

    private String resolveCoverUrl(String coverUrl) {
        URI uri = URI.create(coverUrl);
        if (uri.isAbsolute()) {
            return coverUrl;
        }
        String baseUrl = appProperties.getBaseUrl();
        if (!StringUtils.hasText(baseUrl)) {
            baseUrl = "http://127.0.0.1:8080";
        }
        return baseUrl.replaceAll("/+$", "") + (coverUrl.startsWith("/") ? coverUrl : "/" + coverUrl);
    }

    private boolean isSafeCoverUrl(String url) {
        try {
            URI uri = URI.create(url);
            String scheme = uri.getScheme();
            String host = uri.getHost();
            if (!StringUtils.hasText(scheme) || !ALLOWED_SCHEMES.contains(scheme.toLowerCase())) {
                return false;
            }
            if (!StringUtils.hasText(host)) {
                return false;
            }
            if ("localhost".equalsIgnoreCase(host) || host.endsWith(".localhost")) {
                return false;
            }

            for (InetAddress address : InetAddress.getAllByName(host)) {
                if (!isPublicAddress(address)) {
                    return false;
                }
            }
            return true;
        } catch (IllegalArgumentException | UnknownHostException e) {
            return false;
        }
    }

    private boolean isTrustedLocalCoverPath(String coverUrl) {
        if (!StringUtils.hasText(coverUrl)) {
            return false;
        }
        String trimmed = coverUrl.trim();
        return trimmed.startsWith("/api/navidrome/cover/")
                || (trimmed.startsWith("/api/subsonic/") && trimmed.contains("/cover/"))
                || trimmed.startsWith("/media/");
    }

    private String resolveRedirectUrl(String originalUrl, String location) {
        if (!StringUtils.hasText(location)) {
            return "";
        }
        try {
            return URI.create(originalUrl).resolve(location).toString();
        } catch (IllegalArgumentException e) {
            return "";
        }
    }

    private boolean isPublicAddress(InetAddress address) {
        return !(address.isAnyLocalAddress()
                || address.isLoopbackAddress()
                || address.isLinkLocalAddress()
                || address.isSiteLocalAddress()
                || address.isMulticastAddress()
                || isCloudMetadataAddress(address));
    }

    private boolean isCloudMetadataAddress(InetAddress address) {
        byte[] bytes = address.getAddress();
        return bytes.length == 4
                && (bytes[0] & 0xFF) == 169
                && (bytes[1] & 0xFF) == 254
                && (bytes[2] & 0xFF) == 169
                && (bytes[3] & 0xFF) == 254;
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
        Color normalized = ensureDarkThemeContrast(base);
        Color hover = shift(normalized, 0.06f, 0.10f);
        String accent = toHex(normalized);
        String accentHover = toHex(hover);
        String accentMuted = toRgba(normalized, 0.16f);
        String accentSubtle = toRgba(normalized, 0.08f);
        return new CoverColorResponse(accent, accentHover, accentMuted, accentSubtle);
    }

    private Color ensureDarkThemeContrast(Color base) {
        float[] hsb = Color.RGBtoHSB(base.getRed(), base.getGreen(), base.getBlue(), null);
        float saturation = Math.max(0.24f, Math.min(0.82f, hsb[1]));
        float brightness = Math.max(0.64f, Math.min(0.90f, hsb[2]));
        Color adjusted = Color.getHSBColor(hsb[0], saturation, brightness);

        int guard = 0;
        while (relativeLuminance(adjusted) < 0.32 && guard < 6) {
            adjusted = mix(adjusted, Color.WHITE, 0.16f);
            guard++;
        }

        return adjusted;
    }

    private double relativeLuminance(Color color) {
        return 0.2126 * channelLuminance(color.getRed())
                + 0.7152 * channelLuminance(color.getGreen())
                + 0.0722 * channelLuminance(color.getBlue());
    }

    private double channelLuminance(int channel) {
        double normalized = channel / 255.0;
        return normalized <= 0.03928
                ? normalized / 12.92
                : Math.pow((normalized + 0.055) / 1.055, 2.4);
    }

    private Color mix(Color color, Color target, float amount) {
        int red = Math.round(color.getRed() + (target.getRed() - color.getRed()) * amount);
        int green = Math.round(color.getGreen() + (target.getGreen() - color.getGreen()) * amount);
        int blue = Math.round(color.getBlue() + (target.getBlue() - color.getBlue()) * amount);
        return new Color(red, green, blue);
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
