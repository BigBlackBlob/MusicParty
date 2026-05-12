const TIME_EXP = /\[(\d{2,}):(\d{2})(?:\.(\d{2,3}))?\]/g;

export const parseLyrics = (lrcString) => {
    if (!lrcString) return [];
    const lines = lrcString.split('\n');
    const result = [];

    for (let line of lines) {
        const matches = [...line.matchAll(TIME_EXP)];
        if (matches.length === 0) continue;

        const text = line.replace(TIME_EXP, '').trim();
        if (!text) continue;

        for (const match of matches) {
            const min = parseInt(match[1]);
            const sec = parseInt(match[2]);
            const ms = match[3] ? parseInt(match[3].padEnd(3, '0')) : 0;
            const time = min * 60 * 1000 + sec * 1000 + ms;
            result.push({ time, text });
        }
    }

    return result.sort((a, b) => a.time - b.time);
};

export const mergeTranslatedLyrics = (lyricString, translatedLyricString) => {
    const mainLines = parseLyrics(lyricString);
    const translatedLines = parseLyrics(translatedLyricString);

    if (!mainLines.length) return [];

    const translationByTime = new Map();
    for (const line of translatedLines) {
        if (!translationByTime.has(line.time)) {
            translationByTime.set(line.time, line.text);
        }
    }

    return mainLines.map((line) => ({
        ...line,
        translation: translationByTime.get(line.time) || ''
    }));
};
