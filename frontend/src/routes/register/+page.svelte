<script lang="ts">
  import { api } from '$lib/api';
  let username = '';
  let password = '';
  let error = '';
  async function submit() {
    try {
      await api('/api/accounts/register', { method: 'POST', body: JSON.stringify({ username, password }) });
      location.href = '/account';
    } catch (e) {
      error = e instanceof Error ? e.message : 'Registration failed.';
    }
  }
</script>

<svelte:head><title>Create account · KoalaParty</title></svelte:head>
<main class="auth panel">
  <h1>Keep your rooms</h1>
  <p>Link this browser identity for cross-device access, private rooms, and friends.</p>
  <form
    onsubmit={(e) => {
      e.preventDefault();
      submit();
    }}
  >
    <label
      >Username<input
        bind:value={username}
        minlength="3"
        maxlength="24"
        pattern="[A-Za-z0-9_]+"
        autocomplete="username"
        required
      /></label
    ><label
      >Password<input
        type="password"
        bind:value={password}
        minlength="10"
        maxlength="128"
        autocomplete="new-password"
        required
      /></label
    >{#if error}<p class="error" role="alert">{error}</p>{/if}<button>Create account</button>
  </form>
  <p class="muted">No email is requested. Password recovery is unavailable.</p>
</main>

<style>
  .auth {
    max-width: 460px;
    margin: 5rem auto;
    padding: 2rem;
  }
  .auth form {
    display: grid;
    gap: 1rem;
  }
</style>
