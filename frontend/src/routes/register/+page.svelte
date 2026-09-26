<script lang="ts">
  import { errorText, t } from '$lib/i18n';
  import { api } from '$lib/api';
  let username = '';
  let password = '';
  let error = '';
  let submitting = false;
  async function submit() {
    if (submitting) return;
    submitting = true;
    error = '';
    try {
      await api('/api/accounts/register', {
        method: 'POST',
        body: JSON.stringify({ username: username.trim(), password }),
      });
      location.href = '/account';
    } catch (e) {
      error = errorText(e);
    } finally {
      submitting = false;
    }
  }
</script>

<svelte:head><title>{$t('auth.createAccount')} · KoalaParty</title></svelte:head>
<main class="auth panel">
  <h1>{$t('auth.keepRooms')}</h1>
  <p>{$t('auth.keepRoomsBody')}</p>
  <form
    onsubmit={(e) => {
      e.preventDefault();
      submit();
    }}
  >
    <label
      >{$t('auth.username')}<input
        bind:value={username}
        minlength="3"
        maxlength="24"
        pattern="[A-Za-z0-9_]+"
        autocomplete="username"
        required
      /></label
    ><label
      >{$t('auth.password')}<input
        type="password"
        bind:value={password}
        minlength="10"
        maxlength="128"
        autocomplete="new-password"
        required
      /></label
    >{#if error}<p class="error" role="alert">{error}</p>{/if}<button disabled={submitting}
      >{submitting ? $t('home.creating') : $t('auth.createAccount')}</button
    >
  </form>
  <p class="muted">{$t('auth.noEmail')}</p>
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
