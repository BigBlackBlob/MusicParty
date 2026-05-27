import { describe, expect, it } from 'vitest';
import { resolveVendorChunk } from '../config/vendorChunks.js';

describe('frontend payload chunking', () => {
  it('keeps Vue ecosystem dependencies in the dedicated vendor-vue chunk', () => {
    expect(resolveVendorChunk('/repo/node_modules/vue/dist/vue.runtime.esm-bundler.js')).toBe('vendor-vue');
    expect(resolveVendorChunk('/repo/node_modules/pinia/dist/pinia.mjs')).toBe('vendor-vue');
    expect(resolveVendorChunk('/repo/node_modules/vue-i18n/dist/vue-i18n.mjs')).toBe('vendor-vue');
    expect(resolveVendorChunk('/repo/node_modules/@vueuse/core/index.mjs')).toBe('vendor-vue');
  });

  it('keeps other large third-party dependencies out of the entry chunk', () => {
    expect(resolveVendorChunk('/repo/node_modules/reka-ui/dist/index.mjs')).toBe('vendor-ui');
    expect(resolveVendorChunk('/repo/node_modules/axios/index.js')).toBe('vendor-network');
    expect(resolveVendorChunk('/repo/node_modules/sortablejs/modular/sortable.esm.js')).toBe('vendor-dnd');
    expect(resolveVendorChunk('/repo/src/main.js')).toBeUndefined();
  });
});
