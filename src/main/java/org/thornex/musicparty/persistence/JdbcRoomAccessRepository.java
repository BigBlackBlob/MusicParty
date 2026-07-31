package org.thornex.musicparty.persistence;

import lombok.RequiredArgsConstructor;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.stereotype.Repository;
import java.util.List;
import java.util.Optional;

@Repository
@RequiredArgsConstructor
@ConditionalOnProperty(prefix = "app.music-api.database", name = "enabled", havingValue = "true", matchIfMissing = true)
public class JdbcRoomAccessRepository implements RoomAccessRepository {
    private final JdbcTemplate jdbc;
    @Override public void upsertMembership(PersistedRoomMembership m) {
        jdbc.update("insert into room_membership(room_id, public_id, role, created_at, updated_at) values (?, ?, ?, ?, ?) on conflict(room_id, public_id) do update set role=excluded.role, updated_at=excluded.updated_at",
                m.roomId(), m.publicId(), m.role(), m.createdAt(), m.updatedAt());
    }
    @Override public Optional<PersistedRoomMembership> findMembership(String roomId, String publicId) {
        return jdbc.query("select room_id, public_id, role, created_at, updated_at from room_membership where room_id=? and public_id=?",
                (rs, n) -> new PersistedRoomMembership(rs.getString(1), rs.getString(2), rs.getString(3), rs.getLong(4), rs.getLong(5)), roomId, publicId).stream().findFirst();
    }
    @Override public List<PersistedRoomMembership> listMemberships(String roomId) {
        return jdbc.query("select room_id, public_id, role, created_at, updated_at from room_membership where room_id=? order by created_at", (rs,n) ->
                new PersistedRoomMembership(rs.getString(1),rs.getString(2),rs.getString(3),rs.getLong(4),rs.getLong(5)), roomId);
    }
    @Override public int countOwners(String roomId) { Integer count=jdbc.queryForObject("select count(*) from room_membership where room_id=? and role='OWNER'", Integer.class, roomId); return count == null ? 0 : count; }
    @Override public void deleteMembership(String roomId, String publicId) { jdbc.update("delete from room_membership where room_id=? and public_id=?", roomId, publicId); }
    @Override public void createInvite(PersistedRoomInvite i) { jdbc.update("insert into room_invite(id, room_id, created_by_public_id, secret_hash, label, expires_at, max_uses, used_at, used_by_public_id, revoked_at, created_at) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", i.id(),i.roomId(),i.createdByPublicId(),i.secretHash(),i.label(),i.expiresAt(),i.maxUses(),i.usedAt(),i.usedByPublicId(),i.revokedAt(),i.createdAt()); }
    @Override public Optional<PersistedRoomInvite> findInviteById(String id) { return jdbc.query("select id, room_id, created_by_public_id, secret_hash, label, expires_at, max_uses, used_at, used_by_public_id, revoked_at, created_at from room_invite where id=?", (rs,n) -> invite(rs),id).stream().findFirst(); }
    @Override public Optional<PersistedRoomInvite> findInviteBySecretHash(String hash) { return jdbc.query("select id, room_id, created_by_public_id, secret_hash, label, expires_at, max_uses, used_at, used_by_public_id, revoked_at, created_at from room_invite where secret_hash=?", (rs,n) -> invite(rs),hash).stream().findFirst(); }
    @Override public List<PersistedRoomInvite> listInvites(String roomId) { return jdbc.query("select id, room_id, created_by_public_id, secret_hash, label, expires_at, max_uses, used_at, used_by_public_id, revoked_at, created_at from room_invite where room_id=? order by created_at desc", (rs,n)->invite(rs),roomId); }
    @Override public boolean consumeInvite(String id,String publicId,long usedAt) { return jdbc.update("update room_invite set used_at=?, used_by_public_id=? where id=? and used_at is null and revoked_at is null and expires_at>=?", usedAt,publicId,id,usedAt)==1; }
    @Override public boolean revokeInvite(String id,long revokedAt) { return jdbc.update("update room_invite set revoked_at=? where id=? and revoked_at is null",revokedAt,id)==1; }
    @Override public boolean revokeInvite(String roomId,String id,long revokedAt) { return jdbc.update("update room_invite set revoked_at=? where room_id=? and id=? and revoked_at is null",revokedAt,roomId,id)==1; }
    private PersistedRoomInvite invite(java.sql.ResultSet rs) throws java.sql.SQLException { long used=rs.getLong(8); Long usedAt=rs.wasNull()?null:used; long revoked=rs.getLong(10); Long revokedAt=rs.wasNull()?null:revoked; return new PersistedRoomInvite(rs.getString(1),rs.getString(2),rs.getString(3),rs.getString(4),rs.getString(5),rs.getLong(6),rs.getInt(7),usedAt,rs.getString(9),revokedAt,rs.getLong(11)); }
}
