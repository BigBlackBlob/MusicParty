package org.thornex.musicparty;

import org.junit.jupiter.api.Test;
import org.springframework.boot.test.context.SpringBootTest;

@SpringBootTest(properties = {
		"app.music-api.admin-password=test-admin-password-12345",
		"app.music-api.database.enabled=false",
		"spring.autoconfigure.exclude=org.springframework.boot.autoconfigure.jdbc.DataSourceAutoConfiguration"
})
class MusicPartyApplicationTests {

	@Test
	void contextLoads() {
	}

}
