export const MIN_PLATFORM_VOLUME_OFFSET_DB = -12
export const MAX_PLATFORM_VOLUME_OFFSET_DB = 0

export function normalizePlatformVolumeOffset(value: unknown): number {
  if (typeof value !== 'number' || !Number.isFinite(value)) return MAX_PLATFORM_VOLUME_OFFSET_DB
  return Math.max(MIN_PLATFORM_VOLUME_OFFSET_DB, Math.min(MAX_PLATFORM_VOLUME_OFFSET_DB, Math.round(value)))
}

export function parsePlatformVolumeOffsets(raw: string | null): Partial<Record<string, number>> {
  if (!raw) return {}
  try {
    const stored: unknown = JSON.parse(raw)
    if (!stored || typeof stored !== 'object' || Array.isArray(stored)) return {}
    return Object.fromEntries(Object.entries(stored)
      .map(([platform, value]) => [platform, normalizePlatformVolumeOffset(value)] as const)
      .filter(([, offset]) => offset !== MAX_PLATFORM_VOLUME_OFFSET_DB))
  } catch {
    return {}
  }
}

// A dB offset scales the amplitude before the native fade is applied.
export function getEffectiveVolume(volume: number, offsetDb = 0, fadeGain = 1): number {
  const userVolume = Number.isFinite(volume) ? volume : 0.5
  return Math.max(0, Math.min(1, userVolume * 10 ** (normalizePlatformVolumeOffset(offsetDb) / 20) * fadeGain))
}

export function getWidgetVolume(volume: number, offsetDb: number): number {
  return Math.round(getEffectiveVolume(volume, offsetDb) * 100)
}
