import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

const readSource = (path) => readFileSync(resolve(process.cwd(), path), 'utf8');

describe('accessible control semantics', () => {
  it('gives desktop shell icon and avatar controls accessible names', () => {
    const source = readSource('src/components/layout/MainLayout.vue');

    expect(source).toContain(':aria-label="t(\'rooms.currentRoom\', { name: currentRoomName })"');
    expect(source).toContain(':aria-label="t(\'search.searchAndAdd\')"');
    expect(source).toContain(':aria-label="t(\'settings.activeUsers\')"');
    expect(source).toContain(':aria-label="t(\'settings.toggleTheme\')"');
    expect(source).toContain(':aria-label="t(\'layout.editLayout\')"');
    expect(source).toContain(':aria-label="t(\'settings.title\')"');
  });

  it('gives playback icon controls accessible names', () => {
    const source = readSource('src/components/modules/NowPlayingModule.vue');

    expect(source).toContain(':aria-label="t(\'player.seek\')"');
    expect(source).toContain(':aria-label="t(\'player.shuffle\')"');
    expect(source).toContain(':aria-label="t(\'player.prevUnavailable\')"');
    expect(source).toContain(':aria-label="player.isPaused ? t(\'player.play\') : t(\'player.pause\')"');
    expect(source).toContain(':aria-label="t(\'player.next\')"');
    expect(source).toContain(':aria-label="isLiked ? t(\'player.unlike\') : t(\'player.like\')"');
    expect(source).toContain(':aria-label="t(\'personalPlaylists.saveSong\')"');
  });

  it('does not expose clickable track rows without a label or focus ring', () => {
    const source = readSource('src/components/ui/TrackListItem.vue');

    expect(source).toContain(':aria-label="clickable ? (ariaLabel || title) : undefined"');
    expect(source).toContain('focus-visible:ring-2');
  });

  it('gives room playlist icon controls accessible names', () => {
    const source = readSource('src/components/modules/RoomPlaylistsModule.vue');

    expect(source).toContain(':aria-label="t(\'common.back\')"');
    expect(source).toContain(':aria-label="t(\'roomPlaylists.playAll\')"');
    expect(source).toContain(':aria-label="t(\'queue.remove\')"');
  });
});
