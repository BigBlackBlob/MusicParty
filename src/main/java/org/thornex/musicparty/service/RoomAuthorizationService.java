package org.thornex.musicparty.service;

import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;
import org.springframework.util.StringUtils;
import org.thornex.musicparty.persistence.PersistedRoomMembership;
import org.thornex.musicparty.persistence.RoomAccessRepository;

@Service
@RequiredArgsConstructor
public class RoomAuthorizationService {
    private final RoomAccessRepository accessRepository;
    private final AccountService accountService;
    private final RoomService roomService;

    public boolean isPlatformAdmin(String sessionToken) {
        return accountService.resolveSession(sessionToken).map(s -> "PLATFORM_ADMIN".equals(s.role()) || "ADMIN".equals(s.role())).orElse(false);
    }
    public boolean isMember(String roomId, String publicId) {
        return StringUtils.hasText(publicId) && accessRepository.findMembership(roomId, publicId).isPresent();
    }

    public boolean canAccessRoom(String roomId, String publicId, String sessionToken) {
        if (isPlatformAdmin(sessionToken)) {
            return true;
        }
        return roomService.getRoomAccessMetadata(roomId)
                .map(room -> !room.privateRoom() || isMember(room.roomId(), publicId))
                .orElse(false);
    }
    public boolean isOwner(String roomId, String publicId) {
        return accessRepository.findMembership(roomId, publicId).map(PersistedRoomMembership::owner).orElse(false);
    }
    public boolean canManageRoom(String roomId, String publicId, String sessionToken) {
        return isPlatformAdmin(sessionToken) || isOwner(roomId, publicId);
    }
}
