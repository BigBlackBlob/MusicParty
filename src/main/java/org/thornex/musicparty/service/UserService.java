package org.thornex.musicparty.service;

import lombok.extern.slf4j.Slf4j;
import org.springframework.context.ApplicationEventPublisher;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Service;
import org.springframework.util.StringUtils;
import org.thornex.musicparty.dto.User;
import org.thornex.musicparty.dto.UserSummary;
import org.thornex.musicparty.enums.PlayerAction;
import org.thornex.musicparty.event.SystemMessageEvent;
import org.thornex.musicparty.persistence.PersistedSession;
import org.thornex.musicparty.persistence.PersistedUserProfile;
import org.thornex.musicparty.persistence.UserProfileRepository;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.*;
import java.util.concurrent.*;
import java.util.stream.Collectors;

@Service
@Slf4j
public class UserService {

    // 主存储：服务端签发的私有 Session Token -> User
    private final Map<String, User> usersBySessionToken = new ConcurrentHashMap<>();
    private final Map<String, User> usersByPublicId = new ConcurrentHashMap<>();

    // 辅助索引：SessionId -> Session Token (用于快速查找当前发消息的是谁)
    private final Map<String, String> sessionToToken = new ConcurrentHashMap<>();

    private final ApplicationEventPublisher eventPublisher;
    private final RoomService roomService;
    private final RoomSessionCoordinator roomSessionCoordinator;
    private final UserProfileRepository userProfileRepository;

    // 延迟任务调度器，用于处理断连抖动
    private final ScheduledExecutorService scheduler = Executors.newSingleThreadScheduledExecutor();
    private final Map<String, ScheduledFuture<?>> pendingLeaveEvents = new ConcurrentHashMap<>();

    private static final long USER_EXPIRATION_MS = 1 * 60 * 60 * 1000L;
    private static final long LEAVE_DELAY_SEC = 10; // 10秒延迟判定真正离开

    public UserService(ApplicationEventPublisher eventPublisher,
                       RoomService roomService,
                       RoomSessionCoordinator roomSessionCoordinator,
                       UserProfileRepository userProfileRepository) {
        this.eventPublisher = eventPublisher;
        this.roomService = roomService;
        this.roomSessionCoordinator = roomSessionCoordinator;
        this.userProfileRepository = userProfileRepository;
        this.roomService.setOnlineCountProvider(this::getOnlineCount);
    }

    /**
     * 处理连接
     * @param sessionId WebSocket Session ID
     * @param sessionTokenFront 前端传来的服务端签发 Session Token (可能为空)
     * @param nameFront 前端传来的名字 (可能为空)
     * @return 最终确定的 User 对象
     */
    public User handleConnect(String sessionId, String sessionTokenFront, String nameFront, String requestedRoomId) {
        User user;
        long now = System.currentTimeMillis();

        // 1. 尝试找回老用户
        if (StringUtils.hasText(sessionTokenFront) && usersBySessionToken.containsKey(sessionTokenFront)) {
            user = usersBySessionToken.get(sessionTokenFront);

            // 🟢 检查是否有待执行的“离开”任务，如果有，说明是快速重连，直接取消
            ScheduledFuture<?> pendingLeave = pendingLeaveEvents.remove(user.getSessionToken());
            if (pendingLeave != null) {
                pendingLeave.cancel(false);
                log.info("User {} reconnected quickly, suppressed leave/join logs.", user.getName());
            } else {
                // 如果没有待执行任务，且用户之前是离线状态，且不是游客，则发布加入日志
                if (user.getSessionId() == null && !user.isGuest()) {
                    eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.INFO, PlayerAction.USER_JOIN, user.getPublicId(), null, user.getRoomId()));
                }
            }

