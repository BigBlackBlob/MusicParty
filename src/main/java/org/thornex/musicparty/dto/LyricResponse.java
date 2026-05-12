package org.thornex.musicparty.dto;

public record LyricResponse(
        String lyric,
        String translatedLyric,
        String romanizedLyric
) {
    public static LyricResponse empty() {
        return new LyricResponse("", "", "");
    }
}
