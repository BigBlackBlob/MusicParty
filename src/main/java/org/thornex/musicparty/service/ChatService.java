package org.thornex.musicparty.service;

import lombok.extern.slf4j.Slf4j;
import org.springframework.context.event.EventListener;
import org.springframework.messaging.simp.SimpMessagingTemplate;
import org.springframework.stereotype.Service;
import org.thornex.musicparty.dto.ChatRequest;
import org.thornex.musicparty.dto.ChatMessage;
import org.thornex.musicparty.dto.User;
import org.thornex.musicparty.enums.MessageType;
import org.thornex.musicparty.enums.PlayerAction;
import org.thornex.musicparty.event.RoomSessionEvictedEvent;
import org.thornex.musicparty.event.SystemMessageEvent;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.service.command.ChatCommand;
import org.thornex.musicparty.util.MessageFormatter;

import java.util.*;
import java.util.concurrent.ConcurrentLinkedDeque;
import java.util.function.Function;
import java.util.stream.Collectors;

@Service
@Slf4j
public class ChatService {

    private final Map<String, ConcurrentLinkedDeque<ChatMessage>> roomHistories = new java.util.concurrent.ConcurrentHashMap<>();
    private final ConcurrentLinkedDeque<ChatMessage> publicHistory = new ConcurrentLinkedDeque<>();
    private volatile boolean publicHistoryLoaded = false;
    private final SimpMessagingTemplate messagingTemplate;
    private final UserService userService;
    private final AppProperties appProperties;
    private final RoomStatePersistenceService roomStatePersistenceService;
    private final RoomStateMutationService roomStateMutationService;
    private final RoomSessionCoordinator roomSessionCoordinator;
    private final AfterCommitExecutor afterCommitExecutor;
    
    private final Map<String, ChatCommand> commandMap;
    private final Map<String, Long> lastMessageTime = new java.util.concurrent.ConcurrentHashMap<>();

    public ChatService(SimpMessagingTemplate messagingTemplate,
                       UserService userService,
                       AppProperties appProperties,
                       RoomStatePersistenceService roomStatePersistenceService,
                       RoomStateMutationService roomStateMutationService,
                       RoomSessionCoordinator roomSessionCoordinator,
                       AfterCommitExecutor afterCommitExecutor,
                       List<ChatCommand> commands) {
        this.messagingTemplate = messagingTemplate;
        this.userService = userService;
        this.appProperties = appProperties;
        this.roomStatePersistenceService = roomStatePersistenceService;
        this.roomStateMutationService = roomStateMutationService;
        this.roomSessionCoordinator = roomSessionCoordinator;
        this.afterCommitExecutor = afterCommitExecutor;
        this.commandMap = commands.stream().collect(Collectors.toMap(ChatCommand::getCommand, Function.identity()));
    }

    /**
     * 检查是否允许发送消息（频率限制）
     */
    public boolean canUserSendMessage(String userPublicId) {
        long now = System.currentTimeMillis();
        long last = lastMessageTime.getOrDefault(userPublicId, 0L);
        long minInterval = appProperties.getChat().getMinIntervalMs();

        if (now - last < minInterval) {
            return false;
        }

        lastMessageTime.put(userPublicId, now);
        return true;
    }

    /**
     * 检查消息长度
     */
    public boolean isMessageLengthValid(String content) {
        return content != null && content.length() <= appProperties.getChat().getMaxMessageLength();
    }

    /**
     * 处理传入的消息
     * @return true 如果消息被处理（不广播），false 如果应该继续广播
     */
    public boolean processIncomingMessage(String sessionId, String content) {
        if (content.startsWith("//")) {
            String fullCmd = content.substring(2).trim();
            String[] parts = fullCmd.split("\\s+", 2);
            String cmdKey = parts[0].toLowerCase();
            String args = parts.length > 1 ? parts[1] : "";

            ChatCommand handler = commandMap.get(cmdKey);
            if (handler != null) {
                userService.getUser(sessionId).ifPresent(user -> handler.execute(args, user));
                return true; // 拦截消息
            }
        }
        return false; // 普通消息，继续处理
    }

