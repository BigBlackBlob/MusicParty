package org.thornex.musicparty.persistence;

import org.thornex.musicparty.dto.ChatMessage;

import java.util.List;

public interface ChatRepository {
    void appendMessage(String roomId, ChatMessage message);
    List<ChatMessage> fetchMessages(String roomId, int offset, int limit);
    void replaceMessages(String roomId, List<ChatMessage> messages);
    void deleteRoomHistory(String roomId);
}
