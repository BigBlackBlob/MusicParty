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

function findOwnerIndex(lines: YrcLine[], start: number, end: number): number {
  const mid = (start + end) / 2
  let fallback = -1
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i] as YrcLine
    const ownerEnd = Math.max(line.end, line.start + 1)
    if (start >= line.start && start < ownerEnd) return i
    if (line.start <= mid) fallback = i
  }
  if (fallback >= 0) return fallback
  return lines.length ? 0 : -1
}

// Measured live 2026-09-22: ytlrc/yromalrc line counts do not align with yrc
// (e.g. Cruel Summer 65 vs 60), so extra tracks are paired by time window,
// never by line number.
export function pairByTime(main: YrcLine[], extra: YrcLine[]): string[] {
  const buckets: string[][] = main.map(() => [])
  for (const line of extra) {
    const index = findOwnerIndex(main, line.start, line.end)
    const bucket = index >= 0 ? buckets[index] : undefined
    if (bucket) bucket.push(line.text)
  }
  return buckets.map((bucket) => bucket.join(' '))
}
