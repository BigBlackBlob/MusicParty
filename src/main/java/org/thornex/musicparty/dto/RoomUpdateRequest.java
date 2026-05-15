package org.thornex.musicparty.dto;

public record RoomUpdateRequest(
        String sessionToken,
        String name,
        Boolean isPrivate,
        String password,
        Boolean keepExistingPassword,
        String adminPassword
) {
}
