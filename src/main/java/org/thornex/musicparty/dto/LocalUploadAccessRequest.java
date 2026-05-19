package org.thornex.musicparty.dto;

public record LocalUploadAccessRequest(
        String adminPassword,
        String userName
) {
}