            log.info("User Reconnected: {} (PublicId: {}) -> New Session: {}", user.getName(), user.getPublicId(), sessionId);
            // ... (保持原有逻辑)
            if (user.getSessionId() != null) {
                sessionToToken.remove(user.getSessionId());
            }
            user.setSessionId(sessionId);
        }
        else if (StringUtils.hasText(sessionTokenFront)) {
            user = restorePersistedUser(sessionTokenFront, sessionId).orElse(null);
            if (user != null) {
                log.info("Persisted User Restored: {} (PublicId: {}) -> Session: {}", user.getName(), user.getPublicId(), sessionId);
            } else {
                user = registerNewUser(sessionId, nameFront);
            }
        }
        // 2. 新用户注册
        else {
            user = registerNewUser(sessionId, nameFront);
        }

        String previousRoomId = roomService.normalizeRoomId(user.getRoomId());
        String roomId = StringUtils.hasText(requestedRoomId)
                ? roomService.normalizeRoomId(requestedRoomId)
                : previousRoomId;
        user.setRoomId(roomId);
        user.setLastActiveTime(now);
        sessionToToken.put(sessionId, user.getSessionToken());
        persistUser(user, now);
        if (previousRoomId.equals(roomId)) {
            roomSessionCoordinator.onUserEnteredRoom(roomId, getOnlineUserSummaries(roomId).size());
        } else {
            roomSessionCoordinator.onUserMovedRooms(
                    previousRoomId,
                    roomId,
                    getOnlineUserSummaries(previousRoomId).size(),
                    getOnlineUserSummaries(roomId).size()
            );
        }
        return user;
    }

    public User handleConnect(String sessionId, String sessionTokenFront, String nameFront) {
        return handleConnect(sessionId, sessionTokenFront, nameFront, null);
    }

    public Optional<User> disconnectUser(String sessionId) {
        String token = sessionToToken.remove(sessionId);
        if (token == null) return Optional.empty();

        User user = usersBySessionToken.get(token);
        if (user != null) {
            // 🟢 关键修复：多标签页支持
            // 只有当断开的 Session ID 等于用户当前的主 Session ID 时，才认为用户真的掉线了
            // 如果不等，说明用户已经连接了新的 Session (比如打开了新标签页，关闭了旧标签页)，此时忽略旧连接的断开
            if (sessionId.equals(user.getSessionId())) {
                String roomId = user.getRoomId();
                user.setSessionId(null); // 标记离线
                user.setLastActiveTime(System.currentTimeMillis());
                persistUser(user, user.getLastActiveTime());
                log.info("User Offline (Pending Confirmation): {}", user.getName());

                // 延迟发送离开日志
                if (!user.isGuest()) {
                    String sessionToken = user.getSessionToken();
                    ScheduledFuture<?> future = scheduler.schedule(() -> {
                        pendingLeaveEvents.remove(sessionToken);
                        log.info("User Leave Confirmed: {}", user.getName());
                        eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.INFO, PlayerAction.USER_LEAVE, user.getPublicId(), null, roomId));
                    }, LEAVE_DELAY_SEC, TimeUnit.SECONDS);
                    pendingLeaveEvents.put(sessionToken, future);
                }
                roomSessionCoordinator.onUserDisconnected(roomId, getOnlineUserSummaries(roomId).size());
                return Optional.of(user);
            } else {
                log.debug("Ignored disconnect for stale session {} (Current: {})", sessionId, user.getSessionId());
            }
        }
        return Optional.empty();
    }

    public Optional<User> getUserBySession(String sessionId) {
        String token = sessionToToken.get(sessionId);
        if (token == null) return Optional.empty();
        return Optional.ofNullable(usersBySessionToken.get(token));
    }

    public Optional<User> getUser(String sessionId) {
        return getUserBySession(sessionId);
    }

    // 🟢 改名逻辑：增加查重
    public boolean renameUser(String sessionId, String newName) {
        return getUserBySession(sessionId).map(user -> {
            String rawName = newName.trim();
            // 使用一个新的变量 finalName，确保它不被修改
            String finalName = rawName.length() > 20 ? rawName.substring(0, 20) : rawName;

            if (finalName.isEmpty()) return false;

            // 禁止伪装成 游客
            if (finalName.toLowerCase().startsWith("guest") || finalName.startsWith("游客")) {
                log.warn("Rename failed: Cannot use reserved name '{}'", finalName);
                return false;
            }

            // 检查是否重名 (排除自己)
            boolean exists = usersBySessionToken.values().stream()
                    .anyMatch(u -> user.getRoomId().equals(u.getRoomId()) && u.getName().equalsIgnoreCase(finalName) && !u.getPublicId().equals(user.getPublicId()));

            if (exists) {
                log.warn("Rename failed: {} is already taken.", finalName);
                return false;
            }

            String oldName = user.getName();
            boolean wasGuest = user.isGuest();

            log.info("User Renamed: '{}' -> '{}'", oldName, finalName);
            user.setName(finalName);
            user.setGuest(false); // 改名成功，移除游客身份
            persistUser(user, System.currentTimeMillis());

            // 1. 如果是从游客变成正式用户 -> 发布加入事件
            if (wasGuest) {
                eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.INFO, PlayerAction.USER_JOIN, user.getPublicId(), null, user.getRoomId()));
            }
            // 2. 如果是正式用户改名 -> 发布系统通知
            else if (!oldName.equals(finalName)) {
                String renameMsg = oldName + " 已更名为 " + finalName;
                eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.INFO, null, "SYSTEM", renameMsg, user.getRoomId()));
            }

            return true;
        }).orElse(false);
    }

    // 辅助：名字去重
    private String deduplicateName(String name) {
        String finalName = name;
        int counter = 1;
        while (isNameTaken(finalName)) {
            finalName = name + "_" + counter++;
        }
        return finalName;
    }

    private boolean isNameTaken(String name) {
        return usersBySessionToken.values().stream().anyMatch(u -> u.getName().equalsIgnoreCase(name));
    }

    public boolean bindAccount(String sessionId, String platform, String accountId) {
        return getUserBySession(sessionId).map(user -> {
            Map<String, String> updatedBindings = new HashMap<>(user.getBindings());
            updatedBindings.put(platform, accountId);
            userProfileRepository.replaceBindings(user.getPublicId(), updatedBindings);
            user.getBindings().clear();
            user.getBindings().putAll(updatedBindings);
            return true;
        }).orElse(false);
    }

    public List<UserSummary> getOnlineUserSummaries(String roomId) {
        return usersBySessionToken.values().stream()
                // 只返回在线用户 (sessionId != null)
                .filter(u -> u.getSessionId() != null)
                .filter(u -> roomService.normalizeRoomId(roomId).equals(u.getRoomId()))
                .map(user -> new UserSummary(user.getPublicId(), user.getName(), user.isGuest()))
                .toList();
    }

    public List<UserSummary> getOnlineUserSummaries() {
        return getOnlineUserSummaries(RoomService.DEFAULT_ROOM_ID);
    }

    /**
     * 获取最近活跃的 public id (包括当前在线和正在等待断连确认的用户)
     */
    public Set<String> getRecentlyActivePublicIds(String roomId) {
        String normalizedRoomId = roomService.normalizeRoomId(roomId);
        return usersBySessionToken.values().stream()
                .filter(u -> u.getSessionId() != null || pendingLeaveEvents.containsKey(u.getSessionToken()))
                .filter(u -> normalizedRoomId.equals(u.getRoomId()))
                .map(User::getPublicId)
                .collect(Collectors.toSet());
    }

    public Set<String> getRecentlyActivePublicIds() {
        return getRecentlyActivePublicIds(RoomService.DEFAULT_ROOM_ID);
    }

    public String getRoomIdForSession(String sessionId) {
        return getUser(sessionId).map(User::getRoomId).orElse(RoomService.DEFAULT_ROOM_ID);
    }

    public int getOnlineCount(String roomId) {
        return getOnlineUserSummaries(roomId).size();
    }

    public List<String> moveUsersToDefaultRoom(String roomId) {
        String normalized = roomId == null || roomId.isBlank() ? RoomService.DEFAULT_ROOM_ID : roomId;
        List<String> movedSessions = new ArrayList<>();
        usersBySessionToken.values().forEach(user -> {
            if (normalized.equals(user.getRoomId())) {
                String oldSessionId = user.getSessionId();
                user.setRoomId(RoomService.DEFAULT_ROOM_ID);
                persistUser(user, System.currentTimeMillis());
                if (oldSessionId != null) {
                    movedSessions.add(oldSessionId);
                }
            }
        });
        roomSessionCoordinator.onUsersMovedToRoom(
                normalized,
                RoomService.DEFAULT_ROOM_ID,
                getOnlineUserSummaries(normalized).size(),
                getOnlineUserSummaries(RoomService.DEFAULT_ROOM_ID).size()
        );
        return movedSessions;
    }

    public void movePersistedUsersToDefaultRoom(String roomId) {
        String normalized = roomId == null || roomId.isBlank() ? RoomService.DEFAULT_ROOM_ID : roomId;
        userProfileRepository.moveUsersToRoom(normalized, RoomService.DEFAULT_ROOM_ID);
    }

    @Scheduled(fixedRate = 3600000)
    public void cleanupExpiredUsers() {
        long now = System.currentTimeMillis();
        int initialSize = usersBySessionToken.size();

        // removeIf 是线程安全的 (ConcurrentHashMap)
        usersBySessionToken.entrySet().removeIf(entry -> {
            User user = entry.getValue();
            boolean isOffline = user.getSessionId() == null;
            boolean isExpired = (now - user.getLastActiveTime()) > USER_EXPIRATION_MS;

            if (isOffline && isExpired) {
                log.debug("Cleaning up expired user: {} (PublicId: {})", user.getName(), user.getPublicId());
                usersByPublicId.remove(user.getPublicId());
                return true; // 删除
            }
            return false; // 保留
        });

        int finalSize = usersBySessionToken.size();
        if (initialSize != finalSize) {
            log.info("Cleanup Complete. Removed {} expired users. Current memory users: {}", (initialSize - finalSize), finalSize);
        }
    }

    public Optional<User> getUserBySessionToken(String sessionToken) {
        return Optional.ofNullable(usersBySessionToken.get(sessionToken));
    }

    public Optional<String> resolvePublicIdBySessionToken(String sessionToken) {
        if (!StringUtils.hasText(sessionToken)) {
            return Optional.empty();
        }
        User inMemory = usersBySessionToken.get(sessionToken);
        if (inMemory != null) {
            return Optional.of(inMemory.getPublicId());
        }
        return userProfileRepository.findSessionByHash(hashSessionToken(sessionToken))
                .map(PersistedSession::publicId);
    }

    public Optional<User> getUserByPublicId(String publicId) {
        return Optional.ofNullable(usersByPublicId.get(publicId));
    }

    private String generatePublicId() {
        String publicId;
        do {
            publicId = "u_" + UUID.randomUUID().toString().replace("-", "").substring(0, 12);
        } while (usersByPublicId.containsKey(publicId));
        return publicId;
    }

    private User registerNewUser(String sessionId, String nameFront) {
        String newSessionToken = UUID.randomUUID().toString();
        String publicId = generatePublicId();
        String initialName = StringUtils.hasText(nameFront) ? nameFront : "游客";
        initialName = deduplicateName(initialName);

        User user = new User(newSessionToken, publicId, sessionId, initialName);
        usersBySessionToken.put(newSessionToken, user);
        usersByPublicId.put(publicId, user);
        log.info("New User Registered: {} (PublicId: {})", initialName, publicId);
        return user;
    }

    private Optional<User> restorePersistedUser(String sessionToken, String sessionId) {
        return userProfileRepository.findSessionByHash(hashSessionToken(sessionToken))
                .flatMap(session -> userProfileRepository.findByPublicId(session.publicId())
                        .map(profile -> toUser(profile, sessionToken, sessionId)));
    }

    private User toUser(PersistedUserProfile profile, String sessionToken, String sessionId) {
        User user = new User(sessionToken, profile.publicId(), sessionId, profile.displayName());
        user.setGuest(profile.guest());
        user.setRoomId(roomService.normalizeRoomId(profile.currentRoomId()));
        user.setLastActiveTime(profile.lastSeenAt());
        user.getBindings().putAll(userProfileRepository.findBindingsByPublicId(profile.publicId()));
        usersBySessionToken.put(sessionToken, user);
        usersByPublicId.put(profile.publicId(), user);
        return user;
    }

    private void persistUser(User user, long timestamp) {
        long profileCreatedAt = userProfileRepository.findByPublicId(user.getPublicId())
                .map(PersistedUserProfile::createdAt)
                .orElse(timestamp);
        long sessionCreatedAt = userProfileRepository.findSessionByHash(hashSessionToken(user.getSessionToken()))
                .map(PersistedSession::createdAt)
                .orElse(timestamp);
        userProfileRepository.upsertProfile(new PersistedUserProfile(
                user.getPublicId(),
                user.getName(),
                user.isGuest(),
                roomService.normalizeRoomId(user.getRoomId()),
                profileCreatedAt,
                timestamp
        ));
        userProfileRepository.upsertSession(new PersistedSession(
                hashSessionToken(user.getSessionToken()),
                user.getPublicId(),
                sessionCreatedAt,
                timestamp
        ));
    }

    private String hashSessionToken(String sessionToken) {
        try {
            MessageDigest digest = MessageDigest.getInstance("SHA-256");
            byte[] bytes = digest.digest(sessionToken.getBytes(StandardCharsets.UTF_8));
            StringBuilder builder = new StringBuilder(bytes.length * 2);
            for (byte value : bytes) {
                builder.append(String.format("%02x", value));
            }
            return builder.toString();
        } catch (NoSuchAlgorithmException e) {
            throw new IllegalStateException("SHA-256 is unavailable", e);
        }
    }
}
