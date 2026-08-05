export interface VendorChunk {
  name: string
  packages: readonly string[]
}

export const vendorChunks: readonly VendorChunk[]
export function normalizeModuleId(id: string): string
export function resolveVendorChunk(id: string): string | undefined