    public void addMessage(String roomId, ChatMessage message) {
        String normalizedRoomId = roomId == null || roomId.isBlank() ? RoomService.DEFAULT_ROOM_ID : roomId;
        roomSessionCoordinator.markRoomActive(normalizedRoomId);
        roomStateMutationService.runInTransaction(() -> {
            roomStatePersistenceService.persistRoomMessage(normalizedRoomId, message);
            afterCommitExecutor.run(() -> {
                appendToLoadedRoomHistory(normalizedRoomId, message);
                broadcastChatMessage(normalizedRoomId, message);
            });
        });
    }

    public void addMessage(ChatMessage message) {
        addMessage(RoomService.DEFAULT_ROOM_ID, message);
    }

    public void addPublicMessage(ChatMessage message) {
        roomStateMutationService.runInTransaction(() -> {
            roomStatePersistenceService.persistPublicMessage(message);
            afterCommitExecutor.run(() -> {
                publicHistoryLoaded = true;
                publicHistory.addLast(message);
                trimHistory(publicHistory);
                broadcastPublicChatMessage(message);
            });
        });
    }

    public void handleRoomChat(String sessionId, ChatRequest request) {
        if (request == null || request.content() == null) {
            return;
        }
        String normalizedContent = request.content().trim();
        if (normalizedContent.isEmpty() || !isMessageLengthValid(normalizedContent)) {
            return;
        }
        if (processIncomingMessage(sessionId, normalizedContent)) {
            return;
        }

        userService.getUser(sessionId).ifPresent(user -> {
            if (!canUserSendMessage(user.getPublicId())) {
                return;
            }
            String roomId = userService.getRoomIdForSession(sessionId);
            addMessage(roomId, buildChatMessage(user, normalizedContent));
        });
    }

    public void handlePublicChat(String sessionId, ChatRequest request) {
        if (request == null || request.content() == null) {
            return;
        }
        String normalizedContent = request.content().trim();
        if (normalizedContent.isEmpty() || !isMessageLengthValid(normalizedContent)) {
            return;
        }

        userService.getUser(sessionId).ifPresent(user -> {
            if (!canUserSendMessage("public:" + user.getPublicId())) {
                return;
            }
            addPublicMessage(buildChatMessage(user, normalizedContent));
        });
    }

    private void trimHistory(ConcurrentLinkedDeque<ChatMessage> history) {
        while (history.size() > appProperties.getChat().getMaxHistorySize()) {
            history.removeFirst();
        }
    }
// ... existing code ...
    /**
     * 分页获取历史记录 (从最新往旧推)
     * @param offset 跳过最近的多少条
     * @param limit 取多少条
     */
    public List<ChatMessage> getHistory(String roomId, int offset, int limit) {
        // 我们将其转为 List 进行倒序切片处理
        List<ChatMessage> snapshot = new ArrayList<>(roomHistory(roomId));
        Collections.reverse(snapshot);

        if (offset >= snapshot.size()) {
            return Collections.emptyList();
        }

        int end = Math.min(offset + limit, snapshot.size());
        List<ChatMessage> page = snapshot.subList(offset, end);

        Collections.reverse(page);
        return page;
    }

    public List<ChatMessage> getHistory(int offset, int limit) {
        return getHistory(RoomService.DEFAULT_ROOM_ID, offset, limit);
    }

    public List<ChatMessage> getPublicHistory(int offset, int limit) {
        ensurePublicHistoryLoaded();
        return sliceHistory(publicHistory, offset, limit);
    }

    /**
     * 获取全部聊天记录用于持久化
     */
    public List<ChatMessage> getHistoryFull(String roomId) {
        return new ArrayList<>(roomHistory(roomId));
    }

    public List<ChatMessage> getHistoryFull() {
        return getHistoryFull(RoomService.DEFAULT_ROOM_ID);
    }

    public List<ChatMessage> getPublicHistoryFull() {
        ensurePublicHistoryLoaded();
        return new ArrayList<>(publicHistory);
    }

    /**
     * 恢复聊天记录
     */
    public void restore(String roomId, List<ChatMessage> loadedHistory) {
        List<ChatMessage> replacement = loadedHistory == null ? List.of() : new ArrayList<>(loadedHistory);
        replaceRoomHistory(roomId, replacement);
        roomStatePersistenceService.replaceRoomMessages(roomId, replacement);
    }

    public void restore(List<ChatMessage> loadedHistory) {
        restore(RoomService.DEFAULT_ROOM_ID, loadedHistory);
    }

