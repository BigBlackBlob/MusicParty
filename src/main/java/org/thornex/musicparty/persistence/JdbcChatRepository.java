package org.thornex.musicparty.persistence;

import lombok.RequiredArgsConstructor;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.core.RowMapper;
import org.springframework.stereotype.Repository;
import org.thornex.musicparty.dto.ChatMessage;
import org.thornex.musicparty.enums.MessageType;

import java.util.List;

@Repository
@RequiredArgsConstructor
public class JdbcChatRepository implements ChatRepository {

    private static final RowMapper<ChatMessage> CHAT_MESSAGE_ROW_MAPPER = (rs, rowNum) -> new ChatMessage(
            rs.getString("id"),
            rs.getString("user_id"),
            rs.getString("user_name"),
            rs.getString("content"),
            rs.getLong("created_at"),
            MessageType.valueOf(rs.getString("type"))
    );

    private final JdbcTemplate jdbcTemplate;

    @Override
    public void appendMessage(String roomId, ChatMessage message) {
        jdbcTemplate.update("""
                insert into chat_message(id, room_id, user_id, user_name, content, type, created_at)
                values (?, ?, ?, ?, ?, ?, ?)
                """,
                message.id(),
                roomId,
                message.userId(),
                message.userName(),
                message.content(),
                message.type().name(),
                message.timestamp());
    }

    @Override
    public List<ChatMessage> fetchMessages(String roomId, int offset, int limit) {
        return jdbcTemplate.query("""
                select id, user_id, user_name, content, type, created_at
                from chat_message
                where ((? is null and room_id is null) or room_id = ?)
                order by created_at desc
                limit ? offset ?
                """, CHAT_MESSAGE_ROW_MAPPER, roomId, roomId, limit, offset);
    }
}
