<script lang="ts">
  let username = '';
  let password = '';
  let error = '';
  let submitting = false;
  async function submit() {
    if (submitting) return;
    submitting = true;
    error = '';
    try {
      const r = await fetch('/api/accounts/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: username.trim(), password }),
      });
      if (!r.ok) {
        let message = r.statusText || 'Login failed.';
        try {
          message = ((await r.json()) as { message?: string }).message || message;
        } catch {
          // Keep the HTTP fallback for non-JSON proxy errors.
        }
        throw new Error(message);
      }
      location.href = '/account';
    } catch (e) {
      error = e instanceof Error ? e.message : 'Login failed.';
    } finally {
      submitting = false;
    }
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
    >{#if error}<p class="error" role="alert">{error}</p>{/if}<button disabled={submitting}
      >{submitting ? 'Logging in…' : 'Log in'}</button
    ><a href="/register">Create an account</a>
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
