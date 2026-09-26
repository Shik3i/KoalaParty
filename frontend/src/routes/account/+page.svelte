<script lang="ts">
  import { errorText, locale, t } from '$lib/i18n';
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

  async function load() {
    loading = true;
    error = '';
    try {
      me = await establish();
      displayName = me.displayName;
      if (me.accountId) sessions = await api('/api/account/sessions');
    } catch (e) {
      error = errorText(e);
    } finally {
      loading = false;
    }
  }
  onMount(load);

  async function run(name: string, action: () => Promise<void>, success: string) {
    if (pending) return;
    pending = name;
    error = '';
    notice = '';
    try {
      await action();
      notice = success;
    } catch (e) {
      error = errorText(e);
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
      $t('account.nameUpdated'),
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
      $t('account.passwordChanged'),
    );
  }

  async function revoke(id: string) {
    await run(
      id,
      async () => {
        await api(`/api/account/sessions/${id}`, { method: 'DELETE' });
        sessions = sessions.filter((session) => session.id !== id);
      },
      $t('account.sessionRevoked'),
    );
  }

  async function revokeOthers() {
    await run(
      'sessions',
      async () => {
        await api('/api/account/sessions', { method: 'DELETE' });
        sessions = sessions.filter((session) => session.current);
      },
      $t('account.othersRevoked'),
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
    if (!confirm($t('account.confirmDelete'))) return;
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
    return new Date(value.includes('T') ? value : `${value.replace(' ', 'T')}Z`).toLocaleString($locale);
  }
</script>

<svelte:head><title>{$t('nav.account')} · KoalaParty</title></svelte:head>
<main class="page">
  <h1>{$t('nav.account')}</h1>
  {#if loading}<p class="muted" role="status">{$t('account.loading')}</p>{:else if error && !me}<section
      class="panel error-state"
      role="alert"
    >
      <h2>{$t('account.loadFailed')}</h2>
      <p class="error">{error}</p>
      <button onclick={load}>{$t('player.tryAgain')}</button>
    </section>{:else if me}
    <section class="panel card">
      <div class="avatar">{me.displayName.slice(0, 1).toUpperCase()}</div>
      <div>
        <h2>{me.displayName}</h2>
        <p class="muted">{me.accountId ? $t('account.linked') : $t('account.anonymous')}</p>
      </div>
      <button class="secondary logout" disabled={pending === 'logout'} onclick={logout}>{$t('account.logout')}</button>
    </section>
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    {#if notice}<p class="success" role="status">{notice}</p>{/if}
    {#if !me.accountId}<section class="panel section">
        <h2>{$t('account.profile')}</h2>
        <form
          onsubmit={(e) => {
            e.preventDefault();
            saveProfile();
          }}
        >
          <label
            >{$t('account.displayName')}<input bind:value={displayName} minlength="1" maxlength="32" required /></label
          >
          <button disabled={!!pending}
            >{pending === 'profile' ? $t('account.saving') : $t('account.saveProfile')}</button
          >
        </form>
      </section>
      <section class="panel notice">
        <h2>{$t('account.protect')}</h2>
        <p>{$t('account.protectBody')}</p>
        <a class="button" href="/register">{$t('auth.createAccount')}</a><a class="button secondary" href="/login"
          >{$t('auth.login')}</a
        >
      </section>{:else}
      <div class="grid">
        <section class="panel section">
          <h2>{$t('account.profile')}</h2>
          <form
            onsubmit={(e) => {
              e.preventDefault();
              saveProfile();
            }}
          >
            <label
              >{$t('account.displayName')}<input
                bind:value={displayName}
                minlength="1"
                maxlength="32"
                required
              /></label
            >
            <button disabled={!!pending}
              >{pending === 'profile' ? $t('account.saving') : $t('account.saveProfile')}</button
            >
          </form>
        </section>
        <section class="panel section">
          <h2>{$t('account.changePassword')}</h2>
          <form
            onsubmit={(e) => {
              e.preventDefault();
              changePassword();
            }}
          >
            <label
              >{$t('account.currentPassword')}<input
                type="password"
                bind:value={currentPassword}
                autocomplete="current-password"
                required
              /></label
            >
            <label
              >{$t('account.newPassword')}<input
                type="password"
                bind:value={newPassword}
                minlength="10"
                maxlength="128"
                autocomplete="new-password"
                required
              /></label
            >
            <button disabled={!!pending}
              >{pending === 'password' ? $t('account.changing') : $t('account.changePassword')}</button
            >
          </form>
        </section>
      </div>
      <section class="panel section">
        <div class="section-title">
          <div>
            <h2>{$t('account.sessions')}</h2>
            <p>{$t('account.sessionsBody')}</p>
          </div>
          <button class="secondary" disabled={!!pending || sessions.length < 2} onclick={revokeOthers}
            >{$t('account.logoutOthers')}</button
          >
        </div>
        {#if !sessions.length}
          <p class="muted" role="status">
            {$t('account.noSessions')}
          </p>
        {:else}
          <ul class="sessions">
            {#each sessions as session}<li>
                <div>
                  <b>{session.current ? $t('account.thisDevice') : $t('account.otherDevice')}</b><small
                    >{$t('account.sessionDates', {
                      created: date(session.createdAt),
                      expires: date(session.expiresAt),
                    })}</small
                  >
                </div>
                {#if !session.current}<button class="ghost" disabled={!!pending} onclick={() => revoke(session.id)}
                    >{$t('settings.revoke')}</button
                  >{/if}
              </li>{/each}
          </ul>
        {/if}
      </section>
      <section class="panel section danger-zone">
        <h2>{$t('account.delete')}</h2>
        <p>{$t('account.deleteBody')}</p>
        <label
          >{$t('account.confirmPassword')}<input
            type="password"
            bind:value={deletePassword}
            autocomplete="current-password"
          /></label
        >
        <button class="danger" disabled={!!pending || !deletePassword} onclick={deleteAccount}
          >{pending === 'delete' ? $t('account.deleting') : $t('account.deleteForever')}</button
        >
      </section>
    {/if}
    <section class="panel notice">
      <h2>{$t('account.localIdentity')}</h2>
      <p class="muted">ID: {me.identityId}</p>
      <p>{$t('account.localIdentityBody')}</p>
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
  .section,
  .error-state {
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
  .error-state {
    text-align: center;
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
