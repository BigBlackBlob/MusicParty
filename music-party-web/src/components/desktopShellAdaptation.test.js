import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

const readSource = (path) => readFileSync(resolve(process.cwd(), path), 'utf8');

describe('desktop shell adaptation', () => {
  it('avoids viewport-width shell sizing that can create desktop overflow', () => {
    const app = readSource('src/App.vue');
    const mainLayout = readSource('src/components/layout/MainLayout.vue');
    const styles = readSource('src/style.css');

    expect(app).not.toContain('app-viewport w-screen');
    expect(mainLayout).not.toContain('w-screen');
    expect(styles).not.toContain('width: 100vw;');
    expect(styles).toContain('width: 100%;');
  });

  it('uses transform scale instead of CSS zoom for the desktop stage', () => {
    const mainLayout = readSource('src/components/layout/MainLayout.vue');

    expect(mainLayout).not.toContain('zoom: uiStore.globalZoomLevel');
    expect(mainLayout).toContain('transform: `scale(${uiStore.globalZoomLevel})`');
    expect(mainLayout).toContain("transformOrigin: 'center center'");
    expect(mainLayout).toContain('overflow-visible');
  });
});
