<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  type Friend = { username: string; status: string; direction: string };
  let list: Friend[] = [];
  let username = '';
  let error = '';
  let loading = true;
  let pending = '';
  async function load() {
    try {
      error = '';
      list = await api('/api/friends');
    } catch (e) {
      error = e instanceof Error ? e.message : 'Friends unavailable.';
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
      error = e instanceof Error ? e.message : 'Request failed.';
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
      error = e instanceof Error ? e.message : 'Request failed.';
    } finally {
      pending = '';
    }
  }
</script>

<svelte:head><title>Friends · KoalaParty</title></svelte:head>
<main class="page">
  <h1>Friends</h1>
  <p>Accepted friends can join your friends-only rooms.</p>
  <form
    class="panel send"
    onsubmit={(e) => {
      e.preventDefault();
      send();
    }}
  >
    <label>Username<input bind:value={username} minlength="3" maxlength="24" pattern="[A-Za-z0-9_]+" required /></label
    ><button disabled={!!pending}>{pending === 'send' ? 'Sending…' : 'Send request'}</button>
  </form>
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  <section class="panel list">
    {#if loading}<p class="muted" role="status">Loading friends…</p>{:else if !list.length}<p class="muted">
        No friend relationships yet.
      </p>{/if}{#each list as friend}<article>
        <div><b>{friend.username}</b><small>{friend.status} · {friend.direction}</small></div>
        <div class="row">
          {#if friend.status === 'pending' && friend.direction === 'incoming'}<button
              disabled={!!pending}
              onclick={() => action(friend.username, 'accept')}>Accept</button
            ><button class="secondary" disabled={!!pending} onclick={() => action(friend.username, 'decline')}
              >Decline</button
            >{/if}<button class="ghost" disabled={!!pending} onclick={() => action(friend.username, 'remove')}
            >Remove</button
          ><button class="ghost" disabled={!!pending} onclick={() => action(friend.username, 'block')}>Block</button>
        </div>
      </article>{/each}
  </section>
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
