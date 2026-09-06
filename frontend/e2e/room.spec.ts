import { expect, test, type Page } from '@playwright/test';
import { randomUUID } from 'node:crypto';

type FakePlayerHarness = {
  videoId: string;
  playCalls: number;
  stopCalls: number;
  destroyCalls: number;
  muteCalls: number;
  muted: boolean;
  currentTime: number;
  getPlayerState: () => number;
  finish: () => void;
  blockAutoplay: () => void;
  allowAutoplay: () => void;
  fail: (code: number) => void;
};

const E2E_VIDEO_ID = 'idempo12345';
const E2E_QUEUE_VIDEO_ID = 'queuex12345';

async function identityId(page: Page) {
  return page.evaluate(() => JSON.parse(localStorage.getItem('koalaparty.identity.v1')!).id as string);
}
async function command(
  page: Page,
  roomId: string,
  type: string,
  payload: Record<string, unknown>,
  requestId = randomUUID(),
) {
  return page.evaluate(
    async ({ roomId, type, payload, requestId }) => {
      const me = await fetch('/api/me').then((r) => r.json());
      const room = await fetch(`/api/rooms/${roomId}`).then((r) => r.json());
      const response = await fetch(`/api/rooms/${roomId}/commands`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrfToken },
        body: JSON.stringify({
          type,
          requestId,
          expectedRevision: room.revision,
          payload,
        }),
      });
      return { status: response.status, body: await response.text() };
    },
    { roomId, type, payload, requestId },
  );
}

const fakeYouTubeAPI = String.raw`
  (() => {
    class FakePlayer {
      constructor(mountPoint, options) {
        this.events = options.events;
        this.state = -1;
        this.currentTime = 0;
        this.duration = 100;
        this.videoId = '';
        this.playCalls = 0;
        this.stopCalls = 0;
        this.destroyCalls = 0;
        this.muteCalls = 0;
        this.muted = false;
        this.autoplayDenied = false;
        this.freezeAtWrappedEnd = false;
        this.lastTick = Date.now();
        this.iframe = document.createElement('iframe');
        mountPoint.replaceWith(this.iframe);
        window.__koalaFakePlayer = this;
        queueMicrotask(() => this.events.onReady());
      }
      loadVideoById(request) {
        this.videoId = request.videoId;
        this.freezeAtWrappedEnd = request.startSeconds >= this.duration;
        this.currentTime = this.freezeAtWrappedEnd ? 1 : request.startSeconds;
        this.lastTick = Date.now();
      }
      cueVideoById(request) {
        this.loadVideoById(request);
      }
      getIframe() { return this.iframe; }
      getVideoData() { return { video_id: this.videoId }; }
      getPlayerState() { return this.state; }
      getCurrentTime() {
        if (this.state === 1 && !this.freezeAtWrappedEnd) {
          const now = Date.now();
          this.currentTime += (now - this.lastTick) / 1000;
          this.lastTick = now;
        }
        return this.currentTime;
      }
      getDuration() { return this.duration; }
      getPlaybackRate() { return 1; }
      setPlaybackRate() {}
      playVideo() {
        if (this.autoplayDenied) {
          this.state = 2;
          this.events.onAutoplayBlocked();
          return;
        }
        this.playCalls += 1;
        this.lastTick = Date.now();
        this.state = 1;
        this.events.onStateChange({ data: 1 });
      }
      pauseVideo() {
        this.getCurrentTime();
        this.state = 2;
        this.events.onStateChange({ data: 2 });
      }
      seekTo(position) {
        this.freezeAtWrappedEnd = position >= this.duration;
        this.currentTime = this.freezeAtWrappedEnd ? 1 : position;
        this.lastTick = Date.now();
      }
      mute() { this.muteCalls += 1; this.muted = true; }
      unMute() { this.muted = false; }
      setVolume() {}
      unloadModule() {}
      stopVideo() { this.stopCalls += 1; }
      destroy() {
        this.destroyCalls += 1;
        this.iframe.remove();
      }
      finish() {
        this.currentTime = this.duration;
        this.state = 0;
        this.events.onStateChange({ data: 0 });
      }
      blockAutoplay() {
        this.autoplayDenied = true;
        this.state = 2;
        this.events.onAutoplayBlocked();
      }
      allowAutoplay() { this.autoplayDenied = false; }
      fail(code) { this.events.onError({ data: code }); }
    }
    window.YT = { Player: FakePlayer };
    window.onYouTubeIframeAPIReady?.();
  })();
`;

