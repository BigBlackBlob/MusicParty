package org.thornex.musicparty.dto;

public record AdminCommandRequest(String password, String sessionToken, String command, String roomId) {}
