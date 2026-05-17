package org.thornex.musicparty.persistence;

import lombok.RequiredArgsConstructor;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.core.RowMapper;
import org.springframework.stereotype.Repository;

import java.util.List;
import java.util.Optional;

@Repository
@RequiredArgsConstructor
@ConditionalOnProperty(prefix = "app.music-api.database", name = "enabled", havingValue = "true", matchIfMissing = true)
public class JdbcSubsonicSourceRepository implements SubsonicSourceRepository {

    private static final RowMapper<PersistedSubsonicSource> ROW_MAPPER = (rs, rowNum) -> new PersistedSubsonicSource(
            rs.getString("id"),
            rs.getString("owner_room_id"),
            rs.getString("label"),
            rs.getString("base_url"),
            rs.getString("username"),
            rs.getString("password"),
            rs.getString("client"),
            rs.getString("api_version"),
            rs.getString("allowed_users"),
            rs.getBoolean("enabled"),
            rs.getBoolean("system"),
            rs.getLong("created_at"),
            rs.getLong("updated_at")
    );
    private static final RowMapper<PersistedRoomSubsonicSource> BINDING_ROW_MAPPER = (rs, rowNum) -> new PersistedRoomSubsonicSource(
            rs.getString("room_id"),
            rs.getString("source_id"),
            rs.getBoolean("enabled"),
            rs.getString("display_label"),
            rs.getString("allowed_users"),
            rs.getInt("sort_order"),
            rs.getLong("created_at"),
            rs.getLong("updated_at")
    );

    private final JdbcTemplate jdbcTemplate;

    @Override
    public List<PersistedSubsonicSource> findAll() {
        return jdbcTemplate.query("""
                select id, owner_room_id, label, base_url, username, password, client, api_version, allowed_users,
                       enabled, system, created_at, updated_at
                from subsonic_source
                order by system desc, label asc
                """, ROW_MAPPER);
    }

    @Override
    public Optional<PersistedSubsonicSource> findById(String id) {
        return jdbcTemplate.query("""
                select id, owner_room_id, label, base_url, username, password, client, api_version, allowed_users,
                       enabled, system, created_at, updated_at
                from subsonic_source
                where id = ?
                """, ROW_MAPPER, id).stream().findFirst();
    }

    @Override
    public void upsert(PersistedSubsonicSource source) {
        jdbcTemplate.update("""
                insert into subsonic_source(id, label, base_url, username, password, client, api_version,
                                            allowed_users, enabled, system, owner_room_id, created_at, updated_at)
                values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                on conflict(id) do update set
                  label = excluded.label,
                  base_url = excluded.base_url,
                  username = excluded.username,
                  password = excluded.password,
                  client = excluded.client,
                  api_version = excluded.api_version,
                  allowed_users = excluded.allowed_users,
                  enabled = excluded.enabled,
                  system = excluded.system,
                  owner_room_id = excluded.owner_room_id,
                  updated_at = excluded.updated_at
                """,
                source.id(),
                source.label(),
                source.baseUrl(),
                source.username(),
                source.password(),
                source.client(),
                source.apiVersion(),
                source.allowedUsers(),
                source.enabled(),
                source.system(),
                source.ownerRoomId(),
                source.createdAt(),
                source.updatedAt()
        );
    }

    @Override
    public void delete(String id) {
        jdbcTemplate.update("delete from subsonic_source where id = ? and system = 0", id);
    }

    @Override
    public List<PersistedRoomSubsonicSource> findRoomBindings(String roomId) {
        return jdbcTemplate.query("""
                select room_id, source_id, enabled, display_label, allowed_users, sort_order, created_at, updated_at
                from room_subsonic_source
                where room_id = ?
                order by sort_order asc, source_id asc
                """, BINDING_ROW_MAPPER, roomId);
    }

    @Override
    public List<PersistedRoomSubsonicSource> findAllRoomBindings() {
        return jdbcTemplate.query("""
                select room_id, source_id, enabled, display_label, allowed_users, sort_order, created_at, updated_at
                from room_subsonic_source
                order by room_id asc, sort_order asc, source_id asc
                """, BINDING_ROW_MAPPER);
    }

    @Override
    public Optional<PersistedRoomSubsonicSource> findRoomBinding(String roomId, String sourceId) {
        return jdbcTemplate.query("""
                select room_id, source_id, enabled, display_label, allowed_users, sort_order, created_at, updated_at
                from room_subsonic_source
                where room_id = ? and source_id = ?
                """, BINDING_ROW_MAPPER, roomId, sourceId).stream().findFirst();
    }

    @Override
    public void upsertRoomBinding(PersistedRoomSubsonicSource binding) {
        jdbcTemplate.update("""
                insert into room_subsonic_source(room_id, source_id, enabled, display_label, allowed_users, sort_order, created_at, updated_at)
                values (?, ?, ?, ?, ?, ?, ?, ?)
                on conflict(room_id, source_id) do update set
                  enabled = excluded.enabled,
                  display_label = excluded.display_label,
                  allowed_users = excluded.allowed_users,
                  sort_order = excluded.sort_order,
                  updated_at = excluded.updated_at
                """,
                binding.roomId(),
                binding.sourceId(),
                binding.enabled(),
                binding.displayLabel(),
                binding.allowedUsers(),
                binding.sortOrder(),
                binding.createdAt(),
                binding.updatedAt()
        );
    }

    @Override
    public void deleteRoomBinding(String roomId, String sourceId) {
        jdbcTemplate.update("delete from room_subsonic_source where room_id = ? and source_id = ?", roomId, sourceId);
    }
}
