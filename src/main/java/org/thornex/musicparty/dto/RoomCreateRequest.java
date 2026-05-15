package org.thornex.musicparty.dto;

public record RoomCreateRequest(String name, Boolean isPrivate, String password) {
}
