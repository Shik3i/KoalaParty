// Release smoke test against a running KoalaParty with the REAL YouTube player.
// It automates the manual checklist in docs/testing.md: two viewers in sync, a
// shared pause on the same frame, player layout on desktop and mobile, theater
// and mini-player, fullscreen, a natural end that starts the next video with
// sound, a reload after the shared clock passed the end, skip, reload recovery,
// and the error state of an unplayable video.
//
//   BASE=http://127.0.0.1:18090 node scripts/smoke-youtube.mjs
//
// Run it against the exact image you are about to release, for example:
//   docker run -d --name kp-smoke -e KOALAPARTY_PRODUCTION=false \
//     -e KOALAPARTY_TRUSTED_ORIGINS=http://127.0.0.1:18090 -p 127.0.0.1:18090:8080 <image>
// It needs network access to YouTube and uses Playwright's Chromium from frontend/.
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const require = createRequire(resolve(root, 'frontend', 'package.json'));
const { chromium } = require('@playwright/test');

const BASE = process.env.BASE ?? 'http://127.0.0.1:18090';
const LONG = 'aqz-KE-bpKQ'; // Big Buck Bunny, 10:34
const SHORT = 'jNQXAC9IVRw'; // Me at the zoo, 0:19
const OTHER = 'M7lc1UVf-VE'; // YouTube API demo
const BROKEN = 'aaaaaaaaaaa'; // not playable

const results = [];
function check(name, ok, detail) {
  results.push({ name, ok });
  console.log(`${ok ? 'PASS' : 'FAIL'}  ${name}${detail === undefined ? '' : `  ${JSON.stringify(detail)}`}`);
}

const browser = await chromium.launch({ args: ['--autoplay-policy=no-user-gesture-required'] });
const newViewer = async () => {
  const context = await browser.newContext({ viewport: { width: 1280, height: 720 }, locale: 'en-US' });
  return context.newPage();
};
const snapshot = (page) =>
  page.evaluate(async () => fetch(location.pathname.replace('/room/', '/api/rooms/')).then((r) => r.json()));
const player = (page) =>
  page.evaluate(() => {
    const f = document.querySelector('iframe');
    const p = f && window.YT?.get ? window.YT.get(f.id) : null;
    const r = f?.getBoundingClientRect();
    return {
      iframe: !!f,
      video: p?.getVideoData?.()?.video_id ?? null,
      state: p?.getPlayerState?.() ?? null,
      time: p?.getCurrentTime?.() ?? null,
      muted: p?.isMuted?.() ?? null,
      rect: r ? [Math.round(r.left), Math.round(r.top), Math.round(r.width), Math.round(r.height)] : null,
      fullscreen: !!document.fullscreenElement,
      overflowX: document.documentElement.scrollWidth > innerWidth,
    };
  });
// Offset of this viewer from the room clock, in seconds.
const drift = (page) =>
  page.evaluate(async () => {
    const f = document.querySelector('iframe');
    const p = window.YT.get(f.id);
    const s = await fetch(location.pathname.replace('/room/', '/api/rooms/')).then((r) => r.json());
    const expected =
      s.playback.status === 'playing' ? s.playback.position + (Date.now() - s.serverTime) / 1000 : s.playback.position;
    return p.getCurrentTime() - expected;
  });
const add = async (page, id, mode = 'queue') => {
  const input = page.getByLabel('YouTube URL').first();
  await input.fill(`https://www.youtube.com/watch?v=${id}`);
  await input.press(mode === 'now' ? 'Shift+Enter' : 'Enter');
  await page.waitForTimeout(1500);
};
const waitFor = (page, fn, arg, timeout = 30_000) => page.waitForFunction(fn, arg, { timeout });
const isPlaying = (videoId) => {
  const f = document.querySelector('iframe');
  const p = f && window.YT?.get ? window.YT.get(f.id) : null;
  return p?.getVideoData?.()?.video_id === videoId && p.getPlayerState() === 1;
};

