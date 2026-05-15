package org.thornex.musicparty.persistence;

import lombok.RequiredArgsConstructor;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.core.RowMapper;
import org.springframework.stereotype.Repository;

import java.util.List;
import java.util.Optional;

@Repository
@RequiredArgsConstructor
public class JdbcUserProfileRepository implements UserProfileRepository {

    private static final RowMapper<PersistedUserProfile> USER_PROFILE_ROW_MAPPER = (rs, rowNum) -> new PersistedUserProfile(
            rs.getString("public_id"),
            rs.getString("display_name"),
            rs.getBoolean("is_guest"),
            rs.getLong("created_at"),
            rs.getLong("last_seen_at")
    );
    private static final RowMapper<PersistedSession> SESSION_ROW_MAPPER = (rs, rowNum) -> new PersistedSession(
            rs.getString("session_token_hash"),
            rs.getString("public_id"),
            rs.getLong("created_at"),
            rs.getLong("last_seen_at")
    );

    private final JdbcTemplate jdbcTemplate;

    @Override
    public void upsertProfile(PersistedUserProfile profile) {
        jdbcTemplate.update("""
                insert into user_profile(public_id, display_name, is_guest, created_at, last_seen_at)
                values (?, ?, ?, ?, ?)
                on conflict(public_id) do update set
                  display_name = excluded.display_name,
                  is_guest = excluded.is_guest,
                  created_at = excluded.created_at,
                  last_seen_at = excluded.last_seen_at
                """,
                profile.publicId(),
                profile.displayName(),
                profile.guest(),
                profile.createdAt(),
                profile.lastSeenAt());
    }

    @Override
    public Optional<PersistedUserProfile> findByPublicId(String publicId) {
        List<PersistedUserProfile> rows = jdbcTemplate.query("""
                select public_id, display_name, is_guest, created_at, last_seen_at
                from user_profile
                where public_id = ?
                """, USER_PROFILE_ROW_MAPPER, publicId);
        return rows.stream().findFirst();
    }

    @Override
    public void upsertSession(PersistedSession session) {
        jdbcTemplate.update("""
                insert into user_session(session_token_hash, public_id, created_at, last_seen_at)
                values (?, ?, ?, ?)
                on conflict(session_token_hash) do update set
                  public_id = excluded.public_id,
                  created_at = excluded.created_at,
                  last_seen_at = excluded.last_seen_at
                """,
                session.sessionTokenHash(),
                session.publicId(),
                session.createdAt(),
                session.lastSeenAt());
    }

    @Override
    public Optional<PersistedSession> findSessionByHash(String sessionTokenHash) {
        List<PersistedSession> rows = jdbcTemplate.query("""
                select session_token_hash, public_id, created_at, last_seen_at
                from user_session
                where session_token_hash = ?
                """, SESSION_ROW_MAPPER, sessionTokenHash);
        return rows.stream().findFirst();
    }
}
