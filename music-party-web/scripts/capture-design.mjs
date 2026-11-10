import { chromium } from '@playwright/test';
import { mkdir } from 'node:fs/promises';
import path from 'node:path';

const output = path.resolve('../docs/assets/ui-baseline/2026-11-09/iterations', process.argv[2] || 'round-06');
const phase = process.argv[3] || 'before';
await mkdir(output, { recursive: true });
const browser = await chromium.launch();
try {
  const page = await browser.newPage();
  await page.goto('http://127.0.0.1:5173');
  await page.getByRole('button', { name: '以访客身份进入' }).click();
  await page.getByLabel('昵称').fill('Visual review');
  await page.getByRole('button', { name: '立即进入' }).click();
  await page.getByRole('button', { name: /进入 Lounge/ }).click();
  await page.waitForTimeout(800);
  for (const [width, height] of [[390,844],[320,700],[1440,900]]) {
    await page.setViewportSize({ width, height });
    await page.waitForTimeout(500);
    const tabs = width < 768 ? ['now', 'queue', 'search', 'chat'] : ['desktop'];
    for (const [index, tab] of tabs.entries()) {
      if (width < 768) await page.getByRole('tab').nth(index).click();
      await page.waitForTimeout(600);
      await page.evaluate(() => document.fonts.ready);
      await page.screenshot({ path: path.join(output, `${phase}-${tab}-${width}x${height}.png`), animations: 'disabled' });
      const overflow = await page.evaluate(() => document.documentElement.scrollWidth > innerWidth);
      console.log(`${phase} ${tab} ${width}x${height}: horizontal overflow=${overflow}`);
    }
    if (width < 768) {
      await page.getByRole('tab').nth(0).click();
      await page.waitForTimeout(400);
      await page.getByRole('button', { name: /Settings|设置/ }).click();
      await page.getByRole('navigation', { name: 'Settings sections' }).waitFor();
      await page.screenshot({ path: path.join(output, `${phase}-settings-${width}x${height}.png`), animations: 'disabled' });
      const nav = await page.locator('.settings-center__nav').evaluate(element => ({ scrollWidth: element.scrollWidth, clientWidth: element.clientWidth }));
      console.log(`${phase} settings ${width}x${height}: nav width=${nav.clientWidth}/${nav.scrollWidth}`);
      await page.getByRole('button', { name: /Close|关闭/ }).click();
    }
  }
} finally {
  await browser.close();
}
