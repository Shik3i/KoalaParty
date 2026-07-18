import { expect, test, type Page } from '@playwright/test';

async function identityId(page: Page) {
  return page.evaluate(() => JSON.parse(localStorage.getItem('koalaparty.identity.v1')!).id as string);
}
async function command(page: Page, roomId: string, type: string, payload: Record<string, unknown>) {
  return page.evaluate(
    async ({ roomId, type, payload }) => {
      const me = await fetch('/api/me').then((r) => r.json());
      const room = await fetch(`/api/rooms/${roomId}`).then((r) => r.json());
      const response = await fetch(`/api/rooms/${roomId}/commands`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrfToken },
        body: JSON.stringify({
          type,
          requestId: crypto.randomUUID(),
          expectedRevision: room.revision,
          payload,
        }),
      });
      return { status: response.status, body: await response.text() };
    },
    { roomId, type, payload },
  );
}

test('anonymous room synchronization and authoritative permissions', async ({ browser }) => {
  const ownerContext = await browser.newContext();
  const memberContext = await browser.newContext();
  const thirdContext = await browser.newContext();
  const owner = await ownerContext.newPage();
  const thirdPartyRequests: string[] = [];
  owner.on('request', (request) => {
    if (/youtube|ytimg/i.test(new URL(request.url()).hostname)) thirdPartyRequests.push(request.url());
  });
  await owner.goto('/');
  await owner.getByRole('button', { name: 'Create a room' }).click();
  await expect(owner).toHaveURL(/\/room\/([A-Z2-7]{16})$/);
  await expect(owner.locator('script[src*="youtube.com/iframe_api"]')).toHaveCount(0);
  await expect(owner.getByText(/you consent to loading YouTube's privacy-enhanced player/)).toBeVisible();
  expect(thirdPartyRequests).toEqual([]);
  await owner.getByRole('button', { name: 'Start watching' }).click();
  await expect(owner.locator('script[src*="youtube.com/iframe_api"]')).toHaveCount(1);
  const roomId = owner.url().split('/').at(-1)!;
  const sameIdentity = await ownerContext.newPage();
  await sameIdentity.goto(`/room/${roomId}`);
  await expect(sameIdentity.getByText('Live', { exact: true })).toBeVisible();
  await owner.getByLabel('YouTube URL').fill('https://youtu.be/M7lc1UVf-VE');
  await owner.getByRole('button', { name: 'Add to queue' }).click();
  await expect(owner.locator('.queue li')).toHaveCount(1);
  await owner.locator('.queue .icon').click();
  await expect(owner.locator('.queue li')).toHaveCount(0);
  const member = await memberContext.newPage();
  await member.goto(`/room/${roomId}`);
  await expect(member.locator('.room-header h1')).toBeVisible();
  await expect(owner.locator('.members li')).toHaveCount(2);
  await expect(member.locator('.members li')).toHaveCount(2);
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
  await member.getByLabel('YouTube URL').fill('https://youtu.be/dQw4w9WgXcQ');
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
  expect(await promo.locator('img').evaluateAll((images) => images.every((image) => image.naturalWidth > 0))).toBe(
    true,
  );

  await page.goto('/privacy');
  await expect(page.getByRole('heading', { name: 'Privacy Policy' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'YouTube' })).toBeVisible();
  await expect(page.getByText('admin@koalastuff.net')).toBeVisible();

  await page.goto('/imprint');
  await expect(page.getByRole('heading', { name: 'Legal Notice' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Copyright and trademarks' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Privacy Policy' })).toHaveAttribute('href', '/privacy');
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
});

test('mobile navigation and room empty states remain usable', async ({ browser }) => {
  const context = await browser.newContext({ viewport: { width: 390, height: 844 }, isMobile: true });
  const page = await context.newPage();
  await page.goto('/');
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  await expect(page.getByRole('navigation', { name: 'Main navigation' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Discover' })).toBeVisible();
  await page.getByRole('link', { name: 'Discover' }).click();
  await expect(page.getByRole('heading', { name: 'Invite-only early beta' })).toBeVisible();
  await page.goto('/');
  await page.getByRole('button', { name: 'Create a room' }).click();
  await expect(page).toHaveURL(/\/room\/[A-Z2-7]{16}$/);
  await expect(page.getByText('The queue is empty.')).toBeVisible();
  await expect(page.getByRole('option', { name: 'Public' })).toHaveCount(0);
  await page.getByRole('tab', { name: 'People' }).click();
  await expect(page.getByText('(you)')).toBeVisible();
  await context.close();
});

test('account room library, private invitations, transfer, sessions and deletion work end to end', async ({
  browser,
}) => {
  const suffix = Date.now().toString(36);
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
  await owner.getByRole('button', { name: 'Create a room' }).click();
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
