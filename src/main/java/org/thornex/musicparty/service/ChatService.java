package org.thornex.musicparty.service;

import lombok.extern.slf4j.Slf4j;
import org.springframework.context.event.EventListener;
import org.springframework.messaging.simp.SimpMessagingTemplate;
import org.springframework.stereotype.Service;
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
    
    private final Map<String, ChatCommand> commandMap;
    private final Map<String, Long> lastMessageTime = new java.util.concurrent.ConcurrentHashMap<>();

    public ChatService(SimpMessagingTemplate messagingTemplate,
                       UserService userService,
                       AppProperties appProperties,
                       RoomStatePersistenceService roomStatePersistenceService,
                       List<ChatCommand> commands) {
        this.messagingTemplate = messagingTemplate;
        this.userService = userService;
        this.appProperties = appProperties;
        this.roomStatePersistenceService = roomStatePersistenceService;
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
        ConcurrentLinkedDeque<ChatMessage> history = roomHistory(normalizedRoomId);
        history.addLast(message);
        trimHistory(history);
        roomStatePersistenceService.persistRoomMessage(normalizedRoomId, message);
    }

    public void addMessage(ChatMessage message) {
        addMessage(RoomService.DEFAULT_ROOM_ID, message);
    }

    public void addPublicMessage(ChatMessage message) {
        publicHistoryLoaded = true;
        publicHistory.addLast(message);
        trimHistory(publicHistory);
        roomStatePersistenceService.persistPublicMessage(message);
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
        ConcurrentLinkedDeque<ChatMessage> history = roomHistory(roomId);
        history.clear();
        if (loadedHistory != null) {
            history.addAll(loadedHistory);
        }
        roomStatePersistenceService.replaceRoomMessages(roomId, new ArrayList<>(history));
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
        roomHistory(roomId).clear();
        roomStatePersistenceService.replaceRoomMessages(roomId, List.of());
    }

    public void clearHistory() {
        clearHistory(RoomService.DEFAULT_ROOM_ID);
    }

    public void clearHistoryAndNotify(String roomId) {
        clearHistory(roomId);
        broadcastSystemMessage(roomId, "聊天记录已由管理员清空");
    }

    public void clearHistoryAndNotify() {
        clearHistoryAndNotify(RoomService.DEFAULT_ROOM_ID);
    }

    private void broadcastSystemMessage(String roomId, String content) {
        ChatMessage sysMsg = new ChatMessage(
                UUID.randomUUID().toString(),
                "SYSTEM",
                "SYSTEM",
                content,
                System.currentTimeMillis(),
                MessageType.SYSTEM
        );
        addMessage(roomId, sysMsg);
        messagingTemplate.convertAndSend("/topic/rooms/" + roomId + "/chat", sysMsg);
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
        String roomId = event.getRoomId();

        // 如果是 RESET 事件，清空历史
        if (event.getAction() == PlayerAction.RESET) {
            clearHistory(roomId);
            // 依然发送一条“系统已重置”的消息作为新历史的开始
        }

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

        ChatMessage sysMsg = new ChatMessage(
                UUID.randomUUID().toString(),
                msgUserId,
                msgUserName,
                content,
                System.currentTimeMillis(),
                type
        );

        // 1. 存入历史
        addMessage(roomId, sysMsg);

        // 2. 广播到聊天频道
        messagingTemplate.convertAndSend("/topic/rooms/" + roomId + "/chat", sysMsg);
    }

    public void deleteRoomHistory(String roomId) {
        roomHistories.remove(roomId);
        roomStatePersistenceService.deleteRoomData(roomId);
    }

    private ConcurrentLinkedDeque<ChatMessage> roomHistory(String roomId) {
        String key = roomId == null || roomId.isBlank() ? RoomService.DEFAULT_ROOM_ID : roomId;
        return roomHistories.computeIfAbsent(key, this::loadRoomHistory);
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
