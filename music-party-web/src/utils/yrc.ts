export interface YrcSegment {
  start: number
  dur: number
  end: number
  text: string
}

export interface YrcLine {
  start: number
  end: number
  text: string
  segments: YrcSegment[]
}

const LINE_HEADER = /^\[(\d+),(\d+)\]/
// Netease frequently serves ytlrc/yromalrc as plain colon-timestamp LRC rows
// (e.g. "[00:03.580]译文") instead of segment YRC; accept them as whole-line
// entries so time-window pairing still works.
const LRC_HEADER = /^\[(\d{1,2}):(\d{2})(?:[.:](\d{1,3}))?\]/
const SEGMENT_MARKER_SOURCE = '\\((\\d+),(\\d+),(\\d+)\\)'

interface MarkerHit {
  index: number
  length: number
  segStart: number
  segDur: number
}

export function parseYrc(raw?: string | null): YrcLine[] {
  if (!raw) return []
  const markerRe = new RegExp(SEGMENT_MARKER_SOURCE, 'g')
  const lines: YrcLine[] = []
  for (const row of raw.split(/\r?\n/)) {
    const trimmed = row.trim()
    // /lyric/new rewrites credit metadata as JSON lines; they carry no timing.
    if (!trimmed || trimmed.startsWith('{')) continue
    const lineHeader = LINE_HEADER.exec(trimmed)
    let start = Number.NaN
    let end = Number.NaN
    let bodyStart = 0
    if (lineHeader) {
      start = Number(lineHeader[1])
      end = start + Number(lineHeader[2])
      bodyStart = lineHeader[0].length
    } else {
      const lrcHeader = LRC_HEADER.exec(trimmed)
      if (!lrcHeader) continue
      const msPart = lrcHeader[3]
      start = Number(lrcHeader[1]) * 60000 + Number(lrcHeader[2]) * 1000 + (msPart ? Number(msPart.padEnd(3, '0')) : 0)
      end = start
      bodyStart = lrcHeader[0].length
    }
    const rest = trimmed.slice(bodyStart)
    const markers: MarkerHit[] = []
    let hit: RegExpExecArray | null
    while ((hit = markerRe.exec(rest)) !== null) {
      markers.push({ index: hit.index, length: hit[0].length, segStart: Number(hit[1]), segDur: Number(hit[2]) })
    }
    const segments: YrcSegment[] = []
    if (markers.length) {
      for (let i = 0; i < markers.length; i++) {
        const marker = markers[i] as MarkerHit
        const next = markers[i + 1]
        const text = rest.slice(marker.index + marker.length, next ? next.index : rest.length)
        if (!text) continue
        segments.push({ start: marker.segStart, dur: marker.segDur, end: marker.segStart + marker.segDur, text })
      }
    } else if (rest) {
      segments.push({ start, dur: Math.max(0, end - start), end, text: rest })
    }
    const text = segments.map((segment) => segment.text).join('').trim()
    if (!text || !Number.isFinite(start)) continue
    lines.push({ start, end, text, segments })
  }
  return lines.sort((a, b) => a.start - b.start)
}

function findOwnerIndex(lines: YrcLine[], start: number): number {
  let nearest = -1
  let distance = Infinity
  for (let i = 0; i < lines.length; i++) {
    const delta = Math.abs((lines[i] as YrcLine).start - start)
    if (delta <= 500 && delta < distance) { nearest = i; distance = delta }
  }
  if (nearest >= 0) return nearest
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i] as YrcLine
    const ownerEnd = Math.min(Math.max(line.end, line.start + 1), lines[i + 1]?.start ?? Infinity)
    if (start >= line.start && start < ownerEnd) return i
  }
  return -1
}

// Measured live 2026-09-22: ytlrc/yromalrc line counts do not align with yrc
// (e.g. Cruel Summer 65 vs 60), so extra tracks are paired by onset (within
// 500 ms) or the active line's window, never by line number. Unrelated
// timestamps outside these bounds stay unpaired.
export function pairByTime(main: YrcLine[], extra: YrcLine[]): string[] {
  const buckets: string[][] = main.map(() => [])
  for (const line of extra) {
    const index = findOwnerIndex(main, line.start)
    const bucket = index >= 0 ? buckets[index] : undefined
    if (bucket) bucket.push(line.text)
  }
  return buckets.map((bucket) => bucket.join(' '))
}

/** Legacy timestamps belong to legacy originals; bridge to YRC by original text. */
export function pairLyricExtras(main: YrcLine[], wordExtra: string | null | undefined, legacyMain: string | null | undefined, legacyExtra: string | null | undefined): string[] {
  const primary = pairByTime(main, parseYrc(wordExtra))
  const legacy = parseYrc(legacyMain)
  const fallback = pairByTime(legacy.length ? legacy : main, parseYrc(legacyExtra))
  if (!legacy.length) return primary.map((text, index) => text || fallback[index] || '')
  const key = (text: string) => text.normalize('NFKC').toLowerCase().replace(/[\p{P}\p{Z}\p{S}\s]/gu, '')
  const legacyKeys = legacy.map(line => key(line.text))
  let cursor = 0
  return main.map((line, index) => {
    const textKey = key(line.text)
    let match = -1
    let distance = Infinity
    for (let i = cursor; textKey && i < legacy.length; i++) {
      const delta = Math.abs((legacy[i] as YrcLine).start - line.start)
      if (legacyKeys[i] === textKey && delta < distance) { match = i; distance = delta }
    }
    // Matching in order keeps repeated chorus lines from sharing a legacy row.
    if (match >= 0) cursor = match + 1
    return primary[index] || (match >= 0 ? fallback[match] : '') || ''
  })
}
