import { readFile, writeFile } from 'node:fs/promises';
import path from 'node:path';

const base = path.resolve('../docs/assets/ui-baseline/2026-11-09/iterations');
const rounds = [
  [8, 6.94, '全屏背景纯黑，提亮次要文字。'],
  [9, 7, '搜索顶部采用与队列、聊天一致的表面。'],
  [10, 7.06, '320×700 播放页缩小封面和间距，完整显示控制区。'],
  [11, 7.06, '移动空封面去渐变与光晕，保留音乐符号。'],
  [12, 7.125, '移动内容标题、搜索输入及结果名统一白色。'],
  [13, 7.1875, '移动分段控件统一轨道、边框、圆角和选中文字。'],
  [14, 7.65, '个人资料改黑底分组，修复无效颜色 token；补拍42张状态图。'],
  [15, 7.66, '修复管理员导航高度、结果翻译键、主按钮色；统一入口与后台控件，区分错误与无结果。'],
  [16, 7.5, '搜索与设置采用更克制的黑色表面，收紧管理员导航并调整空播放区。'],
  [17, 7.3625, '移动搜索页压缩顶部间距与分段控件高度，增加窄屏结果区空间。'],
  [18, 6.4, '桌面无曲目状态移除空封面方框，保留低调的均衡器符号与全部播放控制。'],
  [19, 6.8, '复评切换为直接浏览器检查；修正仅本地开发 API 的 WebSocket Origin 配置后进入播放器。'],
  [20, 7, '采用方案 C，空播放状态改为左对齐的大标题、简短说明和轻量音频标记；保留播放控制、队列和移动导航。'],
  [21, 6.375, '移动空播放状态垂直居中，保留标题与轻量音频标记。'],
  [22, 8.3, '移动队列、搜索、聊天等空状态统一左对齐标题层级与紧凑图形标记。'],
  [23, 8.13, '认证和选房页统一两级文字字标与均衡器识别记号。']
];
const pairs = [
  ['第10轮：320px 播放控制区', 'round-10/before-now-320x700.png', 'round-10/after-now-320x700.png'],
  ['第14轮：个人资料页', 'round-14/before-settings-390x844.png', 'round-14/after-settings-390x844.png'],
  ['第15轮：Auth', 'round-14/coverage/auth-390x844.png', 'round-15/coverage/auth-390x844.png'],
  ['第15轮：管理员导航', 'round-14/coverage/admin-settings-2-390x844.png', 'round-15/coverage/admin-settings-2-390x844.png'],
  ['第17轮：移动搜索 390px', 'round-17/before-search-390x844.png', 'round-17/after-search-390x844.png'],
  ['第17轮：移动搜索 320px', 'round-17/before-search-320x700.png', 'round-17/after-search-320x700.png'],
  ['第18轮：桌面空播放态', 'round-18/before-desktop-1440x900.png', 'round-18/after-desktop-1440x900.png'],
  ['第20轮：桌面空播放态', 'round-20/before-desktop-1440x900.png', 'round-20/after-desktop-1440x900.png'],
  ['第20轮：移动空播放态 390px', 'round-20/before-now-390x844.png', 'round-20/after-now-390x844.png'],
  ['第20轮：移动空播放态 320px', 'round-20/before-now-320x700.png', 'round-20/after-now-320x700.png']
  ,['第22轮：移动 Queue 空态 390px', 'round-22/before-queue-390x844.png', 'round-22/after-queue-390x844.png']
  ,['第22轮：移动 Search 空态 390px', 'round-22/before-search-390x844.png', 'round-22/after-search-390x844.png']
  ,['第22轮：移动 Chat 空态 390px', 'round-22/before-chat-390x844.png', 'round-22/after-chat-390x844.png']
  ,['第22轮：移动 Queue 空态 320px', 'round-22/before-queue-320x700.png', 'round-22/after-queue-320x700.png']
  ,['第23轮：认证页品牌字标 390px', 'round-23/before-auth-390x844.png', 'round-23/after-auth-390x844.png']
  ,['第23轮：选房页品牌字标 1440px', 'round-23/before-room-picker-1440x900.png', 'round-23/after-room-picker-1440x900.png']
  ,['第23轮：选房页品牌字标 320px', 'round-23/before-room-picker-320x700.png', 'round-23/after-room-picker-320x700.png']
];
const figures = await Promise.all(pairs.map(async ([title, before, after]) => {
  const images = await Promise.all([before, after].map(async file => `data:image/png;base64,${(await readFile(path.join(base, file))).toString('base64')}`));
  return `<section><h2>${title}</h2><div class="pair">${images.map((src, i) => `<figure><figcaption>${i ? '调整后' : '调整前'}</figcaption><img src="${src}" alt="${title}${i ? '调整后' : '调整前'}"></figure>`).join('')}</div></section>`;
}));
const firstRound = rounds[0][0];
const lastRound = rounds.at(-1)[0];
const x = round => 35 + ((round - firstRound) / (lastRound - firstRound || 1)) * 555;
const y = score => 160 - ((score - 6) / 3) * 125;
const targetY = y(8);
const points = rounds.filter(([, score]) => score !== null).map(([round, score]) => `${x(round)},${y(score)}`).join(' ');
const html = `<!doctype html><html lang="zh-CN"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>MusicParty 视觉迭代记录</title><style>
*{box-sizing:border-box}body{margin:0;background:#000;color:#fff;font:14px/1.6 system-ui,sans-serif}main{max-width:1000px;margin:auto;padding:24px}h1{font-size:24px}h2{font-size:18px;margin:32px 0 12px}p{color:#bbb}table{width:100%;border-collapse:collapse}td,th{padding:10px;text-align:left;border-bottom:1px solid #333}th{color:#fff}.pair{display:grid;grid-template-columns:1fr 1fr;gap:16px;max-width:650px}figure{margin:0}figcaption{padding-bottom:8px;color:#bbb}img{width:100%;height:auto;border:1px solid #333}svg{width:100%;max-width:600px;height:auto}section{border-top:1px solid #333;margin-top:24px}@media(max-width:500px){main{padding:16px}td,th{padding:7px;font-size:12px}}
</style><main><h1>MusicParty 视觉迭代记录</h1><p>2026-11-10。方案 C，字体主导、左对齐。第23轮八项最低8分，均分8.13。评审由 gpt-6.1-sol medium 独立完成。禁用 Impeccable 与 Nielsen 启发式；保留产品操作与流程。</p>
<svg viewBox="0 0 620 210" role="img" aria-label="第8至23轮平均分趋势"><line x1="35" x2="590" y1="${targetY}" y2="${targetY}" stroke="#777" stroke-dasharray="4 4"/><text x="590" y="${targetY - 5}" fill="#aaa" text-anchor="end" font-size="12">8分参考线</text><polyline points="${points}" fill="none" stroke="#d3c2f3" stroke-width="3"/>${rounds.filter(([,score])=>score!==null).map(([round,score])=>`<circle cx="${x(round)}" cy="${y(score)}" r="4" fill="#fff"/><text x="${x(round)}" y="190" fill="#bbb" text-anchor="middle" font-size="12">R${round}</text>`).join('')}</svg>
<table><thead><tr><th>轮次</th><th>均分/10</th><th>改动</th></tr></thead><tbody>${rounds.map(([round,score,change])=>`<tr><td>${round}</td><td>${score===null?'评审中':score.toFixed(2)}</td><td>${change}</td></tr>`).join('')}</tbody></table>
<p>第20轮验证：构建、类型检查与 git diff --check 通过；Playwright 实际页面采集覆盖 320×700、390×844、1440×900，均未见横向溢出。桌面搜索和移动导航入口仍可到达。原始前后 PNG 保存在 docs/assets/ui-baseline/2026-11-09/iterations/round-20。</p>
${figures.join('')}<p>证据纠正：第8轮按逐项分数计算为6.94；旧的部分Settings截图被身份弹窗覆盖，应使用coverage/guest-settings对应尺寸。所有评分为当轮独立判断，平均分不等同于每项达标。</p></main></html>`;
await writeFile(path.resolve('../temp-html/hill-climb-progress.html'), html);
console.log(`Wrote progress HTML: ${Buffer.byteLength(html)} bytes`);
