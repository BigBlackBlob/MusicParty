package org.thornex.musicparty.service;

import org.springframework.http.HttpStatus;
import org.springframework.stereotype.Service;
import org.springframework.util.StringUtils;
import org.springframework.web.server.ResponseStatusException;
import org.thornex.musicparty.dto.Music;

import java.util.Arrays;
import java.util.List;
import java.util.stream.Collectors;

@Service
public class PlaylistExportService {
    public String format(List<Music> musics, String format) {
        String normalized = StringUtils.hasText(format) ? format.toLowerCase(java.util.Locale.ROOT) : "txt";
        return switch (normalized) {
            case "csv" -> toCsv(musics);
            case "json" -> toJson(musics);
            case "txt" -> toText(musics);
            default -> throw new ResponseStatusException(HttpStatus.BAD_REQUEST, "Unsupported export format");
        };
    }

    private String toText(List<Music> musics) {
        return musics.stream()
                .map(music -> "%s - %s [%s:%s]".formatted(
                        String.join(" / ", music.artists() == null ? List.of() : music.artists()),
                        music.name(),
                        music.platform(),
                        music.id()))
                .collect(Collectors.joining(System.lineSeparator()));
    }

    private String toCsv(List<Music> musics) {
        String header = "platform,id,name,artists,duration,coverUrl,externalUrl";
        String rows = musics.stream()
                .map(music -> Arrays.asList(
                        music.platform(),
                        music.id(),
                        music.name(),
                        String.join(" / ", music.artists() == null ? List.of() : music.artists()),
                        String.valueOf(music.duration()),
                        music.coverUrl(),
                        externalUrl(music)
                ).stream().map(this::csvCell).collect(Collectors.joining(",")))
                .collect(Collectors.joining(System.lineSeparator()));
        return rows.isEmpty() ? header : header + System.lineSeparator() + rows;
    }

    private String toJson(List<Music> musics) {
        return "[" + musics.stream()
                .map(music -> "{\"platform\":\"%s\",\"id\":\"%s\",\"name\":\"%s\",\"artists\":[%s],\"duration\":%d,\"coverUrl\":\"%s\",\"externalUrl\":\"%s\"}"
                        .formatted(
                                json(music.platform()),
                                json(music.id()),
                                json(music.name()),
                                (music.artists() == null ? List.<String>of() : music.artists()).stream()
                                        .map(artist -> "\"" + json(artist) + "\"")
                                        .collect(Collectors.joining(",")),
                                music.duration(),
                                json(music.coverUrl()),
                                json(externalUrl(music))))
                .collect(Collectors.joining(",")) + "]";
    }

    private String csvCell(String value) {
        String safe = value == null ? "" : value;
        return "\"" + safe.replace("\"", "\"\"") + "\"";
    }

    private String json(String value) {
        return (value == null ? "" : value)
                .replace("\\", "\\\\")
                .replace("\"", "\\\"")
                .replace("\n", "\\n")
                .replace("\r", "\\r");
    }

    private String externalUrl(Music music) {
        if ("netease".equals(music.platform())) return "https://music.163.com/#/song?id=" + music.id();
        if ("bilibili".equals(music.platform())) return "https://www.bilibili.com/video/" + music.id();
        return "";
    }
}
