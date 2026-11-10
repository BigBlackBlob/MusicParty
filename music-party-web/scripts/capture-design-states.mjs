import { chromium } from '@playwright/test';
import { mkdir } from 'node:fs/promises';
import path from 'node:path';

// Local review evidence only. Search responses are fixtures, never real provider calls.
const output = path.resolve('../docs/assets/ui-baseline/2026-11-09/iterations', process.argv[2] || 'round-14', 'coverage');
await mkdir(output, { recursive: true });
const browser = await chromium.launch();
const widths = [[1440, 900], [390, 844], [320, 700]];
async function shot(page, name) {
  await page.evaluate(() => document.fonts.ready);
  await page.screenshot({ path: path.join(output, `${name}.png`), animations: 'disabled' });
  console.log(name, await page.evaluate(() => ({ overflow: document.documentElement.scrollWidth > innerWidth })));
}
try {
  for (const [width, height] of widths) {
    const context = await browser.newContext({ viewport: { width, height } });
    const page = await context.newPage();
    const size = `${width}x${height}`;
    await page.goto('http://127.0.0.1:5173');
    await page.getByRole('button', { name: '以访客身份进入' }).waitFor();
    await shot(page, `auth-${size}`);
    await page.getByRole('button', { name: '以访客身份进入' }).click();
    await shot(page, `guest-name-${size}`);
    await page.getByLabel('昵称').fill('Visual review');
    await page.getByRole('button', { name: '立即进入' }).click();
    await page.getByRole('button', { name: /进入 Lounge/ }).waitFor();
    await shot(page, `room-picker-${size}`);
    await page.getByRole('button', { name: /进入 Lounge/ }).click();
    await page.waitForTimeout(650);
    await page.getByRole('button', { name: /Settings|设置/, exact: true }).click();
    await page.getByRole('navigation', { name: 'Settings sections' }).waitFor();
    await shot(page, `guest-settings-${size}`);
    await page.getByRole('button', { name: /Close|关闭/, exact: true }).click();
    if (width < 768) await page.getByRole('tab').nth(2).click();
    else await page.getByRole('button', { name: /Search and add|搜索并添加/ }).click();
    await page.waitForTimeout(650);
    let responseMode = 'results';
    let releasePending;
    await page.route('**/api/search/**', async route => {
      const mode = responseMode;
      if (mode === 'loading') await new Promise(resolve => { releasePending = resolve; });
      if (mode === 'error') return route.fulfill({ status: 503, json: { status: 503, code: 'REVIEW_FIXTURE', message: 'Search is temporarily unavailable.' } });
      const rows = mode === 'results' ? Array.from({ length: 7 }, (_, index) => ({
        id: `review-${index}`, name: ['Midnight City', 'Intro', 'A Real Hero', 'Nightcall', 'Everything in Its Right Place', 'Teardrop', 'Something About Us'][index],
        artists: [['M83'], ['The xx'], ['College', 'Electric Youth'], ['Kavinsky'], ['Radiohead'], ['Massive Attack'], ['Daft Punk']][index],
        duration: 240000, platform: 'netease', coverUrl: ''
      })) : [];
      return route.fulfill({ json: rows });
    });
    const input = page.getByPlaceholder(/Search for a track|搜索歌曲/);
    for (const mode of ['results', 'empty', 'loading', 'error']) {
      responseMode = mode;
      await input.fill(`review-${mode}`);
      await input.press('Enter');
      if (mode === 'loading') {
        await page.waitForTimeout(350);
        await shot(page, `search-loading-simulated-${size}`);
        releasePending?.();
        await page.waitForTimeout(250);
      } else {
        if (mode === 'error') await page.getByText('Search is temporarily unavailable.', { exact: true }).waitFor();
        await page.waitForTimeout(550);
        await shot(page, `search-${mode}-simulated-${size}`);
      }
    }
    await context.close();
  }
  for (const [width, height] of widths) {
    const context = await browser.newContext({ viewport: { width, height } });
    const page = await context.newPage();
    const size = `${width}x${height}`;
    await page.goto('http://127.0.0.1:5173');
    await page.getByRole('button', { name: '平台管理员登录' }).click();
    await shot(page, `admin-login-${size}`);
    await page.getByLabel('管理员账号').fill('visual-review-admin');
    await page.getByLabel('管理员密码').fill(process.env.VISUAL_REVIEW_PASSWORD || 'VisualReviewOnly!2026');
    await page.getByRole('button', { name: '平台管理员登录', exact: true }).click();
    await page.getByRole('button', { name: /进入 Lounge/ }).click();
    await page.waitForTimeout(650);
    await page.getByRole('button', { name: /Settings|设置/, exact: true }).click();
    const nav = page.getByRole('navigation', { name: 'Settings sections' });
    await nav.waitFor();
    for (let index = 0; index < await nav.getByRole('button').count(); index++) {
      await nav.getByRole('button').nth(index).click();
      await page.waitForTimeout(350);
      await shot(page, `admin-settings-${index}-${size}`);
    }
    await context.close();
  }
} finally {
  await browser.close();
}
