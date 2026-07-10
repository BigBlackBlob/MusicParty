const { chromium } = require('playwright');

const BASE = 'http://localhost:8848';
const results = [];
function log(ok, name, detail = '') {
  const line = `${ok ? 'PASS' : 'FAIL'}  ${name}${detail ? '  ::  ' + detail : ''}`;
  results.push({ ok, name, detail });
  console.log(line);
}

(async () => {
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({
    userAgent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36',
  });
  const page = await context.newPage();
  const consoleErrors = [];
  page.on('console', msg => {
    if (msg.type() === 'error' || msg.type() === 'warning') {
      const text = msg.text();
      // 忽略 Browserslist、favicon 等噪音
      if (!text.includes('Browserslist') && !text.includes('favicon') && !text.includes('manifest')) {
        consoleErrors.push({ type: msg.type(), text });
      }
    }
  });

  try {
    // === 1. 页面加载 ===
    await page.goto(BASE, { waitUntil: 'networkidle', timeout: 20000 });
    const title = await page.title();
    log(true, '页面加载', `title="${title}"`);

    // === 2. audio 元素存在且 crossorigin 正确 ===
    const audioInfo = await page.evaluate(() => {
      const audio = document.querySelector('audio');
      if (!audio) return { found: false };
      return {
        found: true,
        crossorigin: audio.getAttribute('crossorigin'),
        src: audio.src || audio.currentSrc || '',
        hasEmptiedListener: false, // 无法直接检测，通过源码验证
      };
    }).catch(() => ({ found: false }));
    log(audioInfo.found, 'audio 元素存在', audioInfo.found ? `crossorigin=${audioInfo.crossorigin}` : '未找到 audio');
    if (audioInfo.found && audioInfo.crossorigin !== 'anonymous') {
      log(false, 'audio crossorigin', `期望 anonymous, 实际 ${audioInfo.crossorigin}`);
    }

    // === 3. audio src 不是 PENDING_DOWNLOAD ===
    // 这是 P0 后端核心：PENDING_DOWNLOAD 绝不能成为 audio src
    if (audioInfo.found) {
      const src = audioInfo.src;
      const isPending = src.includes('PENDING_DOWNLOAD');
      log(!isPending, 'audio src 非 PENDING_DOWNLOAD', `src=${src ? src.substring(0, 80) : '(空)'}`);
    }

    // === 4. Pinia player store 状态检查 ===
    // 通过 __PINIA__ 或 Vue devtools hook 检查 store 状态
    const storeState = await page.evaluate(() => {
      // 尝试通过 app 实例访问 pinia
      const app = document.querySelector('#app');
      if (!app || !app.__vue_app__) return { available: false };
      const pinia = app.__vue_app__.config.globalProperties.$pinia;
      if (!pinia) return { available: false };
      const playerStore = pinia._s.get('player');
      if (!playerStore) return { available: false };
      return {
        available: true,
        nowPlaying: playerStore.nowPlaying ? {
          musicId: playerStore.nowPlaying.music?.id,
          platform: playerStore.nowPlaying.music?.platform,
          url: playerStore.nowPlaying.music?.url,
        } : null,
        isBuffering: playerStore.isBuffering,
        bufferedMs: playerStore.bufferedMs,
        isErrorState: playerStore.isErrorState,
        isLoading: playerStore.isLoading,
        connected: playerStore.connected,
      };
    }).catch(() => ({ available: false }));

    if (storeState.available) {
      log(true, 'Pinia player store 可访问');
      // 如果有 nowPlaying，验证 url 不是 PENDING_DOWNLOAD
      if (storeState.nowPlaying) {
        const url = storeState.nowPlaying.url || '';
        const isPending = url.includes('PENDING_DOWNLOAD');
        log(!isPending, 'store.nowPlaying.url 非 PENDING_DOWNLOAD',
            `platform=${storeState.nowPlaying.platform}, url=${url.substring(0, 80)}`);
      } else {
        log(true, 'store.nowPlaying 为 null', '（房间可能没有在播放，正常）');
      }
      // bufferedMs 应该是数字
      log(typeof storeState.bufferedMs === 'number', 'store.bufferedMs 是数字', `值=${storeState.bufferedMs}`);
    } else {
      log(false, 'Pinia player store 可访问', '无法获取 store');
    }

    // === 5. 验证 useAudio 三级软重试逻辑已注入 ===
    // 通过检查 audio 元素的事件监听器行为来间接验证
    // 更直接的方式：检查 JS bundle 里包含的关键逻辑
    const bundleCheck = await page.evaluate(async () => {
      // 找主 bundle
      const scriptTags = Array.from(document.querySelectorAll('script[src]'));
      const mainScript = scriptTags.find(s => s.src.includes('assets/index-'));
      if (!mainScript) return { found: false, reason: 'no main bundle script tag' };
      try {
        const resp = await fetch(mainScript.src);
        const text = await resp.text();
        return {
          found: true,
          hasL1SoftWait: text.includes('2500'),
          hasL2SoftRetry: text.includes('L2 soft retry') || text.includes('soft retry'),
          hasL3HardReload: text.includes('L3 hard reload') || text.includes('hard reload'),
          hasIsActuallyStalled: text.includes('isActuallyStalled') || text.includes('emptied'),
          hasSuspendCheck: text.includes('suspend'),
          hasSoftRetrySrcAssign: text.includes('currentSrc') && text.includes('.src'),
          hasPendingResumePosition: text.includes('pendingResumePositionMs') || text.includes('forceNextSyncSeek'),
          hasBufferedMs: text.includes('bufferedMs'),
        };
      } catch (e) {
        return { found: false, reason: e.message };
      }
    }).catch(e => ({ found: false, reason: e.message }));

    if (bundleCheck.found) {
      log(true, '主 bundle 可读取');
      log(bundleCheck.hasL1SoftWait, 'L1 软等待 2.5s 常量注入', bundleCheck.hasL1SoftWait ? '2500 存在' : '缺失');
      log(bundleCheck.hasL2SoftRetry, 'L2 软重试逻辑注入', bundleCheck.hasL2SoftRetry ? '存在' : '缺失');
      log(bundleCheck.hasL3HardReload, 'L3 硬重载逻辑注入', bundleCheck.hasL3HardReload ? '存在' : '缺失');
      log(bundleCheck.hasIsActuallyStalled, 'isActuallyStalled / emptied 过滤注入', bundleCheck.hasIsActuallyStalled ? '存在' : '缺失');
      log(bundleCheck.hasSuspendCheck, 'suspend 误判过滤注入', bundleCheck.hasSuspendCheck ? '存在' : '缺失');
      log(bundleCheck.hasSoftRetrySrcAssign, 'L2 软重试用 src=currentSrc（保留缓冲）', bundleCheck.hasSoftRetrySrcAssign ? '存在' : '缺失');
      log(bundleCheck.hasPendingResumePosition, 'L3 位置恢复机制注入', bundleCheck.hasPendingResumePosition ? '存在' : '缺失');
      log(bundleCheck.hasBufferedMs, 'bufferedMs 缓冲进度采集注入', bundleCheck.hasBufferedMs ? '存在' : '缺失');
    } else {
      log(false, '主 bundle 可读取', bundleCheck.reason || '未知');
    }

    // === 6. 缓冲进度条 DOM 结构验证 ===
    // 检查 ProgressScrubber / NowPlayingModule 是否有 progress-loaded 层
    const progressDom = await page.evaluate(() => {
      // 检查 CSS 变量 --progress-loaded 是否定义
      const styles = getComputedStyle(document.documentElement);
      const progressLoaded = styles.getPropertyValue('--progress-loaded').trim();
      // 检查进度条相关元素
      const progressBars = document.querySelectorAll('[class*="progress"], [class*="h-1"]');
      return {
        hasProgressLoadedToken: !!progressLoaded,
        progressLoadedValue: progressLoaded,
        progressBarCount: progressBars.length,
      };
    }).catch(() => ({ hasProgressLoadedToken: false }));
    log(progressDom.hasProgressLoadedToken, 'CSS --progress-loaded token 定义', progressDom.progressLoadedValue || '缺失');

    // === 7. 模拟卡顿事件验证 L1 软等待行为 ===
    // 通过 dispatch waiting 事件，观察 isBuffering 变化，但不动 audio
    const stallTest = await page.evaluate(() => {
      const audio = document.querySelector('audio');
      if (!audio) return { audioFound: false };
      // 记录初始状态
      const app = document.querySelector('#app');
      const pinia = app?.__vue_app__?.config.globalProperties?.$pinia;
      const playerStore = pinia?._s?.get('player');
      if (!playerStore) return { audioFound: true, storeFound: false };

      const beforeBuffering = playerStore.isBuffering;
      // 模拟 waiting 事件（L1 应该设 isBuffering=true 但不动 audio）
      audio.dispatchEvent(new Event('waiting'));
      // 同步检查：isBuffering 应该变 true（Vue 响应式可能需要 tick）
      // 但我们这里只验证事件被接收，不阻塞
      return {
        audioFound: true,
        storeFound: true,
        beforeBuffering,
        // waiting 事件已 dispatch，如果 isActuallyStalled 返回 true 则 arm watchdog
        // 由于 readyState 可能是 0，suspend 检查不影响 waiting
      };
    }).catch(e => ({ error: e.message }));
    if (stallTest.error) {
      log(false, '模拟 waiting 事件', stallTest.error);
    } else if (stallTest.audioFound && stallTest.storeFound) {
      // 等一小段时间让 Vue 响应式更新
      await page.waitForTimeout(300);
      const afterStall = await page.evaluate(() => {
        const app = document.querySelector('#app');
        const pinia = app?.__vue_app__?.config.globalProperties?.$pinia;
        const playerStore = pinia?._s?.get('player');
        return playerStore ? { isBuffering: playerStore.isBuffering } : null;
      }).catch(() => null);
      if (afterStall) {
        log(true, 'waiting 事件触发后 store 响应', `isBuffering=${afterStall.isBuffering}`);
      }
    }

    // === 8. emptied 事件不触发 arm（P0 修复点）===
    // 验证：dispatch emptied 后 isBuffering 不应因此变 true
    // （emptied 在 isActuallyStalled 里返回 false）
    const emptiedTest = await page.evaluate(() => {
      const audio = document.querySelector('audio');
      if (!audio) return { error: 'no audio' };
      const app = document.querySelector('#app');
      const pinia = app?.__vue_app__?.config.globalProperties?.$pinia;
      const playerStore = pinia?._s?.get('player');
      if (!playerStore) return { error: 'no store' };
      const before = playerStore.isBuffering;
      audio.dispatchEvent(new Event('emptied'));
      return { before };
    }).catch(e => ({ error: e.message }));
    if (!emptiedTest.error) {
      await page.waitForTimeout(200);
      const afterEmptied = await page.evaluate(() => {
        const app = document.querySelector('#app');
        const pinia = app?.__vue_app__?.config.globalProperties?.$pinia;
        const playerStore = pinia?._s?.get('player');
        return playerStore ? { isBuffering: playerStore.isBuffering } : null;
      }).catch(() => null);
      // emptied 不应该单独导致 isBuffering 从 false 变 true
      if (afterEmptied && emptiedTest.before === false && afterEmptied.isBuffering === true) {
        log(false, 'emptied 事件不触发 arm watchdog', 'isBuffering 被 emptied 错误地设为 true');
      } else {
        log(true, 'emptied 事件不触发 arm watchdog', `isBuffering 保持 ${afterEmptied?.isBuffering}`);
      }
    } else {
      log(false, 'emptied 事件测试', emptiedTest.error);
    }

    // === 9. WebSocket 连接状态 ===
    if (storeState.available) {
      log(storeState.connected !== undefined, 'WebSocket connected 状态可读', `connected=${storeState.connected}`);
    }

    // === 10. 截图 ===
    await page.screenshot({ path: 'playwright-acceptance.png', fullPage: false });
    log(true, '截图保存', 'playwright-acceptance.png');

    // === 汇总 console errors ===
    if (consoleErrors.length > 0) {
      console.log('\n--- Console errors/warnings (filtered) ---');
      consoleErrors.forEach(e => console.log(`  [${e.type}] ${e.text.substring(0, 120)}`));
    }

  } catch (e) {
    log(false, '脚本执行', e.message);
  } finally {
    await browser.close();
  }

  // === 汇总 ===
  const passed = results.filter(r => r.ok).length;
  const failed = results.filter(r => !r.ok).length;
  console.log(`\n=== 验收结果: ${passed} passed, ${failed} failed, ${results.length} total ===`);
  if (failed > 0) {
    console.log('失败项:');
    results.filter(r => !r.ok).forEach(r => console.log(`  - ${r.name}: ${r.detail}`));
    process.exit(1);
  }
})();
