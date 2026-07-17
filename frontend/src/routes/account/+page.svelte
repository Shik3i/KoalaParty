<script lang="ts">
  import { onMount } from 'svelte';
  import { api, establish, type Principal } from '$lib/api';

  type Session = { id: string; createdAt: string; expiresAt: string; current: boolean };
  let me: Principal | null = null;
  let sessions: Session[] = [];
  let displayName = '';
  let currentPassword = '';
  let newPassword = '';
  let deletePassword = '';
  let error = '';
  let notice = '';
  let loading = true;
  let pending = '';

  onMount(async () => {
    try {
      me = await establish();
      displayName = me.displayName;
      if (me.accountId) sessions = await api('/api/account/sessions');
    } catch (e) {
      error = e instanceof Error ? e.message : 'Identity unavailable.';
    } finally {
      loading = false;
    }
  });

  async function run(name: string, action: () => Promise<void>, success: string) {
    if (pending) return;
    pending = name;
    error = '';
    notice = '';
    try {
      await action();
      notice = success;
    } catch (e) {
      error = e instanceof Error ? e.message : 'Account action failed.';
    } finally {
      pending = '';
    }
  }

  async function saveProfile() {
    await run(
      'profile',
      async () => {
        me = await api('/api/account/profile', { method: 'PATCH', body: JSON.stringify({ displayName }) });
      },
      'Display name updated.',
    );
  }

  async function changePassword() {
    await run(
      'password',
      async () => {
        await api('/api/account/password', { method: 'POST', body: JSON.stringify({ currentPassword, newPassword }) });
        currentPassword = '';
        newPassword = '';
        sessions = sessions.filter((session) => session.current);
      },
      'Password changed.',
    );
  }

  async function revoke(id: string) {
    await run(
      id,
      async () => {
        await api(`/api/account/sessions/${id}`, { method: 'DELETE' });
        sessions = sessions.filter((session) => session.id !== id);
      },
      'Session revoked.',
    );
  }

  async function revokeOthers() {
    await run(
      'sessions',
      async () => {
        await api('/api/account/sessions', { method: 'DELETE' });
        sessions = sessions.filter((session) => session.current);
      },
      'All other sessions revoked.',
    );
  }

  async function logout() {
    await run(
      'logout',
      async () => {
        await api('/api/accounts/logout', { method: 'POST' });
        location.href = '/';
      },
      '',
    );
  }

  async function deleteAccount() {
    if (!confirm('Delete your account, revoke every session, and anonymize retained room history?')) return;
    await run(
      'delete',
      async () => {
        await api('/api/account', { method: 'DELETE', body: JSON.stringify({ password: deletePassword }) });
        localStorage.removeItem('koalaparty.identity.v1');
        location.href = '/';
      },
      '',
    );
  }

  function date(value: string) {
    return new Date(value.includes('T') ? value : `${value.replace(' ', 'T')}Z`).toLocaleString();
  }
</script>

