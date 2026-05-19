import { describe, expect, test } from 'vitest';
import { parseId3v2Tags } from './id3Tags';

const synchsafe = (value) => [
  (value >> 21) & 0x7f,
  (value >> 14) & 0x7f,
  (value >> 7) & 0x7f,
  value & 0x7f
];

const frame = (id, value) => {
  const payload = new TextEncoder().encode(value);
  const size = payload.length + 1;
  return [
    ...new TextEncoder().encode(id),
    (size >> 24) & 0xff,
    (size >> 16) & 0xff,
    (size >> 8) & 0xff,
    size & 0xff,
    0,
    0,
    3,
    ...payload
  ];
};

const tag = (...frames) => {
  const body = frames.flat();
  return new Uint8Array([
    73,
    68,
    51,
    3,
    0,
    0,
    ...synchsafe(body.length),
    ...body
  ]).buffer;
};

describe('parseId3v2Tags', () => {
  test('reads common ID3v2 text frames', () => {
    const result = parseId3v2Tags(tag(
      frame('TIT2', 'Song Title'),
      frame('TPE1', 'Alice / Bob'),
      frame('TALB', 'Album Name')
    ));

    expect(result).toEqual({
      title: 'Song Title',
      artists: 'Alice / Bob',
      album: 'Album Name'
    });
  });

  test('returns an empty object when ID3 header is missing', () => {
    expect(parseId3v2Tags(new Uint8Array([1, 2, 3]).buffer)).toEqual({});
  });
});
