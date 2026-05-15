package org.thornex.musicparty.dto;

public record CurrentUserResponse(String sessionToken, String publicId, String name, boolean isGuest) {}
