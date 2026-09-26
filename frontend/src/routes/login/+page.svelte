<script lang="ts">
  import { errorText, t } from '$lib/i18n';
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
        let message = r.statusText || $t('auth.loginFailed');
        try {
          message = ((await r.json()) as { message?: string }).message || message;
        } catch {
          // Keep the HTTP fallback for non-JSON proxy errors.
        }
        throw new Error(message);
      }
      location.href = '/account';
    } catch (e) {
      error = errorText(e);
    } finally {
      submitting = false;
    }
  }
</script>

<svelte:head><title>{$t('auth.login')} · KoalaParty</title></svelte:head>
<main class="auth panel">
  <h1>{$t('auth.welcomeBack')}</h1>
  <form
    onsubmit={(e) => {
      e.preventDefault();
      submit();
    }}
  >
    <label>{$t('auth.username')}<input bind:value={username} autocomplete="username" required /></label><label
      >{$t('auth.password')}<input
        type="password"
        bind:value={password}
        autocomplete="current-password"
        required
      /></label
    >{#if error}<p class="error" role="alert">{error}</p>{/if}<button disabled={submitting}
      >{submitting ? $t('auth.loggingIn') : $t('auth.login')}</button
    ><a href="/register">{$t('auth.createInstead')}</a>
  </form>
  <p class="muted">{$t('auth.noRecovery')}</p>
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
