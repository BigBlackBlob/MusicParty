package org.thornex.musicparty.security;

import jakarta.servlet.http.HttpServletRequest;
import org.springframework.stereotype.Component;
import org.springframework.util.StringUtils;
import org.thornex.musicparty.config.AppProperties;

import java.net.InetAddress;
import java.net.UnknownHostException;
import java.util.ArrayList;
import java.util.List;

@Component
public class ClientIpResolver {
    private final List<CidrBlock> trustedProxyCidrs;

    public ClientIpResolver(AppProperties appProperties) {
        this.trustedProxyCidrs = parseCidrs(appProperties.getAuth().getTrustedProxyCidrs());
    }

    public String resolve(HttpServletRequest request) {
        String remoteAddr = cleanAddress(request.getRemoteAddr());
        if (!isTrustedProxy(remoteAddr)) {
            return remoteAddr;
        }

        String forwardedFor = request.getHeader("X-Forwarded-For");
        if (StringUtils.hasText(forwardedFor)) {
            for (String candidate : forwardedFor.split(",")) {
                String cleaned = cleanAddress(candidate);
                if (StringUtils.hasText(cleaned) && !"unknown".equalsIgnoreCase(cleaned)) {
                    return cleaned;
                }
            }
        }

        String realIp = cleanAddress(request.getHeader("X-Real-IP"));
        return StringUtils.hasText(realIp) && !"unknown".equalsIgnoreCase(realIp) ? realIp : remoteAddr;
    }

    private boolean isTrustedProxy(String remoteAddr) {
        if (!StringUtils.hasText(remoteAddr) || trustedProxyCidrs.isEmpty()) {
            return false;
        }
        try {
            InetAddress address = InetAddress.getByName(remoteAddr);
            return trustedProxyCidrs.stream().anyMatch(block -> block.contains(address));
        } catch (UnknownHostException ignored) {
            return false;
        }
    }

    private static String cleanAddress(String value) {
        if (!StringUtils.hasText(value)) {
            return "";
        }
        String trimmed = value.trim();
        int zoneIndex = trimmed.indexOf('%');
        return zoneIndex >= 0 ? trimmed.substring(0, zoneIndex) : trimmed;
    }

    private static List<CidrBlock> parseCidrs(String raw) {
        if (!StringUtils.hasText(raw)) {
            return List.of();
        }
        List<CidrBlock> blocks = new ArrayList<>();
        for (String part : raw.split(",")) {
            String cidr = part.trim();
            if (!cidr.isEmpty()) {
                blocks.add(CidrBlock.parse(cidr));
            }
        }
        return List.copyOf(blocks);
    }

    private record CidrBlock(byte[] network, int prefixLength) {
        static CidrBlock parse(String cidr) {
            String[] parts = cidr.split("/", 2);
            try {
                InetAddress address = InetAddress.getByName(parts[0].trim());
                int maxBits = address.getAddress().length * 8;
                int prefix = parts.length == 2 ? Integer.parseInt(parts[1].trim()) : maxBits;
                if (prefix < 0 || prefix > maxBits) {
                    throw new IllegalArgumentException("Invalid CIDR prefix: " + cidr);
                }
                return new CidrBlock(address.getAddress(), prefix);
            } catch (UnknownHostException | NumberFormatException e) {
                throw new IllegalArgumentException("Invalid trusted proxy CIDR: " + cidr, e);
            }
        }

        boolean contains(InetAddress address) {
            byte[] target = address.getAddress();
            if (target.length != network.length) {
                return false;
            }
            int fullBytes = prefixLength / 8;
            int remainingBits = prefixLength % 8;
            for (int i = 0; i < fullBytes; i++) {
                if (target[i] != network[i]) {
                    return false;
                }
            }
            if (remainingBits == 0) {
                return true;
            }
            int mask = 0xFF << (8 - remainingBits);
            return (target[fullBytes] & mask) == (network[fullBytes] & mask);
        }
    }
}
