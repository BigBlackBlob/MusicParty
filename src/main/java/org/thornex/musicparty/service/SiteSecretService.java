package org.thornex.musicparty.service;

import org.springframework.stereotype.Service;
import org.thornex.musicparty.persistence.SiteSettingRepository;

import java.security.SecureRandom;
import java.util.Base64;

@Service
public class SiteSecretService {
    private static final String SITE_SECRET_KEY = "security.site-secret.v1";

    private final SiteSettingRepository repository;
    private final SecureRandom secureRandom = new SecureRandom();

    public SiteSecretService(SiteSettingRepository repository) {
        this.repository = repository;
    }

    public synchronized String getOrCreateSiteSecret() {
        return repository.findValue(SITE_SECRET_KEY)
                .filter(value -> !value.isBlank())
                .orElseGet(() -> {
                    byte[] secret = new byte[32];
                    secureRandom.nextBytes(secret);
                    String encoded = Base64.getEncoder().encodeToString(secret);
                    repository.upsert(SITE_SECRET_KEY, encoded, true, System.currentTimeMillis());
                    return encoded;
                });
    }
}
