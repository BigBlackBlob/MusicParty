import { beforeEach, describe, expect, it } from 'vitest'
import { obsoleteStorageKeys, removeObsoleteStorage } from './legacyStorage'

describe('legacy storage migration', () => {
  beforeEach(() => localStorage.clear())

  it('removes Java-era credentials and transient caches while preserving preferences', () => {
    for (const key of obsoleteStorageKeys) localStorage.setItem(key, 'legacy')
    localStorage.setItem('mp_volume', '0.4')

    removeObsoleteStorage()

    for (const key of obsoleteStorageKeys) expect(localStorage.getItem(key)).toBeNull()
    expect(localStorage.getItem('mp_volume')).toBe('0.4')
  })
})
