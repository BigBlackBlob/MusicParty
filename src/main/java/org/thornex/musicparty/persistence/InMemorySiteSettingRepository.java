package org.thornex.musicparty.persistence;

import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.stereotype.Repository;

import java.util.Map;
import java.util.Optional;
import java.util.concurrent.ConcurrentHashMap;

@Repository
@ConditionalOnProperty(prefix = "app.music-api.database", name = "enabled", havingValue = "false")
public class InMemorySiteSettingRepository implements SiteSettingRepository {
    private final Map<String, String> settings = new ConcurrentHashMap<>();

    @Override
    public Optional<String> findValue(String key) {
        return Optional.ofNullable(settings.get(key));
    }

    @Override
    public void upsert(String key, String value, boolean secret, long updatedAt) {
        settings.put(key, value);
    }
}