test('ended fullscreen playback is not restarted and its iframe is torn down', async ({ page }) => {
  await page.route('**/iframe_api', (route) =>
    route.fulfill({ status: 200, contentType: 'application/javascript', body: fakeYouTubeAPI }),
  );
  await page.goto('/');
  await page.locator('.hero').getByRole('button', { name: 'Create a room' }).click();
  await page.waitForFunction(
    () => !!(window as Window & { __koalaFakePlayer?: FakePlayerHarness }).__koalaFakePlayer?.videoId,
  );
  await page.getByRole('button', { name: 'Play', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Pause', exact: true })).toBeVisible();

  const automaticSkip = page.waitForResponse((response) => {
    if (!response.url().endsWith('/commands') || response.request().method() !== 'POST') return false;
    const body = response.request().postDataJSON();
    return body.type === 'playback.ended' && body.payload.position === 100 && body.payload.duration === 100;
  });
  const playCallsAtEnd = await page.evaluate(() => {
    const player = (window as Window & { __koalaFakePlayer: FakePlayerHarness }).__koalaFakePlayer;
    player.finish();
    document.dispatchEvent(new Event('fullscreenchange'));
    document.dispatchEvent(new Event('webkitfullscreenchange'));
    return player.playCalls;
  });
  expect((await automaticSkip).status()).toBe(200);
  await expect(page.getByText('Add a YouTube video to start watching.')).toBeVisible();
  expect(
    await page.evaluate(() => {
      const player = (window as Window & { __koalaFakePlayer: FakePlayerHarness }).__koalaFakePlayer;
      return {
        playCalls: player.playCalls,
        stopCalls: player.stopCalls,
        destroyCalls: player.destroyCalls,
        iframeCount: document.querySelectorAll('.player iframe').length,
      };
    }),
  ).toEqual({ playCalls: playCallsAtEnd, stopCalls: 1, destroyCalls: 1, iframeCount: 0 });
});

test('the next queued video starts when the iframe briefly retains the previous ENDED state', async ({ page }) => {
  await page.route('**/iframe_api', (route) =>
    route.fulfill({ status: 200, contentType: 'application/javascript', body: fakeYouTubeAPI }),
  );
  await page.goto('/');
  await page.locator('.hero').getByRole('button', { name: 'Create a room' }).click();
  await page.waitForFunction(
    () => !!(window as Window & { __koalaFakePlayer?: FakePlayerHarness }).__koalaFakePlayer?.videoId,
  );
  await page.getByLabel('YouTube URL').fill(`https://youtu.be/${E2E_QUEUE_VIDEO_ID}`);
  await page.getByRole('button', { name: 'Add to queue' }).click();
  await expect(page.locator('.queue li')).toHaveCount(1);
  await page.getByRole('button', { name: 'Play', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Pause', exact: true })).toBeVisible();
  await page.waitForFunction(
    () => (window as Window & { __koalaFakePlayer: FakePlayerHarness }).__koalaFakePlayer.playCalls > 0,
  );

  const automaticAdvance = page.waitForResponse((response) => {
    if (!response.url().endsWith('/commands') || response.request().method() !== 'POST') return false;
    return response.request().postDataJSON().type === 'playback.ended';
  });
  const playCallsBeforeEnd = await page.evaluate(() => {
    const player = (window as Window & { __koalaFakePlayer: FakePlayerHarness }).__koalaFakePlayer;
    player.finish();
    return player.playCalls;
  });

  expect((await automaticAdvance).status()).toBe(200);
  await expect(page.locator('.queue li')).toHaveCount(0);
  await expect
    .poll(() =>
      page.evaluate(() => {
        const player = (window as Window & { __koalaFakePlayer: FakePlayerHarness }).__koalaFakePlayer;
        return {
          videoId: player.videoId,
          playCalls: player.playCalls,
          muteCalls: player.muteCalls,
          muted: player.muted,
        };
      }),
    )
    .toEqual({
      videoId: E2E_QUEUE_VIDEO_ID,
      playCalls: playCallsBeforeEnd + 1,
      muteCalls: 0,
      muted: false,
    });
  await expect(page.getByRole('button', { name: 'Pause', exact: true })).toBeVisible();
});

test('an autoplay block never mutes media and offers an explicit sound-preserving retry', async ({ page }) => {
  await page.route('**/iframe_api', (route) =>
    route.fulfill({ status: 200, contentType: 'application/javascript', body: fakeYouTubeAPI }),
  );
  await page.goto('/');
  await page.locator('.hero').getByRole('button', { name: 'Create a room' }).click();
  await page.waitForFunction(
    () => !!(window as Window & { __koalaFakePlayer?: FakePlayerHarness }).__koalaFakePlayer?.videoId,
  );
  await page.getByRole('button', { name: 'Play', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Pause', exact: true })).toBeVisible();
  await page.evaluate(() =>
    (window as Window & { __koalaFakePlayer: FakePlayerHarness }).__koalaFakePlayer.blockAutoplay(),
  );
  await expect(page.getByRole('button', { name: 'Autoplay blocked — play with sound' })).toBeVisible();
  expect(
    await page.evaluate(() => {
      const player = (window as Window & { __koalaFakePlayer: FakePlayerHarness }).__koalaFakePlayer;
      return { muteCalls: player.muteCalls, muted: player.muted };
    }),
  ).toEqual({ muteCalls: 0, muted: false });
  const roomId = page.url().split('/').at(-1)!;
  const playCallsBeforePause = await page.evaluate(
    () => (window as Window & { __koalaFakePlayer: FakePlayerHarness }).__koalaFakePlayer.playCalls,
  );
  expect((await command(page, roomId, 'player.pause', { position: 0 })).status).toBe(200);
  await expect(page.getByRole('button', { name: 'Play', exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Autoplay blocked — play with sound' })).toHaveCount(0);
  expect(
    await page.evaluate(
      () => (window as Window & { __koalaFakePlayer: FakePlayerHarness }).__koalaFakePlayer.playCalls,
    ),
  ).toBe(playCallsBeforePause);
  await page.getByRole('button', { name: 'Play', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Autoplay blocked — play with sound' })).toBeVisible();
  await page.evaluate(() =>
    (window as Window & { __koalaFakePlayer: FakePlayerHarness }).__koalaFakePlayer.allowAutoplay(),
  );
  await page.getByRole('button', { name: 'Autoplay blocked — play with sound' }).click({ force: true });
  await expect(page.getByRole('button', { name: 'Autoplay blocked — play with sound' })).toHaveCount(0);
  expect(
    await page.evaluate(() => {
      const player = (window as Window & { __koalaFakePlayer: FakePlayerHarness }).__koalaFakePlayer;
      return { muteCalls: player.muteCalls, muted: player.muted };
    }),
  ).toEqual({ muteCalls: 0, muted: false });
});

test('a rejected native playback command immediately restores the authoritative player state', async ({ page }) => {
  await page.route('**/iframe_api', (route) =>
    route.fulfill({ status: 200, contentType: 'application/javascript', body: fakeYouTubeAPI }),
  );
  await page.goto('/');
  await page.locator('.hero').getByRole('button', { name: 'Create a room' }).click();
  await page.waitForFunction(
    () => !!(window as Window & { __koalaFakePlayer?: FakePlayerHarness }).__koalaFakePlayer?.videoId,
  );
  await page.getByRole('button', { name: 'Play', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Pause', exact: true })).toBeVisible();
  await page.waitForFunction(
    () => (window as Window & { __koalaFakePlayer: FakePlayerHarness }).__koalaFakePlayer.getPlayerState() === 1,
  );
  await page.waitForTimeout(3_200);

  let rejected = false;
  await page.route(/\/api\/rooms\/[^/]+\/commands$/, async (route) => {
    const request = route.request();
    if (!rejected && request.method() === 'POST') {
      rejected = true;
      await route.fulfill({
        status: 503,
        contentType: 'application/json',
        body: JSON.stringify({ error: { code: 'forced_failure', message: 'Playback command failed.' } }),
      });
      return;
    }
    await route.continue();
  });
  const playCallsBeforeFailure = await page.evaluate(
    () => (window as Window & { __koalaFakePlayer: FakePlayerHarness }).__koalaFakePlayer.playCalls,
  );
  await page.evaluate(() =>
    (window as Window & { __koalaFakePlayer: FakePlayerHarness }).__koalaFakePlayer.pauseVideo(),
  );
  await page.waitForFunction(
    ({ playCalls }) => {
      const player = (window as Window & { __koalaFakePlayer: FakePlayerHarness }).__koalaFakePlayer;
      return player.playCalls > playCalls && player.getPlayerState() === 1;
    },
    { playCalls: playCallsBeforeFailure },
  );
  expect(rejected).toBe(true);
  await expect(page.getByRole('button', { name: 'Pause', exact: true })).toBeVisible();
});

test('mobile player errors and notices do not overlap their controls or bottom navigation', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.route('**/iframe_api', (route) =>
    route.fulfill({ status: 200, contentType: 'application/javascript', body: fakeYouTubeAPI }),
  );
  await page.goto('/');
  await page.locator('.hero').getByRole('button', { name: 'Create a room' }).click();
  await page.waitForFunction(
    () => !!(window as Window & { __koalaFakePlayer?: FakePlayerHarness }).__koalaFakePlayer?.videoId,
  );
  await page.evaluate(() => (window as Window & { __koalaFakePlayer: FakePlayerHarness }).__koalaFakePlayer.fail(150));
  await expect(page.locator('.player-error')).toBeVisible();
  const overlayLayout = await page.locator('.player-error').evaluate((overlay) => {
    const bounds = overlay.getBoundingClientRect();
    const actions = overlay.querySelector('.player-error-actions')!.getBoundingClientRect();
    return {
      opaque: !getComputedStyle(overlay).backgroundColor.startsWith('rgba'),
      actionsInside: actions.top >= bounds.top && actions.bottom <= bounds.bottom,
      helperHidden: getComputedStyle(overlay.querySelector('small')!).display === 'none',
    };
  });
  expect(overlayLayout).toEqual({ opaque: true, actionsInside: true, helperHidden: true });

  await page.getByLabel('YouTube URL').fill('https://example.com/video');
  await page.getByRole('button', { name: 'Play now' }).click();
  const notice = page.locator('.status--error');
  await expect(notice).toHaveAttribute('role', 'alert');
  const fixedLayout = await page.evaluate(() => {
    const toast = document.querySelector<HTMLElement>('.status')!.getBoundingClientRect();
    const navigation = document.querySelector<HTMLElement>('.site-header nav')!.getBoundingClientRect();
    return { toastBottom: Math.round(toast.bottom), navigationTop: Math.round(navigation.top) };
  });
  expect(fixedLayout.toastBottom).toBeLessThanOrEqual(fixedLayout.navigationTop);
});

test('reload at the finished server position advances instead of looping the first second', async ({ page }) => {
  await page.route('**/iframe_api', (route) =>
    route.fulfill({ status: 200, contentType: 'application/javascript', body: fakeYouTubeAPI }),
  );
  await page.goto('/');
  await page.locator('.hero').getByRole('button', { name: 'Create a room' }).click();
  await page.waitForFunction(
    () => !!(window as Window & { __koalaFakePlayer?: FakePlayerHarness }).__koalaFakePlayer?.videoId,
  );
  const roomId = page.url().split('/').at(-1)!;
  await page.getByLabel('YouTube URL').fill(`https://youtu.be/${E2E_QUEUE_VIDEO_ID}`);
  await page.getByRole('button', { name: 'Add to queue' }).click();
  await page.getByRole('button', { name: 'Play', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Pause', exact: true })).toBeVisible();
  expect((await command(page, roomId, 'player.seek', { position: 100 })).status).toBe(200);

  const recoveredEnd = page.waitForResponse((response) => {
    if (!response.url().endsWith('/commands') || response.request().method() !== 'POST') return false;
    const body = response.request().postDataJSON();
    return body.type === 'playback.ended' && body.payload.position === 100 && body.payload.duration === 100;
  });
  await page.reload();
  expect((await recoveredEnd).status()).toBe(200);
  await expect(page.locator('.queue li')).toHaveCount(0);
  await expect
    .poll(() =>
      page.evaluate(() => {
        const player = (window as Window & { __koalaFakePlayer: FakePlayerHarness }).__koalaFakePlayer;
        return { videoId: player.videoId, playing: player.playCalls > 0 };
      }),
    )
    .toEqual({ videoId: E2E_QUEUE_VIDEO_ID, playing: true });
  expect(
    await page.evaluate(
      () => (window as Window & { __koalaFakePlayer: FakePlayerHarness }).__koalaFakePlayer.currentTime,
    ),
  ).toBeLessThan(10);
});

test('keyboard controls, manual resync, diagnostics download and reconnect stay usable', async ({ browser }) => {
  const context = await browser.newContext({ acceptDownloads: true });
  const page = await context.newPage();
  await page.route('**/iframe_api', (route) =>
    route.fulfill({ status: 200, contentType: 'application/javascript', body: fakeYouTubeAPI }),
  );
  await page.goto('/');
  await page.locator('.hero').getByRole('button', { name: 'Create a room' }).click();
  await page.waitForFunction(
    () => !!(window as Window & { __koalaFakePlayer?: FakePlayerHarness }).__koalaFakePlayer?.videoId,
  );
  const playerGeometry = await page.evaluate(() => {
    const container = document.querySelector('.player')!.getBoundingClientRect();
    const host = document.querySelector('.player-host')!.getBoundingClientRect();
    const iframe = document.querySelector('.player iframe')!.getBoundingClientRect();
    return {
      container: { width: container.width, height: container.height },
      host: { width: host.width, height: host.height },
      iframe: { width: iframe.width, height: iframe.height },
    };
  });
  expect(playerGeometry.container.width).toBeGreaterThan(700);
  expect(playerGeometry.container.height).toBeGreaterThan(390);
  expect(playerGeometry.host).toEqual(playerGeometry.container);
  expect(playerGeometry.iframe).toEqual(playerGeometry.container);
  await expect(page.locator('.player iframe')).toHaveAttribute('allow', /picture-in-picture/);
  await page.evaluate(() => (document.activeElement as HTMLElement | null)?.blur());
  await page.keyboard.press('k');
  await expect(page.getByRole('button', { name: 'Pause', exact: true })).toBeVisible();
  await page.keyboard.press('k');
  await expect(page.getByRole('button', { name: 'Play', exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Sync now' }).click();
  const download = page.waitForEvent('download');
  await page.getByRole('button', { name: 'Download' }).click();
  await expect((await download).suggestedFilename()).toMatch(/^koalaparty-[a-z2-7]{16}-diagnostics\.txt$/);

  await page.evaluate(() => window.dispatchEvent(new PageTransitionEvent('pagehide', { persisted: true })));
  await expect(page.locator('.connection')).toHaveText('Reconnecting');
  await page.evaluate(() => window.dispatchEvent(new PageTransitionEvent('pageshow', { persisted: true })));
  await expect(page.locator('.connection')).toHaveText('Live', { timeout: 15_000 });

  await context.setOffline(true);
  await expect(page.getByText('You are offline. Changes will resume after reconnecting.')).toBeVisible();
  await expect(page.locator('.connection')).toHaveText('Reconnecting');
  await context.setOffline(false);
  await expect(page.locator('.connection')).toHaveText('Live', { timeout: 15_000 });
  await context.close();
});

test('legacy anonymous koala names keep their animal badge', async ({ page }) => {
  await page.addInitScript(
    ({ id, secret }) => {
      localStorage.setItem(
        'koalaparty.identity.v1',
        JSON.stringify({ id, secret, displayName: 'Koala 474', avatarSeed: 'legacy-koala' }),
      );
    },
    { id: randomUUID(), secret: 'legacy-koala-secret'.padEnd(43, 'x') },
  );
  await page.goto('/');
  await page.locator('.hero').getByRole('button', { name: 'Create a room' }).click();
  const participant = page.locator('.members li').filter({ hasText: 'Koala 474' });
  await expect(participant).toBeVisible();
  await expect(participant.locator('.avatar')).toContainText('🐨');
});

test('anonymous room synchronization and authoritative permissions', async ({ browser }) => {
  const ownerContext = await browser.newContext();
  const memberContext = await browser.newContext();
  const thirdContext = await browser.newContext();
  for (const context of [ownerContext, memberContext, thirdContext]) {
    await context.route('**/iframe_api', (route) =>
      route.fulfill({ status: 200, contentType: 'application/javascript', body: fakeYouTubeAPI }),
    );
  }
  const owner = await ownerContext.newPage();
  await owner.goto('/');
  await owner.locator('.hero').getByRole('button', { name: 'Create a room' }).click();
  await expect(owner).toHaveURL(/\/room\/([A-Z2-7]{16})$/);
  // A new room starts with a preset cued and loads YouTube's player on entry.
  await expect(owner.getByText(/loads YouTube's privacy-enhanced player/)).toBeVisible();
  await expect(owner.locator('script[src*="youtube.com/iframe_api"]')).toHaveCount(1);
  const roomId = owner.url().split('/').at(-1)!;
  await owner.goto('/');
  await expect(owner.getByRole('heading', { name: 'Your recent rooms' })).toBeVisible();
  await expect(owner.getByText(/online · (Playing|Paused) at/)).toBeVisible();
  const recentRoom = owner.getByRole('link', { name: /^Open / });
  await expect(recentRoom).toHaveAttribute('href', `/room/${roomId}`);
  await recentRoom.click();
  await expect(owner).toHaveURL(`/room/${roomId}`);
  const sameIdentity = await ownerContext.newPage();
  await sameIdentity.goto(`/room/${roomId}`);
  await expect(sameIdentity.getByText('Live', { exact: true })).toBeVisible();
  await owner.getByLabel('YouTube URL').fill(`https://youtu.be/${E2E_QUEUE_VIDEO_ID}`);
  await owner.getByRole('button', { name: 'Add to queue' }).click();
  await expect(owner.locator('.queue li')).toHaveCount(1);
  const retryRequestId = randomUUID();
  expect((await command(owner, roomId, 'queue.add', { videoId: E2E_VIDEO_ID }, retryRequestId)).status).toBe(200);
  expect((await command(owner, roomId, 'queue.add', { videoId: E2E_VIDEO_ID }, retryRequestId)).status).toBe(200);
  expect(
    await owner.evaluate(
      async ({ id, videoId }) => {
        const snapshot = await fetch(`/api/rooms/${id}`).then((response) => response.json());
        return snapshot.queue.filter((item: { media: { providerId: string } }) => item.media.providerId === videoId)
          .length;
      },
      { id: roomId, videoId: E2E_VIDEO_ID },
    ),
  ).toBe(1);
  await owner.locator('.queue .icon').first().click();
  await expect(owner.locator('.queue li')).toHaveCount(1);
  await owner.locator('.queue .icon').first().click();
  await expect(owner.locator('.queue li')).toHaveCount(0);
  const playbackSpeed = owner.getByLabel('Playback speed');
  await expect(playbackSpeed).toBeVisible();
  await expect(owner.getByRole('button', { name: '🐘 First video' })).toBeVisible();
  await expect(owner.getByRole('button', { name: '🎵 Player demo' })).toBeVisible();
  const settingsButton = owner.getByRole('button', { name: 'Room settings' });
  await expect(settingsButton).toHaveAttribute('aria-expanded', 'false');
  await settingsButton.click();
  const closeSettingsButton = owner.getByRole('button', { name: 'Close settings' });
  await expect(closeSettingsButton).toHaveAttribute('aria-expanded', 'true');
  await expect(owner.locator('#room-settings')).toBeVisible();
  await closeSettingsButton.click();
  const playbackSpeedBox = await playbackSpeed.evaluate((node) => {
    const rect = node.getBoundingClientRect();
    return { width: rect.width, viewportWidth: window.innerWidth };
  });
  expect(playbackSpeedBox.width).toBeGreaterThan(120);
  expect(playbackSpeedBox.width).toBeLessThan(240);
  const member = await memberContext.newPage();
  await member.goto(`/room/${roomId}`);
  await expect(member.locator('.room-header h1')).toBeVisible();
  await expect(owner.locator('.members li')).toHaveCount(2);
  await expect(member.locator('.members li')).toHaveCount(2);
  await owner.getByLabel('YouTube URL').fill(`https://youtu.be/${E2E_QUEUE_VIDEO_ID}`);
  await owner.getByRole('button', { name: 'Add to queue' }).click();
  await expect(member.locator('.queue li')).toHaveCount(1);
  const ownerVote = owner.locator('.queue .vote');
  const memberVote = member.locator('.queue .vote');
  await ownerVote.click();
  await expect(ownerVote).toHaveClass(/active/);
  await expect(memberVote).not.toHaveClass(/active/);
  await expect(ownerVote).toContainText('1');
  await expect(memberVote).toContainText('1');
  await memberVote.click();
  await expect(ownerVote).toHaveClass(/active/);
  await expect(memberVote).toHaveClass(/active/);
  await expect(ownerVote).toContainText('2');
  await owner.locator('.queue .icon').click();
  await expect(member.locator('.queue li')).toHaveCount(0);
  await member.getByRole('button', { name: '❤️' }).click();
  await expect(owner.locator('.reaction-overlay').getByText('❤️')).toBeVisible();
  await expect(owner.getByText(/Perfectly synced|Buffering|s (behind|ahead)/)).toBeVisible();
  await owner.getByRole('button', { name: 'Float mini-player' }).click();
  await expect(owner.locator('.player-wrap')).toHaveClass(/mini-player/);
  expect(
    await owner.locator('.player-wrap').evaluate((node) => {
      const rect = node.getBoundingClientRect();
      return {
        position: getComputedStyle(node).position,
        rightGap: window.innerWidth - rect.right,
        bottomGap: window.innerHeight - rect.bottom,
        fullyVisible:
          rect.top >= 0 && rect.left >= 0 && rect.right <= window.innerWidth && rect.bottom <= window.innerHeight,
      };
    }),
  ).toEqual({ position: 'fixed', rightGap: 16, bottomGap: 16, fullyVisible: true });
  await owner.getByRole('button', { name: 'Dock player' }).click();
  await member.getByRole('button', { name: 'Play', exact: true }).click();
  await expect(member.getByRole('button', { name: 'Pause', exact: true })).toBeVisible();
  await member.waitForTimeout(1_100);
  await member.getByRole('button', { name: 'Pause', exact: true }).click();
  await expect(member.getByRole('button', { name: 'Play', exact: true })).toBeVisible();
  const pausedPosition = await member.evaluate(async (id) => {
    const snapshot = await fetch(`/api/rooms/${id}`).then((response) => response.json());
    return snapshot.playback.position as number;
  }, roomId);
  expect(pausedPosition).toBeGreaterThan(0.8);
  await member.getByRole('button', { name: 'Play', exact: true }).click();
  await expect(member.getByRole('button', { name: 'Pause', exact: true })).toBeVisible();
  // Use a video outside the random initial-preset pool so duplicate rejection
  // cannot make this synchronization assertion flaky.
  await member.getByLabel('YouTube URL').fill(`https://youtu.be/${E2E_VIDEO_ID}`);
  await member.getByRole('button', { name: 'Add to queue' }).click();
  await expect(owner.locator('.queue li')).toHaveCount(1);
  const memberId = await identityId(member);
  expect(
    (
      await command(owner, roomId, 'member.permission', {
        identityId: memberId,
        permission: 'playback.play_pause',
        allowed: false,
      })
    ).status,
  ).toBe(200);
  await member.reload();
  await expect(member.getByRole('button', { name: 'Pause', exact: true })).toBeDisabled();
  expect((await command(owner, roomId, 'member.role', { identityId: memberId, role: 'admin' })).status).toBe(200);
  await member.reload();
  await expect(member.getByRole('button', { name: 'Pause', exact: true })).toBeEnabled();
  const ownerId = await identityId(owner);
  expect((await command(member, roomId, 'member.role', { identityId: ownerId, role: 'member' })).status).toBe(403);
  const third = await thirdContext.newPage();
  await third.goto(`/room/${roomId}`);
  await expect(third.locator('.room-header h1')).toBeVisible();
  const thirdId = await identityId(third);
  expect((await command(member, roomId, 'member.ban', { identityId: thirdId })).status).toBe(200);
  await third.reload();
  await expect(third.getByText('You are banned from this room.')).toBeVisible();
  await owner.reload();
  await expect(owner.getByText('(you)')).toBeVisible();
  await Promise.all([ownerContext.close(), memberContext.close(), thirdContext.close()]);
});

test('KoalaSync promotion and legal pages are complete and responsive', async ({ page }) => {
  await page.goto('/');
  const promo = page.getByRole('region', { name: 'Take the watch party to almost any video site.' });
  await expect(promo).toBeVisible();
  await expect(promo.getByText('Netflix', { exact: true })).toBeVisible();
  await expect(promo.getByText('Disney+', { exact: true })).toBeVisible();
  await expect(promo.getByRole('link', { name: /See KoalaSync/ })).toHaveAttribute(
    'href',
    'https://sync.koalastuff.net/',
  );
  await expect
    .poll(() =>
      promo
        .locator('img')
        .evaluateAll((images) =>
          images
            .filter((image) => !image.complete || image.naturalWidth === 0)
            .map((image) => image.getAttribute('src')),
        ),
    )
    .toEqual([]);

  await page.goto('/privacy');
  await expect(page.getByRole('heading', { name: 'Privacy Policy' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'YouTube' })).toBeVisible();
  await expect(page.getByText('admin@koalastuff.net')).toBeVisible();

  await page.goto('/');
  await expect(page.getByRole('link', { name: 'Imprint' })).toHaveAttribute('href', 'https://koalastuff.net/legal');
});

test('mobile navigation and room empty states remain usable', async ({ browser }) => {
  const context = await browser.newContext({ viewport: { width: 390, height: 844 }, isMobile: true });
  const page = await context.newPage();
  await page.goto('/');
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  const navigation = page.getByRole('navigation', { name: 'Main navigation' });
  await expect(navigation).toBeVisible();
  const navigationBox = await navigation.evaluate((node) => {
    const rect = node.getBoundingClientRect();
    return { bottom: rect.bottom, top: rect.top, viewportHeight: window.innerHeight };
  });
  expect(Math.abs(navigationBox.bottom - navigationBox.viewportHeight)).toBeLessThanOrEqual(1);
  expect(navigationBox.top).toBeGreaterThan(700);
  await expect(page.locator('.brand img[src="/icons/koalaparty-192.png"]')).toHaveCount(1);
  await expect(page.getByRole('link', { name: 'Discover' })).toBeVisible();
  await page.getByRole('link', { name: 'Discover' }).click();
  await expect(page.getByRole('heading', { name: 'Invite-only early beta' })).toBeVisible();
  await page.goto('/');
  await page.locator('.hero').getByRole('button', { name: 'Create a room' }).click();
  await expect(page).toHaveURL(/\/room\/[A-Z2-7]{16}$/);
  await expect(page.getByText('The queue is empty.')).toBeVisible();
  await expect(page.getByRole('option', { name: 'Public' })).toHaveCount(0);
  await page.getByRole('button', { name: 'Float mini-player' }).click();
  const mobileMiniPlayer = await page.evaluate(() => {
    const player = document.querySelector('.player-wrap')!.getBoundingClientRect();
    const navigation = document.querySelector('.site-header nav')!.getBoundingClientRect();
    return {
      position: getComputedStyle(document.querySelector('.player-wrap')!).position,
      gap: navigation.top - player.bottom,
      fullyVisible: player.top >= 0 && player.left >= 0 && player.right <= innerWidth,
    };
  });
  expect(mobileMiniPlayer.position).toBe('fixed');
  expect(mobileMiniPlayer.gap).toBeGreaterThanOrEqual(15);
  expect(mobileMiniPlayer.fullyVisible).toBe(true);
  await page.getByRole('button', { name: 'Dock player' }).click();
  await page.getByRole('tab', { name: 'People' }).click();
  await expect(page.getByText('(you)')).toBeVisible();
  await context.close();
});

test('account room library, private invitations, transfer, sessions and deletion work end to end', async ({
  browser,
}) => {
  const suffix = `${Date.now().toString(36)}_${randomUUID().slice(0, 8)}`;
  const ownerName = `owner_${suffix}`;
  const memberName = `member_${suffix}`;
  const password = 'very-secure-password';
  const newPassword = 'an-even-better-password';
  const ownerContext = await browser.newContext();
  const memberContext = await browser.newContext();
  const owner = await ownerContext.newPage();
  const member = await memberContext.newPage();

  async function register(page: Page, username: string) {
    await page.goto('/register');
    await page.getByLabel('Username').fill(username);
    await page.getByLabel('Password').fill(password);
    await page.getByRole('button', { name: 'Create account' }).click();
    await expect(page).toHaveURL(/\/account$/);
    await expect(page.getByText('Linked account')).toBeVisible();
  }

  await register(owner, ownerName);
  await owner.goto('/');
  await owner.locator('.hero').getByRole('button', { name: 'Create a room' }).click();
  await expect(owner).toHaveURL(/\/room\/([A-Z2-7]{16})$/);
  const roomURL = owner.url();
  const roomLabel = await owner.locator('.room-header h1').textContent();

  await owner.goto('/rooms');
  await expect(owner.getByRole('heading', { name: roomLabel ?? '' })).toBeVisible();
  await owner.getByRole('link', { name: 'Open' }).click();
  await owner.getByRole('button', { name: 'Room settings' }).click();
  await owner.getByLabel('Visibility').selectOption('private');
  await expect(owner.locator('.visibility')).toHaveText('private');

  await register(member, memberName);
  await member.goto(roomURL);
  await expect(member.getByRole('heading', { name: 'Couldn’t enter this room' })).toBeVisible();

  await owner.getByLabel('Account username').fill(memberName);
  await owner.getByRole('button', { name: 'Invite', exact: true }).click();
  await expect(owner.getByText(memberName, { exact: true })).toBeVisible();
  await member.goto(roomURL);
  await expect(member.locator('.room-header h1')).toHaveText(roomLabel ?? '');

  await owner.getByRole('button', { name: 'Transfer', exact: true }).click();
  await owner.getByRole('alertdialog').getByRole('button', { name: 'Transfer' }).click();
  await expect(member.getByText('owner', { exact: true })).toBeVisible();

  await owner.getByRole('button', { name: 'Leave room' }).click();
  await owner.getByRole('alertdialog').getByRole('button', { name: 'Leave room' }).click();
  await expect(owner).toHaveURL(/\/rooms$/);

  await member.getByRole('button', { name: 'Room settings' }).click();
  await member.getByRole('button', { name: 'Delete room' }).click();
  await member.getByRole('alertdialog').getByRole('button', { name: 'Delete room' }).click();
  await expect(member).toHaveURL(/\/rooms$/);

  await owner.goto('/account');
  await owner.getByLabel('Display name').fill('Polished Koala');
  await owner.getByRole('button', { name: 'Save profile' }).click();
  await expect(owner.getByText('Display name updated.')).toBeVisible();
  await owner.getByLabel('Current password').fill(password);
  await owner.getByLabel('New password').fill(newPassword);
  await owner.getByRole('button', { name: 'Change password' }).click();
  await expect(owner.getByText('Password changed.')).toBeVisible();
  await owner.getByLabel('Confirm password').fill(newPassword);
  owner.once('dialog', (dialog) => dialog.accept());
  await owner.getByRole('button', { name: 'Delete account permanently' }).click();
  await expect(owner).toHaveURL(/\/$/);

  await ownerContext.close();
  await memberContext.close();
});
