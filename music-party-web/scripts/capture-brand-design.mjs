import { chromium } from '@playwright/test';
import { mkdir } from 'node:fs/promises';
import path from 'node:path';

const output = path.resolve('../docs/assets/ui-baseline/2026-11-09/iterations', process.argv[2] || 'round-23');
const phase = process.argv[3] || 'before';
const viewports = [[1440, 900], [390, 844], [320, 700]];

await mkdir(output, { recursive: true });
const browser = await chromium.launch();

try {
  for (const [width, height] of viewports) {
    const page = await browser.newPage({ viewport: { width, height } });
    await page.goto('http://127.0.0.1:5173');
    await page.getByRole('button', { name: '以访客身份进入' }).waitFor();
    await page.evaluate(() => document.fonts.ready);
    await page.screenshot({ path: path.join(output, `${phase}-auth-${width}x${height}.png`), animations: 'disabled' });

    await page.getByRole('button', { name: '以访客身份进入' }).click();
    await page.getByLabel('昵称').fill('Visual review');
    await page.getByRole('button', { name: '立即进入' }).click();
    await page.getByRole('button', { name: /进入 Lounge/ }).waitFor();
    await page.evaluate(() => document.fonts.ready);
    await page.screenshot({ path: path.join(output, `${phase}-room-picker-${width}x${height}.png`), animations: 'disabled' });

    console.log(`${phase} ${width}x${height}: horizontal overflow=${await page.evaluate(() => document.documentElement.scrollWidth > innerWidth)}`);
    await page.close();
  }
} finally {
  await browser.close();
}
