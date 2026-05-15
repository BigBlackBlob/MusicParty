package org.thornex.musicparty.persistence;

import lombok.RequiredArgsConstructor;
import org.springframework.boot.autoconfigure.condition.ConditionalOnBean;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.core.RowMapper;
import org.springframework.stereotype.Repository;

import java.sql.ResultSet;
import java.sql.SQLException;
import java.util.List;
import java.util.Optional;

@Repository
@RequiredArgsConstructor
@ConditionalOnBean(JdbcTemplate.class)
public class JdbcRoomRepository implements RoomRepository {

    private static final RowMapper<PersistedRoom> ROOM_ROW_MAPPER = new PersistedRoomRowMapper();

    private final JdbcTemplate jdbcTemplate;

    @Override
    public List<PersistedRoom> findAllActive() {
        return jdbcTemplate.query("""
                select id, name, owner_public_id, visibility, password_hash, password_version, system, created_at, last_active_at, deleted_at
                from room
                where deleted_at is null
                order by system desc, last_active_at desc, created_at asc
                """, ROOM_ROW_MAPPER);
    }

    @Override
    public Optional<PersistedRoom> findById(String roomId) {
        List<PersistedRoom> rows = jdbcTemplate.query("""
                select id, name, owner_public_id, visibility, password_hash, password_version, system, created_at, last_active_at, deleted_at
                from room
                where id = ? and deleted_at is null
                """, ROOM_ROW_MAPPER, roomId);
        return rows.stream().findFirst();
    }

    @Override
    public void upsert(PersistedRoom room) {
        jdbcTemplate.update("""
                insert into room(id, name, owner_public_id, visibility, password_hash, password_version, system, created_at, last_active_at, deleted_at)
                values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                on conflict(id) do update set
                  name = excluded.name,
                  owner_public_id = excluded.owner_public_id,
                  visibility = excluded.visibility,
                  password_hash = excluded.password_hash,
                  password_version = excluded.password_version,
                  system = excluded.system,
                  created_at = excluded.created_at,
                  last_active_at = excluded.last_active_at,
                  deleted_at = excluded.deleted_at
                """,
                room.id(),
                room.name(),
                room.ownerPublicId(),
                room.visibility(),
                room.passwordHash(),
                room.passwordVersion(),
                room.system(),
                room.createdAt(),
                room.lastActiveAt(),
                room.deletedAt());
    }

    @Override
    public void touch(String roomId, long lastActiveAt) {
        jdbcTemplate.update("update room set last_active_at = ? where id = ?", lastActiveAt, roomId);
    }

    @Override
    public void softDelete(String roomId, long deletedAt) {
        jdbcTemplate.update("update room set deleted_at = ? where id = ?", deletedAt, roomId);
    }

    private static final class PersistedRoomRowMapper implements RowMapper<PersistedRoom> {
        @Override
        public PersistedRoom mapRow(ResultSet rs, int rowNum) throws SQLException {
            long deletedAtValue = rs.getLong("deleted_at");
            Long deletedAt = rs.wasNull() ? null : deletedAtValue;
            return new PersistedRoom(
                    rs.getString("id"),
                    rs.getString("name"),
                    rs.getString("owner_public_id"),
                    rs.getString("visibility"),
                    rs.getString("password_hash"),
                    rs.getInt("password_version"),
                    rs.getBoolean("system"),
                    rs.getLong("created_at"),
                    rs.getLong("last_active_at"),
                    deletedAt
            );
        }
    }
}
