package org.thornex.musicparty.service;

import org.junit.jupiter.api.Test;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.dto.ChatMessage;
import org.thornex.musicparty.enums.MessageType;
import org.thornex.musicparty.enums.PlayerAction;
import org.thornex.musicparty.event.SystemMessageEvent;
import org.thornex.musicparty.persistence.InMemoryChatRepository;
import org.thornex.musicparty.persistence.InMemoryPlaybackStateRepository;
import org.thornex.musicparty.persistence.InMemoryQueueRepository;

import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;

class ChatServiceTests {

    @Test
    void resetSystemEventReplacesRoomHistoryInsteadOfAppendingDuplicateSystemMessages() {
        ChatService chatService = createChatService();
        String roomId = "room-1";

        chatService.addMessage(roomId, new ChatMessage("msg-1", "user-1", "Alice", "hello", 1L, MessageType.CHAT));

        chatService.onSystemEvent(new SystemMessageEvent(
                this,
                SystemMessageEvent.Level.WARN,
                PlayerAction.RESET,
                "SYSTEM",
                null,
                roomId
        ));

        List<ChatMessage> history = chatService.getHistoryFull(roomId);
        assertThat(history).hasSize(1);
        assertThat(history.getFirst().type()).isEqualTo(MessageType.SYSTEM);
        assertThat(history.getFirst().content()).isEqualTo("系统已被重置");
    }

    @Test
    void clearHistoryAndNotifyLeavesSingleSystemMessageAsNewHistoryStart() {
        ChatService chatService = createChatService();
        String roomId = "room-2";

        chatService.addMessage(roomId, new ChatMessage("msg-1", "user-1", "Alice", "hello", 1L, MessageType.CHAT));
        chatService.addMessage(roomId, new ChatMessage("msg-2", "user-2", "Bob", "world", 2L, MessageType.CHAT));

        chatService.clearHistoryAndNotify(roomId);

        List<ChatMessage> history = chatService.getHistoryFull(roomId);
        assertThat(history).hasSize(1);
        assertThat(history.getFirst().type()).isEqualTo(MessageType.SYSTEM);
        assertThat(history.getFirst().content()).isEqualTo("聊天记录已由管理员清空");
    }

    private ChatService createChatService() {
        AppProperties properties = new AppProperties();
        return new ChatService(
                null,
                null,
                properties,
                new RoomStatePersistenceService(
                        new InMemoryQueueRepository(),
                        new InMemoryChatRepository(),
                        new InMemoryPlaybackStateRepository()
                ),
                new RoomStateMutationService(new TestTransactionManager()),
                new AfterCommitExecutor(),
                List.of()
        );
    }
}
