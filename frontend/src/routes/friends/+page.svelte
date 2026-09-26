<script lang="ts">
  import { errorText, t, type MessageKey } from '$lib/i18n';
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  type Friend = { username: string; status: string; direction: string };
  let list: Friend[] = [];
  let username = '';
  let error = '';
  let loading = true;
  let pending = '';
  async function load() {
    loading = true;
    try {
      error = '';
      list = await api('/api/friends');
    } catch (e) {
      error = errorText(e);
    } finally {
      loading = false;
    }
  }
  onMount(load);
  async function send() {
    if (pending) return;
    pending = 'send';
    error = '';
    try {
      await api('/api/friends', { method: 'POST', body: JSON.stringify({ username: username.trim() }) });
      username = '';
      await load();
    } catch (e) {
      error = errorText(e);
    } finally {
      pending = '';
    }
  }
  async function action(user: string, value: string) {
    if (pending) return;
    pending = `${user}:${value}`;
    error = '';
    try {
      await api(`/api/friends/${encodeURIComponent(user)}/${value}`, { method: 'POST' });
      await load();
    } catch (e) {
      error = errorText(e);
    } finally {
      pending = '';
    }
  }
</script>

<svelte:head><title>{$t('nav.friends')} · KoalaParty</title></svelte:head>
<main class="page">
  <h1>{$t('nav.friends')}</h1>
  <p>{$t('friends.body')}</p>
  <form
    class="panel send"
    onsubmit={(e) => {
      e.preventDefault();
      send();
    }}
  >
    <label
      >{$t('auth.username')}<input
        bind:value={username}
        minlength="3"
        maxlength="24"
        pattern="[A-Za-z0-9_]+"
        required
      /></label
    ><button disabled={!!pending}>{pending === 'send' ? $t('friends.sending') : $t('friends.send')}</button>
  </form>
  {#if error && list.length}<p class="error" role="alert">{error}</p>{/if}
  {#if error && !list.length}<section class="panel empty error-state" role="alert">
      <h2>{$t('friends.loadFailed')}</h2>
      <p class="error">{error}</p>
      <button onclick={load}>{$t('player.tryAgain')}</button>
    </section>{:else}<section class="panel list">
      {#if loading}<p class="muted" role="status">{$t('friends.loading')}</p>{:else if !list.length}<p class="muted">
          {$t('friends.empty')}
        </p>{/if}{#each list as friend}<article>
          <div>
            <b>{friend.username}</b><small
              >{$t(`friends.status.${friend.status}` as MessageKey)} · {$t(
                `friends.direction.${friend.direction}` as MessageKey,
              )}</small
            >
          </div>
          <div class="row">
            {#if friend.status === 'pending' && friend.direction === 'incoming'}<button
                disabled={!!pending}
                onclick={() => action(friend.username, 'accept')}>{$t('friends.accept')}</button
              ><button class="secondary" disabled={!!pending} onclick={() => action(friend.username, 'decline')}
                >{$t('friends.decline')}</button
              >{/if}<button class="ghost" disabled={!!pending} onclick={() => action(friend.username, 'remove')}
              >{$t('friends.remove')}</button
            ><button class="ghost" disabled={!!pending} onclick={() => action(friend.username, 'block')}
              >{$t('friends.block')}</button
            >
          </div>
        </article>{/each}
    </section>{/if}
</main>

<style>
  .page {
    max-width: 760px;
    margin: 4rem auto;
    padding: 0 1rem;
  }
  .send {
    padding: 1rem;
    display: grid;
    grid-template-columns: 1fr auto;
    align-items: end;
    gap: 1rem;
  }
  .list {
    padding: 1rem;
    margin-top: 1rem;
  }
  .empty {
    padding: 2rem;
    text-align: center;
  }
  .list article {
    display: flex;
    justify-content: space-between;
    gap: 1rem;
    padding: 1rem 0;
    border-bottom: 1px solid var(--border-subtle);
  }
  .list article:last-child {
    border: 0;
  }
  .list small {
    display: block;
    color: var(--text-muted);
    margin-top: 0.3rem;
  }
  @media (max-width: 600px) {
    .send {
      grid-template-columns: 1fr;
    }
    .list article {
      display: grid;
    }
  }
</style>
