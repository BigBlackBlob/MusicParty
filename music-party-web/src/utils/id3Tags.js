const FRAME_TO_FIELD = {
  TIT2: 'title',
  TPE1: 'artists',
  TALB: 'album'
};

const textDecoder = (encoding) => {
  if (encoding === 1 || encoding === 2) return new TextDecoder('utf-16');
  return new TextDecoder('utf-8');
};

const trimNulls = (value) => String(value || '').replace(/\0+$/g, '').trim();

const readSynchsafe = (bytes, offset) =>
  (bytes[offset] << 21) | (bytes[offset + 1] << 14) | (bytes[offset + 2] << 7) | bytes[offset + 3];

const readUint32 = (bytes, offset) =>
  ((bytes[offset] << 24) | (bytes[offset + 1] << 16) | (bytes[offset + 2] << 8) | bytes[offset + 3]) >>> 0;

export function parseId3v2Tags(buffer) {
  const bytes = buffer instanceof Uint8Array ? buffer : new Uint8Array(buffer);
  if (bytes.length < 10) return {};
  if (String.fromCharCode(bytes[0], bytes[1], bytes[2]) !== 'ID3') return {};

  const version = bytes[3];
  const tagSize = readSynchsafe(bytes, 6);
  const tagEnd = Math.min(bytes.length, 10 + tagSize);
  const result = {};
  let offset = 10;

  while (offset + 10 <= tagEnd) {
    const frameId = String.fromCharCode(bytes[offset], bytes[offset + 1], bytes[offset + 2], bytes[offset + 3]);
    if (!/^[A-Z0-9]{4}$/.test(frameId)) break;
    const frameSize = version === 4 ? readSynchsafe(bytes, offset + 4) : readUint32(bytes, offset + 4);
    if (frameSize <= 0 || offset + 10 + frameSize > tagEnd) break;

    const field = FRAME_TO_FIELD[frameId];
    if (field && frameSize > 1) {
      const encoding = bytes[offset + 10];
      const frameBytes = bytes.slice(offset + 11, offset + 10 + frameSize);
      const value = trimNulls(textDecoder(encoding).decode(frameBytes));
      if (value) result[field] = value;
    }

    offset += 10 + frameSize;
  }

  return result;
}

export async function readAudioTagsFromFile(file) {
  const head = await file.slice(0, 256 * 1024).arrayBuffer();
  return parseId3v2Tags(head);
}