    public void restorePublic(List<ChatMessage> loadedHistory) {
        publicHistory.clear();
        if (loadedHistory != null) {
            publicHistory.addAll(loadedHistory);
        }
        publicHistoryLoaded = true;
        roomStatePersistenceService.replacePublicMessages(new ArrayList<>(publicHistory));
    }

    public void clearHistory(String roomId) {
        replaceRoomHistory(roomId, List.of());
        roomStatePersistenceService.replaceRoomMessages(roomId, List.of());
    }

    public void clearHistory() {
        clearHistory(RoomService.DEFAULT_ROOM_ID);
    }

    public void clearHistoryAndNotify(String roomId) {
        String normalizedRoomId = roomId == null || roomId.isBlank() ? RoomService.DEFAULT_ROOM_ID : roomId;
        ChatMessage sysMsg = buildSystemChatMessage(normalizedRoomId, "SYSTEM", "SYSTEM", "聊天记录已由管理员清空", MessageType.SYSTEM);
        roomSessionCoordinator.markRoomActive(normalizedRoomId);
        roomStateMutationService.runInTransaction(() -> {
            roomStatePersistenceService.replaceRoomMessages(normalizedRoomId, List.of(sysMsg));
            afterCommitExecutor.run(() -> {
                replaceRoomHistory(normalizedRoomId, List.of(sysMsg));
                broadcastChatMessage(normalizedRoomId, sysMsg);
            });
        });
    }

    public void clearHistoryAndNotify() {
        clearHistoryAndNotify(RoomService.DEFAULT_ROOM_ID);
    }

    private void broadcastSystemMessage(String roomId, String content) {
        String normalizedRoomId = roomId == null || roomId.isBlank() ? RoomService.DEFAULT_ROOM_ID : roomId;
        ChatMessage sysMsg = buildSystemChatMessage(normalizedRoomId, "SYSTEM", "SYSTEM", content, MessageType.SYSTEM);
        roomSessionCoordinator.markRoomActive(normalizedRoomId);
        roomStateMutationService.runInTransaction(() -> {
            roomStatePersistenceService.persistRoomMessage(normalizedRoomId, sysMsg);
            afterCommitExecutor.run(() -> {
                appendToLoadedRoomHistory(normalizedRoomId, sysMsg);
                broadcastChatMessage(normalizedRoomId, sysMsg);
            });
        });
    }

    /**
     * 监听系统事件，自动生成系统消息
     * 这里实现了将“操作日志”写入“系统聊天Tab”的需求
     */
    @EventListener
    public void onSystemEvent(SystemMessageEvent event) {
        // 忽略错误提示，只记录操作成功的事件（根据需求调整）
        // 这里我们记录所有 INFO, WARN, SUCCESS 级别的事件，忽略 ERROR (通常 ERROR 只弹 Toast)
        if (event.getLevel() == SystemMessageEvent.Level.ERROR) return;
        String roomId = event.getRoomId() == null || event.getRoomId().isBlank() ? RoomService.DEFAULT_ROOM_ID : event.getRoomId();

        String userName = "SYSTEM";
        if (!"SYSTEM".equals(event.getUserId())) {
            userName = userService.getUserByPublicId(event.getUserId())
                    .map(User::getName)
                    .orElse("Unknown");
        }

        String content = MessageFormatter.format(event, userName);
        MessageType type;

        if (event.getAction() == PlayerAction.LIKE) {
            type = MessageType.LIKE;
        } else if (event.getAction() == PlayerAction.PLAY_START) {
            type = MessageType.PLAY_START;
        } else {
            type = MessageType.SYSTEM;
        }

        String msgUserId = (event.getAction() == PlayerAction.LIKE || event.getAction() == PlayerAction.PLAY_START) ? event.getUserId() : "SYSTEM";
        String msgUserName = (event.getAction() == PlayerAction.LIKE || event.getAction() == PlayerAction.PLAY_START) ? userName : "SYSTEM";

        ChatMessage sysMsg = buildSystemChatMessage(roomId, msgUserId, msgUserName, content, type);
        roomSessionCoordinator.markRoomActive(roomId);

        if (event.getAction() == PlayerAction.RESET) {
            roomStatePersistenceService.replaceRoomMessages(roomId, List.of(sysMsg));
            afterCommitExecutor.run(() -> {
                replaceRoomHistory(roomId, List.of(sysMsg));
                broadcastChatMessage(roomId, sysMsg);
            });
            return;
        }

        roomStatePersistenceService.persistRoomMessage(roomId, sysMsg);
        afterCommitExecutor.run(() -> {
            appendToLoadedRoomHistory(roomId, sysMsg);
            broadcastChatMessage(roomId, sysMsg);
        });
    }

