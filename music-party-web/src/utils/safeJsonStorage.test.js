import { describe, expect, it } from 'vitest';
import { isPlainObject, safeJsonStorage } from './safeJsonStorage.js';

describe('safeJsonStorage', () => {
  it('returns fallback and removes corrupt JSON', () => {
    localStorage.setItem('broken', '{bad json');

    expect(safeJsonStorage.read('broken', [], { validate: Array.isArray })).toEqual([]);
    expect(localStorage.getItem('broken')).toBeNull();
  });

  it('returns fallback and removes values with the wrong shape', () => {
    localStorage.setItem('bindings', '[]');

    expect(safeJsonStorage.read('bindings', {}, { validate: isPlainObject })).toEqual({});
    expect(localStorage.getItem('bindings')).toBeNull();
  });

  it('returns fallback and removes oversized values', () => {
    localStorage.setItem('huge', JSON.stringify([1, 2, 3]));

    expect(safeJsonStorage.read('huge', [], { validate: Array.isArray, maxLength: 2 })).toEqual([]);
    expect(localStorage.getItem('huge')).toBeNull();
  });
});
