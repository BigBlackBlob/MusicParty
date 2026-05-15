package org.thornex.musicparty.persistence;

import org.thornex.musicparty.dto.ChatMessage;

import java.util.ArrayList;
import java.util.Collections;
import java.util.List;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;

public class InMemoryChatRepository implements ChatRepository {

    private final Map<String, List<ChatMessage>> roomMessages = new ConcurrentHashMap<>();
    private final List<ChatMessage> publicMessages = Collections.synchronizedList(new ArrayList<>());

    @Override
    public void appendMessage(String roomId, ChatMessage message) {
        messageStore(roomId).add(message);
    }

    @Override
    public List<ChatMessage> fetchMessages(String roomId, int offset, int limit) {
        List<ChatMessage> source = messageStore(roomId);
        List<ChatMessage> reversed = new ArrayList<>(source);
        Collections.reverse(reversed);
        if (offset >= reversed.size()) {
            return List.of();
        }
        return new ArrayList<>(reversed.subList(offset, Math.min(offset + limit, reversed.size())));
    }

    @Override
    public void replaceMessages(String roomId, List<ChatMessage> messages) {
        List<ChatMessage> store = messageStore(roomId);
        synchronized (store) {
            store.clear();
            store.addAll(messages);
        }
    }

    @Override
    public void deleteRoomHistory(String roomId) {
        roomMessages.remove(roomId);
    }

    private List<ChatMessage> messageStore(String roomId) {
        if (roomId == null) {
            return publicMessages;
        }
        return roomMessages.computeIfAbsent(roomId, ignored -> Collections.synchronizedList(new ArrayList<>()));
    }
}
