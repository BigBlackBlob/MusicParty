package org.thornex.musicparty.service;

import org.springframework.stereotype.Service;
import org.springframework.util.StringUtils;
import org.thornex.musicparty.persistence.SiteSettingRepository;

import java.util.Optional;

@Service
public class SiteSettingService {
    public static final String NETEASE_COOKIE = "netease.cookie";
    public static final String BILIBILI_SESSDATA = "bilibili.sessdata";

    private final SiteSettingRepository repository;
    private final SubsonicCredentialCipher cipher;

    public SiteSettingService(SiteSettingRepository repository, SubsonicCredentialCipher cipher) {
        this.repository = repository;
        this.cipher = cipher;
    }

    public Optional<String> findSecret(String key) {
        return repository.findValue(key).map(cipher::decrypt).filter(StringUtils::hasText);
    }

    public String secretOrDefault(String key, String fallback) {
        return findSecret(key).orElse(fallback);
    }

    public void putSecret(String key, String value) {
        repository.upsert(key, cipher.encrypt(value), true, System.currentTimeMillis());
    }

    public void importSecretIfMissing(String key, String value) {
        if (!StringUtils.hasText(value) || repository.findValue(key).isPresent()) {
            return;
        }
        putSecret(key, value);
    }
}
