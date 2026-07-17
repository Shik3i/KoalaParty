<script lang="ts">
  let username = '';
  let password = '';
  let error = '';
  async function submit() {
    const r = await fetch('/api/accounts/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password }),
    });
    if (!r.ok) {
      error = ((await r.json()) as { message: string }).message;
      return;
    }
    location.href = '/account';
  }
</script>

<svelte:head><title>Log in · KoalaParty</title></svelte:head>
<main class="auth panel">
  <h1>Welcome back</h1>
  <form
    onsubmit={(e) => {
      e.preventDefault();
      submit();
    }}
  >
    <label>Username<input bind:value={username} autocomplete="username" required /></label><label
      >Password<input type="password" bind:value={password} autocomplete="current-password" required /></label
    >{#if error}<p class="error" role="alert">{error}</p>{/if}<button>Log in</button><a href="/register"
      >Create an account</a
    >
  </form>
  <p class="muted">Password recovery is not available in this release.</p>
</main>

<style>
  .auth {
    max-width: 430px;
    margin: 5rem auto;
    padding: 2rem;
  }
  .auth form {
    display: grid;
    gap: 1rem;
  }
</style>