    public void deleteRoomHistory(String roomId) {
        roomHistories.remove(roomId);
        roomStatePersistenceService.deleteRoomData(roomId);
    }

    public void evictRoomHistory(String roomId) {
        roomHistories.remove(roomId);
    }

    private ConcurrentLinkedDeque<ChatMessage> roomHistory(String roomId) {
        String key = roomId == null || roomId.isBlank() ? RoomService.DEFAULT_ROOM_ID : roomId;
        return roomHistories.computeIfAbsent(key, this::loadRoomHistory);
    }

    private void appendToRoomHistory(String roomId, ChatMessage message) {
        ConcurrentLinkedDeque<ChatMessage> history = roomHistory(roomId);
        history.addLast(message);
        trimHistory(history);
    }

    private void appendToLoadedRoomHistory(String roomId, ChatMessage message) {
        String key = roomId == null || roomId.isBlank() ? RoomService.DEFAULT_ROOM_ID : roomId;
        ConcurrentLinkedDeque<ChatMessage> history = roomHistories.get(key);
        if (history == null) {
            return;
        }
        history.addLast(message);
        trimHistory(history);
    }

    private void replaceRoomHistory(String roomId, List<ChatMessage> messages) {
        ConcurrentLinkedDeque<ChatMessage> history = roomHistory(roomId);
        history.clear();
        history.addAll(messages);
    }

    private ChatMessage buildSystemChatMessage(String roomId, String userId, String userName, String content, MessageType type) {
        return new ChatMessage(
                UUID.randomUUID().toString(),
                userId,
                userName,
                content,
                System.currentTimeMillis(),
                type
        );
    }

    private ChatMessage buildChatMessage(User user, String content) {
        return new ChatMessage(
                UUID.randomUUID().toString(),
                user.getPublicId(),
                user.getName(),
                content,
                System.currentTimeMillis(),
                MessageType.CHAT
        );
    }

    private void broadcastChatMessage(String roomId, ChatMessage message) {
        if (messagingTemplate == null) {
            return;
        }
        messagingTemplate.convertAndSend("/topic/rooms/" + roomId + "/chat", message);
    }

    private void broadcastPublicChatMessage(ChatMessage message) {
        if (messagingTemplate == null) {
            return;
        }
        messagingTemplate.convertAndSend("/topic/public/chat", message);
    }

    @EventListener
    public void onRoomSessionEvicted(RoomSessionEvictedEvent event) {
        if (!RoomService.DEFAULT_ROOM_ID.equals(event.getRoomId())) {
            roomHistories.remove(event.getRoomId());
        }
    }

    private ConcurrentLinkedDeque<ChatMessage> loadRoomHistory(String roomId) {
        ConcurrentLinkedDeque<ChatMessage> history = new ConcurrentLinkedDeque<>();
        history.addAll(roomStatePersistenceService.loadRoomMessages(roomId, appProperties.getChat().getMaxHistorySize()));
        return history;
    }

    private void ensurePublicHistoryLoaded() {
        if (publicHistoryLoaded) {
            return;
        }
        List<ChatMessage> loaded = roomStatePersistenceService.loadPublicMessages(appProperties.getChat().getMaxHistorySize());
        publicHistory.clear();
        publicHistory.addAll(loaded);
        publicHistoryLoaded = true;
    }

    private List<ChatMessage> sliceHistory(ConcurrentLinkedDeque<ChatMessage> history, int offset, int limit) {
        List<ChatMessage> snapshot = new ArrayList<>(history);
        Collections.reverse(snapshot);
        if (offset >= snapshot.size()) {
            return Collections.emptyList();
        }
        int end = Math.min(offset + limit, snapshot.size());
        List<ChatMessage> page = snapshot.subList(offset, end);
        Collections.reverse(page);
        return page;
    }
}
