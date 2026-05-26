package org.thornex.musicparty.websocket;

public record SocketEnvelope(String type, String requestId, String roomId, Object payload) {
}
