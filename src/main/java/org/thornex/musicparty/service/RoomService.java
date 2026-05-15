package org.thornex.musicparty.service;

import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.ObjectMapper;
import jakarta.annotation.PostConstruct;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.context.ApplicationEventPublisher;
import org.springframework.stereotype.Service;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.dto.RoomInfo;
import org.thornex.musicparty.event.RoomListUpdateEvent;

import java.io.File;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.ConcurrentHashMap;
import java.util.function.ToIntFunction;

@Service
@RequiredArgsConstructor
@Slf4j
public class RoomService {
    public static final String DEFAULT_ROOM_ID = "lounge";
    private static final String DEFAULT_ROOM_NAME = "Lounge";
    private static final String ROOMS_FILE = "data/rooms.json";

    private final ObjectMapper objectMapper;
    private final ApplicationEventPublisher eventPublisher;
    private final AppProperties appProperties;
    private final Map<String, StoredRoom> rooms = new ConcurrentHashMap<>();
    private volatile ToIntFunction<String> onlineCountProvider = roomId -> 0;

    @PostConstruct
    public void init() {
        loadRooms();
        ensureDefaultRoom();
    }

    public void setOnlineCountProvider(ToIntFunction<String> provider) {
        this.onlineCountProvider = provider == null ? roomId -> 0 : provider;
    }

    public String normalizeRoomId(String roomId) {
        if (roomId == null || roomId.isBlank() || !rooms.containsKey(roomId)) {
            return DEFAULT_ROOM_ID;
        }
        return roomId;
    }

    public List<RoomInfo> listRooms() {
        ensureDefaultRoom();
        return rooms.values().stream()
                .sorted(Comparator.comparing(StoredRoom::system).reversed().thenComparing(StoredRoom::createdAt))
                .map(this::toInfo)
                .toList();
    }

    public RoomInfo createRoom(String rawName, String creatorPublicId) {
        String name = sanitizeName(rawName);
        if (name.isEmpty()) {
            throw new IllegalArgumentException("Room name cannot be empty");
        }
        boolean exists = rooms.values().stream().anyMatch(room -> room.name().equalsIgnoreCase(name));
        if (exists) {
            throw new IllegalArgumentException("Room name already exists");
        }

        String roomId;
        do {
            roomId = "room-" + UUID.randomUUID().toString().replace("-", "").substring(0, 10);
        } while (rooms.containsKey(roomId));

        StoredRoom room = new StoredRoom(roomId, name, creatorPublicId, System.currentTimeMillis(), false);
        rooms.put(roomId, room);
        saveRooms();
        publishRoomList();
        return toInfo(room);
    }

    public boolean canDelete(String roomId, String requesterPublicId, boolean admin) {
        StoredRoom room = rooms.get(roomId);
        if (room == null || room.system()) return false;
        return admin || (requesterPublicId != null && requesterPublicId.equals(room.creatorPublicId()));
    }

    public RoomInfo getRoom(String roomId) {
        return toInfo(rooms.get(normalizeRoomId(roomId)));
    }

    public boolean deleteRoom(String roomId, String requesterPublicId, boolean admin) {
        if (!canDelete(roomId, requesterPublicId, admin)) {
            return false;
        }
        rooms.remove(roomId);
        saveRooms();
        publishRoomList();
        return true;
    }

    public boolean isAdminPassword(String value) {
        String adminPassword = appProperties.getAdminPassword();
        return adminPassword != null && !adminPassword.isBlank() && adminPassword.equals(value);
    }

    public void publishRoomList() {
        eventPublisher.publishEvent(new RoomListUpdateEvent(this, listRooms()));
    }

    private RoomInfo toInfo(StoredRoom room) {
        if (room == null) {
            room = rooms.get(DEFAULT_ROOM_ID);
        }
        return new RoomInfo(room.roomId(), room.name(), room.creatorPublicId(), room.createdAt(), room.system(), onlineCountProvider.applyAsInt(room.roomId()));
    }

    private String sanitizeName(String rawName) {
        if (rawName == null) return "";
        String trimmed = rawName.trim().replaceAll("\\s+", " ");
        if (trimmed.length() > 32) {
            trimmed = trimmed.substring(0, 32).trim();
        }
        return trimmed;
    }

    private void ensureDefaultRoom() {
        rooms.putIfAbsent(DEFAULT_ROOM_ID, new StoredRoom(DEFAULT_ROOM_ID, DEFAULT_ROOM_NAME, "SYSTEM", 0L, true));
    }

    private void loadRooms() {
        File file = new File(ROOMS_FILE);
        if (!file.exists()) return;
        try {
            List<StoredRoom> loaded = objectMapper.readValue(file, new TypeReference<List<StoredRoom>>() {});
            for (StoredRoom room : loaded) {
                if (room != null && room.roomId() != null && !room.roomId().isBlank() && !room.system()) {
                    rooms.put(room.roomId(), room);
                }
            }
        } catch (Exception e) {
            log.error("Failed to load rooms from {}", file.getAbsolutePath(), e);
        }
    }

    private synchronized void saveRooms() {
        File file = new File(ROOMS_FILE);
        if (file.getParentFile() != null && !file.getParentFile().exists()) {
            file.getParentFile().mkdirs();
        }
        List<StoredRoom> persisted = new ArrayList<>(rooms.values().stream().filter(room -> !room.system()).toList());
        try {
            objectMapper.writeValue(file, persisted);
        } catch (Exception e) {
            log.error("Failed to save rooms to {}", file.getAbsolutePath(), e);
        }
    }

    private record StoredRoom(String roomId, String name, String creatorPublicId, long createdAt, boolean system) {
    }
}
