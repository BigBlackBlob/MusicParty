import com.fasterxml.jackson.databind.ObjectMapper;
import org.sqlite.SQLiteDataSource;
import org.springframework.jdbc.core.JdbcTemplate;
import org.thornex.musicparty.dto.*;
import org.thornex.musicparty.enums.MessageType;
import org.thornex.musicparty.persistence.*;

var fixture = System.getenv("MUSICPARTY_JAVA_SQLITE_FIXTURE");
if (fixture == null || fixture.isBlank()) throw new IllegalStateException("MUSICPARTY_JAVA_SQLITE_FIXTURE is required");
var dataSource = new SQLiteDataSource();
dataSource.setUrl("jdbc:sqlite:" + fixture);
var jdbc = new JdbcTemplate(dataSource);
var json = new ObjectMapper();

var rooms = new JdbcRoomRepository(jdbc);
var room = rooms.findById("go-roundtrip-room").orElseThrow();
if (!room.name().equals("Go Roundtrip Room")) throw new IllegalStateException("Java could not read Go room");
if (new JdbcUserAccountRepository(jdbc).findByUsername("go-roundtrip").isEmpty()) throw new IllegalStateException("Java could not read Go account");
if (new JdbcQueueRepository(jdbc, json).loadQueue("go-roundtrip-room").size() != 1) throw new IllegalStateException("Java could not read Go queue JSON");
if (new JdbcPlaybackStateRepository(jdbc, json).findByRoomId("go-roundtrip-room").orElseThrow().currentMusic() == null) throw new IllegalStateException("Java could not read Go playback JSON");
if (new JdbcChatRepository(jdbc).fetchMessages("go-roundtrip-room", 0, 20).stream().noneMatch(message -> message.id().equals("go-roundtrip-chat"))) throw new IllegalStateException("Java could not read Go chat");
if (new JdbcRoomPlaylistRepository(jdbc, json).listTracks("go-roundtrip-room", "go-roundtrip-room-playlist", 0, 20).size() != 1) throw new IllegalStateException("Java could not read Go room playlist JSON");
if (new JdbcUserPlaylistRepository(jdbc, json).listTracks("go-roundtrip-user", "go-roundtrip-user-playlist", 0, 20).size() != 1) throw new IllegalStateException("Java could not read Go user playlist JSON");
if (!new JdbcSiteSettingRepository(jdbc).findValue("go.roundtrip").orElseThrow().equals("go-value")) throw new IllegalStateException("Java could not read Go setting");
if (new JdbcSubsonicSourceRepository(jdbc).findById("go-roundtrip-subsonic").isEmpty()) throw new IllegalStateException("Java could not read Go Subsonic source");
if (new JdbcLocalTrackRepository(jdbc).findById("go-roundtrip-music").isEmpty()) throw new IllegalStateException("Java could not read Go local track");

var profiles = new JdbcUserProfileRepository(jdbc);
profiles.upsertProfile(new PersistedUserProfile("java-roundtrip-user", "Java Roundtrip User", false, "go-roundtrip-room", 2000L, 2000L));
new JdbcRoomAccessRepository(jdbc).upsertMembership(new PersistedRoomMembership("go-roundtrip-room", "java-roundtrip-user", "MEMBER", 2000L, 2000L));
rooms.touch("go-roundtrip-room", 4242L);
new JdbcChatRepository(jdbc).appendMessage("go-roundtrip-room", new ChatMessage("java-roundtrip-chat", "java-roundtrip-user", "Java", "Java chat", 2100L, MessageType.SYSTEM));
new JdbcSiteSettingRepository(jdbc).upsert("java.roundtrip", "ok", false, 2200L);
new JdbcLocalTrackRepository(jdbc).grantUploadUser("java-roundtrip-uploader", 2300L);
System.out.println("JAVA_REPOSITORY_ROUNDTRIP_OK");
/exit
