package org.thornex.musicparty.persistence;

import lombok.RequiredArgsConstructor;
import org.springframework.boot.autoconfigure.condition.ConditionalOnBean;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.core.RowMapper;
import org.springframework.stereotype.Repository;
import org.springframework.transaction.annotation.Transactional;
import org.thornex.musicparty.dto.ChatMessage;
import org.thornex.musicparty.enums.MessageType;

import java.util.List;

@Repository
@RequiredArgsConstructor
@ConditionalOnBean(JdbcTemplate.class)
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

    @Override
    @Transactional
    public void replaceMessages(String roomId, List<ChatMessage> messages) {
        if (roomId == null) {
            jdbcTemplate.update("delete from chat_message where room_id is null");
        } else {
            jdbcTemplate.update("delete from chat_message where room_id = ?", roomId);
        }
        for (ChatMessage message : messages) {
            appendMessage(roomId, message);
        }
    }

    @Override
    public void deleteRoomHistory(String roomId) {
        jdbcTemplate.update("delete from chat_message where room_id = ?", roomId);
    }
}
