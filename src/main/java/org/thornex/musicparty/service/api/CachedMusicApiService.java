package org.thornex.musicparty.service.api;

public interface CachedMusicApiService extends IMusicApiService {
    default String cacheKey(String musicId) {
        return musicId;
    }
}
