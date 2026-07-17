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
          expectedRevision: room.playback.revision,
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
  await owner.goto('/');
  await owner.getByRole('button', { name: 'Create a room' }).click();
  await expect(owner).toHaveURL(/\/room\/([A-Z2-7]{16})$/);
  const roomId = owner.url().split('/').at(-1)!;
  const member = await memberContext.newPage();
  await member.goto(`/room/${roomId}`);
  await expect(member.getByRole('heading', { name: /Koala|Wombat|Possum|Kookaburra/ })).toBeVisible();
  await expect(owner.locator('.members li')).toHaveCount(2);
  await expect(member.locator('.members li')).toHaveCount(2);
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
  const thirdId = await identityId(third);
  expect((await command(member, roomId, 'member.ban', { identityId: thirdId })).status).toBe(200);
  await third.reload();
  await expect(third.getByText('You are banned from this room.')).toBeVisible();
  await owner.reload();
  await expect(owner.getByText('(you)')).toBeVisible();
  await Promise.all([ownerContext.close(), memberContext.close(), thirdContext.close()]);
});
