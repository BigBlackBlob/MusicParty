package org.thornex.musicparty.controller;

import lombok.RequiredArgsConstructor;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;
import org.thornex.musicparty.dto.RoomInfo;
import org.thornex.musicparty.dto.RoomVerifyRequest;
import org.thornex.musicparty.service.RoomAccessGrant;
import org.thornex.musicparty.service.RoomAccessService;
import org.thornex.musicparty.service.RoomService;
import org.thornex.musicparty.service.UserService;

import java.util.Map;
import java.util.List;

@RestController
@RequestMapping("/api/rooms")
@RequiredArgsConstructor
public class RoomController {
    private final RoomService roomService;
    private final RoomAccessService roomAccessService;
    private final UserService userService;

    @GetMapping
    public List<RoomInfo> listRooms() {
        return roomService.listRooms();
    }

    @PostMapping("/{roomId}/verify")
    public ResponseEntity<?> verifyRoomAccess(@PathVariable String roomId, @RequestBody RoomVerifyRequest request) {
        return userService.resolvePublicIdBySessionToken(request.sessionToken())
                .map(publicId -> toVerifyResponse(roomId, publicId, request.password()))
                .orElseGet(() -> ResponseEntity.status(HttpStatus.UNAUTHORIZED).body(Map.of(
                        "valid", false,
                        "message", "Unknown session token"
                )));
    }

    private ResponseEntity<Map<String, Object>> toVerifyResponse(String roomId, String publicId, String password) {
        RoomAccessGrant grant = roomAccessService.verifyAccess(roomId, publicId, password);
        if (!grant.allowed()) {
            return ResponseEntity.status(HttpStatus.UNAUTHORIZED).body(Map.of(
                    "valid", false,
                    "message", "Invalid room password"
            ));
        }

        return ResponseEntity.ok(Map.of(
                "valid", true,
                "roomAccessToken", grant.roomAccessToken(),
                "expiresAt", grant.expiresAt()
        ));
    }
}
