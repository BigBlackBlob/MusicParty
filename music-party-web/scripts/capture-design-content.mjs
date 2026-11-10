import { chromium } from '@playwright/test';
import { mkdir, writeFile } from 'node:fs/promises';
import path from 'node:path';

// Fixtures live only in the browser. No queue or chat commands are sent.
const output = path.resolve('../docs/assets/ui-baseline/2026-11-09/iterations', process.argv[2] || 'round-17', 'content');
await mkdir(output, { recursive: true });
const browser = await chromium.launch();
const measurements = [];
try {
  for (const [width, height] of [[1440, 900], [390, 844], [320, 700]]) {
    const page = await browser.newPage({ viewport: { width, height } });
    await page.route('**/api/music/**', route => route.fulfill({ json: {} }));
    await page.route('**/api/theme/**', route => route.fulfill({ json: {} }));
    await page.goto('http://127.0.0.1:5173');
    await page.getByRole('button', { name: '以访客身份进入' }).click();
    await page.getByLabel('昵称').fill('Visual review');
    await page.getByRole('button', { name: '立即进入' }).click();
    await page.getByRole('button', { name: /进入 Lounge/ }).click();
    await page.waitForTimeout(700);
    await page.evaluate(() => {
      const root = document.querySelector('#app');
      const provides = root._vnode?.component?.appContext?.provides || {};
      const pinia = Reflect.ownKeys(provides).map(key => provides[key]).find(value => value?._s instanceof Map);
      if (!pinia) throw new Error(`Review fixture could not locate Pinia: ${Reflect.ownKeys(provides).map(String).join(',')}`);
      window.reviewPinia = pinia;
      window.reviewStoreIds = [...pinia._s.keys()];
      const runtime = pinia._s.get('room-runtime');
      const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="600" height="600"><rect width="600" height="600" fill="#13222a"/><circle cx="300" cy="310" r="198" fill="#c4a682"/><circle cx="300" cy="310" r="145" fill="#244451"/><circle cx="300" cy="310" r="88" fill="#162a34"/><text x="40" y="67" fill="#fff" font-size="24" font-family="sans-serif" letter-spacing="8">NIGHT STUDIES</text><text x="40" y="555" fill="#fff" font-size="15" font-family="sans-serif" letter-spacing="4">VISUAL REVIEW FIXTURE · 01</text></svg>`;
      const coverUrl = `data:image/svg+xml,${encodeURIComponent(svg)}`;
      const names = ['Night Studies', 'A Quiet Place Between Two Cities', 'After Hours', 'Home Again', 'Blue in Green', 'Open Water', 'Slow Motion', 'First Light'];
      const tracks = names.map((name, index) => ({ id: `visual-${index}`, name, artists: [index % 2 ? 'North Atlantic' : 'The Evening Sessions'], duration: 240000, platform: 'netease', coverUrl, url: '', needsProxy: false }));
      runtime.isPaused = true;
      runtime.nowPlaying = { music: tracks[1], currentPosition: 73000, enqueuedByName: 'Visual review', likedUserIds: [], likeMarkers: [], playEpoch: 1, positionUpdatedAt: Date.now() };
      runtime.queue = tracks.map((music, index) => ({ queueId: `visual-${index}`, music, enqueuedBy: { name: 'Visual review', publicId: 'review' }, status: 'READY' }));
      window.reviewRuntimeState = { nowPlaying: runtime.nowPlaying, queueLength: runtime.queue.length };
    });
    await page.waitForTimeout(500);
    async function shot(name) {
      await page.evaluate(() => document.fonts.ready);
      await page.screenshot({ path: path.join(output, `${name}-simulated-${width}x${height}.png`), animations: 'disabled' });
      measurements.push({ name, width, height, ...await page.evaluate(() => ({ horizontalOverflow: document.documentElement.scrollWidth > innerWidth, ...(window.reviewRuntimeState ? { fixtureState: window.reviewRuntimeState, stores: window.reviewStoreIds } : {}) })) });
    }
    await shot('playing');
    if (width < 768) {
      await page.getByRole('tab').nth(1).click();
      await page.waitForTimeout(650);
      await shot('queue');
      await page.getByRole('tab').nth(0).click();
      await page.waitForTimeout(650);
      await page.getByRole('button', { name: /Settings|设置/, exact: true }).click();
      await page.getByRole('navigation', { name: 'Settings sections' }).waitFor();
      measurements.push({ name: 'settings-nav', width, height, ...await page.locator('.settings-center__nav').evaluate(nav => ({
        nav: { width: nav.clientWidth, scrollWidth: nav.scrollWidth },
        options: [...nav.querySelectorAll('button')].map(button => ({ text: button.innerText, x: button.getBoundingClientRect().x, right: button.getBoundingClientRect().right, scrollWidth: button.scrollWidth, width: button.clientWidth }))
      })) });
      await shot('settings');
      await page.getByRole('button', { name: /Close|关闭/, exact: true }).click();
    }
    await page.evaluate(() => {
      const audio = window.reviewPinia._s.get('audio-playback');
      audio.isErrorState = true;
      audio.isBuffering = false;
    });
    await page.waitForTimeout(200);
    await shot('playback-error');
    await page.close();
  }
} finally { await browser.close(); }
await writeFile(path.join(output, 'measurements.json'), JSON.stringify(measurements, null, 2));
console.log(JSON.stringify(measurements));