try {
  // 1–2: two viewers, same room, in sync; pause from the second lands on one frame.
  const a = await newViewer();
  await a.goto(BASE);
  await a.locator('.hero').getByRole('button', { name: 'Create a room' }).click();
  await a.waitForURL(/\/room\//);
  const room = a.url();
  await add(a, LONG, 'now');
  const b = await newViewer();
  await b.goto(room);
  await waitFor(a, isPlaying, LONG);
  await waitFor(b, isPlaying, LONG);
  const src = await b.locator('iframe').getAttribute('src');
  check('privacy-enhanced player loads for both viewers', src?.startsWith('https://www.youtube-nocookie.com/'), src);
  await a.waitForTimeout(15_000);
  const [da, db] = await Promise.all([drift(a), drift(b)]);
  check('both viewers play within 0.25 s of each other', Math.abs(da - db) < 0.25, { a: da.toFixed(3), b: db.toFixed(3) });
  await b.getByRole('button', { name: 'Pause', exact: true }).first().click();
  await a.waitForTimeout(3000);
  const [pa, pb, ps] = await Promise.all([player(a), player(b), snapshot(a)]);
  check('a pause leaves both viewers on the same frame', pa.state === 2 && pb.state === 2 && Math.abs(pa.time - pb.time) < 0.1 && Math.abs(pa.time - ps.playback.position) < 0.1, {
    a: pa.time?.toFixed(3),
    b: pb.time?.toFixed(3),
    server: ps.playback.position.toFixed(3),
  });
  await a.getByRole('button', { name: 'Play', exact: true }).first().click();
  await waitFor(a, isPlaying, LONG);

  // 3: layout on desktop and phone.
  const desktop = await player(a);
  check('desktop player is 16:9 without horizontal overflow', Math.abs(desktop.rect[2] / desktop.rect[3] - 16 / 9) < 0.01 && !desktop.overflowX, desktop.rect);
  await b.setViewportSize({ width: 390, height: 844 });
  await b.waitForTimeout(1000);
  const phone = await player(b);
  check('phone player fills the width at 16:9 without overflow', phone.rect[2] === 390 && Math.abs(phone.rect[2] / phone.rect[3] - 16 / 9) < 0.02 && !phone.overflowX, phone.rect);

  // 4: theater mode and the mini-player.
  await a.keyboard.press('t');
  await a.waitForTimeout(800);
  const theater = await a.evaluate(() => {
    const host = document.querySelector('.player-host').getBoundingClientRect();
    const f = document.querySelector('iframe').getBoundingClientRect();
    return { host: [Math.round(host.width), Math.round(host.height)], iframe: [Math.round(f.width), Math.round(f.height)], overflowX: document.documentElement.scrollWidth > innerWidth };
  });
  check('theater mode keeps the iframe equal to the enlarged player', JSON.stringify(theater.host) === JSON.stringify(theater.iframe) && !theater.overflowX, theater);
  await a.keyboard.press('t');
  await b.keyboard.press('m');
  await b.waitForTimeout(800);
  const mini = await b.evaluate(() => {
    const f = document.querySelector('iframe').getBoundingClientRect();
    const nav = [...document.querySelectorAll('nav')].map((n) => n.getBoundingClientRect()).find((r) => r.height > 0 && r.bottom >= innerHeight - 2);
    return { top: Math.round(f.top), bottom: Math.round(f.bottom), left: Math.round(f.left), right: Math.round(f.right), navTop: nav ? Math.round(nav.top) : innerHeight };
  });
  check('phone mini-player stays visible above the bottom navigation', mini.left >= 0 && mini.right <= 390 && mini.top >= 0 && mini.bottom <= mini.navTop, mini);
  await b.keyboard.press('m');
  await b.setViewportSize({ width: 1280, height: 720 });

  // 5: fullscreen fills the viewport, exit restores the bounds, playback resyncs.
  const before = await player(a);
  await a.getByRole('button', { name: 'Fullscreen', exact: true }).click();
  await a.waitForTimeout(1500);
  const inside = await player(a);
  check('fullscreen fills the viewport', inside.fullscreen && inside.rect[2] === 1280 && inside.rect[3] >= 718, inside.rect);
  await a.evaluate(() => document.exitFullscreen());
  await a.waitForTimeout(2500);
  const after = await player(a);
  const resync = await drift(a);
  check('leaving fullscreen restores the player and stays in sync', !after.fullscreen && JSON.stringify(after.rect) === JSON.stringify(before.rect) && Math.abs(resync) < 0.5, { before: before.rect, after: after.rect, drift: resync.toFixed(3) });

  // 6: a natural end starts the next queued video with sound for both viewers.
  await add(a, SHORT, 'now');
  await add(a, OTHER);
  await waitFor(a, isPlaying, SHORT);
  await waitFor(a, isPlaying, OTHER, 45_000);
  await waitFor(b, isPlaying, OTHER, 15_000);
  const [na, nb] = await Promise.all([player(a), player(b)]);
  check('the next video starts with sound for both viewers after a natural end', !na.muted && !nb.muted, { a: na.muted, b: nb.muted });
  // …and an empty queue ends in fullscreen: fullscreen closes, the iframe goes.
  await a.getByRole('button', { name: 'Fullscreen', exact: true }).click();
  await a.waitForTimeout(800);
  await a.locator('.scrubber-input').evaluate((input) => {
    input.value = String(Number(input.max) - 3);
    input.dispatchEvent(new Event('input', { bubbles: true }));
    input.dispatchEvent(new Event('change', { bubbles: true }));
  });
  await waitFor(a, () => !document.querySelector('iframe'), undefined, 30_000).catch(() => {});
  await a.waitForTimeout(1000);
  const idle = await player(a);
  const idleRoom = await snapshot(a);
  check('an empty queue ends fullscreen, removes the iframe and records the end', !idle.fullscreen && !idle.iframe && !idleRoom.playback.media && idleRoom.events.at(-1)?.type === 'media.ended', {
    fullscreen: idle.fullscreen,
    iframe: idle.iframe,
    last: idleRoom.events.at(-1)?.type,
  });
  const recapDialog = await a.locator('[role=dialog]').count();
  check('the party recap is offered as a notice, not a blocking dialog', recapDialog === 0);

  // 7: nobody watches while a video ends; a reload moves on instead of looping.
  await add(a, SHORT, 'now');
  await waitFor(a, isPlaying, SHORT);
  await add(a, LONG);
  await b.goto(`${BASE}/privacy`);
  const roomPath = new URL(room).pathname;
  await a.goto(`${BASE}/privacy`);
  await a.waitForTimeout(30_000);
  await a.goto(`${BASE}${roomPath}`);
  await waitFor(a, isPlaying, LONG, 45_000).catch(() => {});
  const reloaded = await player(a);
  const reloadedRoom = await snapshot(a);
  check('reload after the end reports it once and starts the next video', reloaded.video === LONG && reloaded.state === 1 && !reloaded.muted && reloadedRoom.events.filter((e) => e.type === 'media.ended' && e.payload?.duration < 30).length >= 1, {
    video: reloaded.video,
    state: reloaded.state,
  });

  // 8: skip, and position recovery after a reload.
  await add(a, SHORT);
  await a.getByRole('button', { name: /^Skip/ }).first().click();
  await waitFor(a, isPlaying, SHORT);
  check('skip starts the next queued video', (await player(a)).video === SHORT);
  await add(a, LONG, 'now');
  await waitFor(a, isPlaying, LONG);
  await a.waitForTimeout(6000);
  const beforeReload = (await player(a)).time;
  await a.reload();
  await waitFor(a, isPlaying, LONG);
  await a.waitForTimeout(3000);
  const afterReload = (await player(a)).time;
  check('a reload continues from the shared position', afterReload > beforeReload + 2 && Math.abs(await drift(a)) < 1, { beforeReload, afterReload });

  // 9: an unplayable video shows its error inside the player with both actions.
  await add(a, BROKEN, 'now');
  await a.locator('.player-error').waitFor({ timeout: 20_000 });
  const error = await a.evaluate(() => {
    const e = document.querySelector('.player-error');
    const p = document.querySelector('.player').getBoundingClientRect();
    const r = e.getBoundingClientRect();
    return {
      inside: r.top >= p.top - 1 && r.bottom <= p.bottom + 1,
      buttons: [...e.querySelectorAll('button')].map((b) => b.textContent.trim()),
      hint: e.querySelector('small')?.textContent.trim(),
    };
  });
  check('an unplayable video shows its error inside the player with retry and skip', error.inside && error.buttons.length === 2 && !/usually/i.test(error.hint ?? ''), error);
} catch (error) {
  check('smoke run completed', false, String(error).slice(0, 300));
} finally {
  await browser.close();
}

const failed = results.filter((r) => !r.ok).length;
console.log(`\n${results.length - failed}/${results.length} checks passed`);
process.exit(failed ? 1 : 0);
