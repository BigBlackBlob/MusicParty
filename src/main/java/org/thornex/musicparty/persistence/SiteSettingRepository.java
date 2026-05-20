package org.thornex.musicparty.persistence;

import java.util.Optional;

public interface SiteSettingRepository {
    Optional<String> findValue(String key);
    void upsert(String key, String value, boolean secret, long updatedAt);
}
