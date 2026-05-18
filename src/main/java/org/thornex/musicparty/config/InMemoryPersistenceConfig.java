package org.thornex.musicparty.config;

import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.thornex.musicparty.persistence.ChatRepository;
import org.thornex.musicparty.persistence.InMemoryChatRepository;
import org.thornex.musicparty.persistence.InMemoryPlaybackStateRepository;
import org.thornex.musicparty.persistence.InMemoryQueueRepository;
import org.thornex.musicparty.persistence.InMemoryRoomRepository;
import org.thornex.musicparty.persistence.InMemorySubsonicSourceRepository;
import org.thornex.musicparty.persistence.InMemoryUserProfileRepository;
import org.thornex.musicparty.persistence.InMemoryMigrationStateRepository;
import org.thornex.musicparty.persistence.MigrationStateRepository;
import org.thornex.musicparty.persistence.PlaybackStateRepository;
import org.thornex.musicparty.persistence.QueueRepository;
import org.thornex.musicparty.persistence.RoomRepository;
import org.thornex.musicparty.persistence.RoomPlaylistRepository;
import org.thornex.musicparty.persistence.InMemoryRoomPlaylistRepository;
import org.thornex.musicparty.persistence.SubsonicSourceRepository;
import org.thornex.musicparty.persistence.UserProfileRepository;

@Configuration
@ConditionalOnProperty(prefix = "app.music-api.database", name = "enabled", havingValue = "false")
public class InMemoryPersistenceConfig {

    @Bean
    public RoomRepository roomRepository() {
        return new InMemoryRoomRepository();
    }

    @Bean
    public UserProfileRepository userProfileRepository() {
        return new InMemoryUserProfileRepository();
    }

    @Bean
    public QueueRepository queueRepository() {
        return new InMemoryQueueRepository();
    }

    @Bean
    public ChatRepository chatRepository() {
        return new InMemoryChatRepository();
    }

    @Bean
    public PlaybackStateRepository playbackStateRepository() {
        return new InMemoryPlaybackStateRepository();
    }

    @Bean
    public MigrationStateRepository migrationStateRepository() {
        return new InMemoryMigrationStateRepository();
    }

    @Bean
    public SubsonicSourceRepository subsonicSourceRepository() {
        return new InMemorySubsonicSourceRepository();
    }

    @Bean
    public RoomPlaylistRepository roomPlaylistRepository() {
        return new InMemoryRoomPlaylistRepository();
    }
}
