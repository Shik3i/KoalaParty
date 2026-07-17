<script lang="ts">
  import { onMount } from 'svelte';
  import { establish, type Principal } from '$lib/api';
  let me: Principal | null = null;
  let error = '';
  let loading = true;
  onMount(async () => {
    try {
      me = await establish();
    } catch (e) {
      error = e instanceof Error ? e.message : 'Identity unavailable.';
    } finally {
      loading = false;
    }
  });
</script>

<svelte:head><title>Account · KoalaParty</title></svelte:head>
<main class="page">
  <h1>Account</h1>
  {#if loading}<p class="muted" role="status">Loading identity…</p>{:else if error}<p class="error" role="alert">
      {error}
    </p>{:else if me}<section class="panel card">
      <div class="avatar">{me.displayName.slice(0, 1).toUpperCase()}</div>
      <div>
        <h2>{me.displayName}</h2>
        <p class="muted">{me.accountId ? 'Linked account' : 'Persistent anonymous identity'}</p>
      </div>
    </section>
    {#if !me.accountId}<section class="panel notice">
        <h2>Protect your rooms</h2>
        <p>
          This identity belongs only to this browser. Create an account before clearing storage to preserve ownership.
        </p>
        <a class="button" href="/register">Create account</a><a class="button secondary" href="/login">Log in</a>
      </section>{/if}
    <section class="panel notice">
      <h2>Local identity</h2>
      <p class="muted">ID: {me.identityId}</p>
      <p>There is no anonymous recovery key and no browser fingerprinting.</p>
    </section>{/if}
</main>

<style>
  .page {
    max-width: 720px;
    margin: 4rem auto;
    padding: 0 1rem;
  }
  .card,
  .notice {
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
  .card h2 {
    margin: 0;
  }
  .notice .button {
    margin-right: 0.5rem;
  }
</style>