<svelte:head><title>Account · KoalaParty</title></svelte:head>
<main class="page">
  <h1>Account</h1>
  {#if loading}<p class="muted" role="status">Loading identity…</p>{:else if error && !me}<p class="error" role="alert">
      {error}
    </p>{:else if me}
    <section class="panel card">
      <div class="avatar">{me.displayName.slice(0, 1).toUpperCase()}</div>
      <div>
        <h2>{me.displayName}</h2>
        <p class="muted">{me.accountId ? 'Linked account' : 'Persistent anonymous identity'}</p>
      </div>
      <button class="secondary logout" disabled={pending === 'logout'} onclick={logout}>Log out</button>
    </section>
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    {#if notice}<p class="success" role="status">{notice}</p>{/if}
    {#if !me.accountId}<section class="panel notice">
        <h2>Protect your rooms</h2>
        <p>
          This identity belongs only to this browser. Create an account before clearing storage to preserve ownership.
        </p>
        <a class="button" href="/register">Create account</a><a class="button secondary" href="/login">Log in</a>
      </section>{:else}
      <div class="grid">
        <section class="panel section">
          <h2>Profile</h2>
          <form
            onsubmit={(e) => {
              e.preventDefault();
              saveProfile();
            }}
          >
            <label>Display name<input bind:value={displayName} minlength="1" maxlength="32" required /></label>
            <button disabled={!!pending}>{pending === 'profile' ? 'Saving…' : 'Save profile'}</button>
          </form>
        </section>
        <section class="panel section">
          <h2>Change password</h2>
          <form
            onsubmit={(e) => {
              e.preventDefault();
              changePassword();
            }}
          >
            <label
              >Current password<input
                type="password"
                bind:value={currentPassword}
                autocomplete="current-password"
                required
              /></label
            >
            <label
              >New password<input
                type="password"
                bind:value={newPassword}
                minlength="10"
                maxlength="128"
                autocomplete="new-password"
                required
              /></label
            >
            <button disabled={!!pending}>{pending === 'password' ? 'Changing…' : 'Change password'}</button>
          </form>
        </section>
      </div>
      <section class="panel section">
        <div class="section-title">
          <div>
            <h2>Active sessions</h2>
            <p>Devices currently signed in to this account.</p>
          </div>
          <button class="secondary" disabled={!!pending || sessions.length < 2} onclick={revokeOthers}
            >Log out other devices</button
          >
        </div>
        <ul class="sessions">
          {#each sessions as session}<li>
              <div>
                <b>{session.current ? 'This device' : 'Signed-in device'}</b><small
                  >Created {date(session.createdAt)} · expires {date(session.expiresAt)}</small
                >
              </div>
              {#if !session.current}<button class="ghost" disabled={!!pending} onclick={() => revoke(session.id)}
                  >Revoke</button
                >{/if}
            </li>{/each}
        </ul>
      </section>
      <section class="panel section danger-zone">
        <h2>Delete account</h2>
        <p>
          Revokes every session, anonymizes retained identity references, and closes rooms still owned by this account.
          Transfer rooms you want to keep first.
        </p>
        <label
          >Confirm password<input type="password" bind:value={deletePassword} autocomplete="current-password" /></label
        >
        <button class="danger" disabled={!!pending || !deletePassword} onclick={deleteAccount}
          >{pending === 'delete' ? 'Deleting…' : 'Delete account permanently'}</button
        >
      </section>
    {/if}
    <section class="panel notice">
      <h2>Local identity</h2>
      <p class="muted">ID: {me.identityId}</p>
      <p>There is no anonymous recovery key and no browser fingerprinting.</p>
    </section>
  {/if}
</main>

<style>
  .page {
    max-width: 900px;
    margin: 4rem auto;
    padding: 0 1rem;
  }
  .card,
  .notice,
  .section {
    padding: 1.5rem;
    margin: 1rem 0;
  }
  .card {
    display: flex;
    align-items: center;
    gap: 1rem;
  }
  .avatar {
    width: 3.5rem;
    height: 3.5rem;
    border-radius: 50%;
    display: grid;
    place-content: center;
    background: var(--accent-muted);
    font-weight: 900;
    font-size: 1.4rem;
  }
  .card h2,
  .section h2 {
    margin: 0;
  }
  .logout {
    margin-left: auto;
  }
  .notice .button {
    margin-right: 0.5rem;
  }
  .grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 1rem;
  }
  .section form {
    display: grid;
    gap: 1rem;
    margin-top: 1rem;
  }
  .section-title {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 1rem;
  }
  .section-title p {
    margin-bottom: 0;
    color: var(--text-muted);
  }
  .sessions {
    list-style: none;
    padding: 0;
    margin: 1rem 0 0;
  }
  .sessions li {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    padding: 0.8rem 0;
    border-top: 1px solid var(--border-subtle);
  }
  .sessions small {
    display: block;
    color: var(--text-muted);
    margin-top: 0.25rem;
  }
  .danger-zone {
    border-color: color-mix(in srgb, var(--danger) 45%, var(--border-subtle));
  }
  .danger-zone label {
    max-width: 420px;
    margin: 1rem 0;
  }
  .success {
    color: var(--success);
  }
  @media (max-width: 700px) {
    .grid {
      grid-template-columns: 1fr;
      gap: 0;
    }
    .section-title {
      align-items: stretch;
      flex-direction: column;
    }
  }
</style>
