package org.thornex.musicparty.dto;

public record LocalUploadResult(
        LocalTrack track,
        boolean duplicate,
        String duplicateOf
) {
    public static LocalUploadResult created(LocalTrack track) {
        return new LocalUploadResult(track, false, null);
    }

    public static LocalUploadResult duplicate(LocalTrack track) {
        return new LocalUploadResult(track, true, track.id());
    }
}
