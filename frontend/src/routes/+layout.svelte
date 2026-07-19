<script lang="ts">
  import '../lib/styles/tokens.css';
  import '../lib/styles/themes/light.css';
  import '../lib/styles/themes/dark.css';
  import '../lib/styles/base.css';
  import { onMount } from 'svelte';
  import { Compass, FilmSlate, UsersThree, UserCircle, ShieldStar, Sun, Moon, Monitor } from 'phosphor-svelte';
  import { applyTheme, initialTheme, type Theme } from '$lib/theme';
  import { establish, type Principal } from '$lib/api';
  let { children } = $props();
  let theme: Theme = $state('system');
  let principal: Principal | null = $state(null);
  let version = $state('');
  onMount(async () => {
    theme = initialTheme();
    applyTheme(theme);
    try {
      principal = await establish();
    } catch {}
    try {
      const info = (await fetch('/api/version').then((r) => r.json())) as { version?: string };
      version = info.version ?? '';
    } catch {}
  });
  function setTheme(next: Theme) {
    theme = next;
    applyTheme(next);
  }
  const themeOptions: { value: Theme; label: string }[] = [
    { value: 'system', label: 'System theme' },
    { value: 'light', label: 'Light theme' },
    { value: 'dark', label: 'Dark theme' },
  ];
</script>

<svelte:head>
  <meta name="theme-color" media="(prefers-color-scheme: light)" content="#f2f3e9" />
  <meta name="theme-color" media="(prefers-color-scheme: dark)" content="#0d1b15" />
  <meta property="og:type" content="website" />
  <meta property="og:site_name" content="KoalaParty" />
  <meta property="og:title" content="KoalaParty — Watch YouTube together privately" />
  <meta
    property="og:description"
    content="Synchronized YouTube watch parties with a shared queue — no accounts, ads, analytics, or tracking."
  />
  <meta name="twitter:card" content="summary" />
  <meta name="twitter:title" content="KoalaParty — Watch YouTube together privately" />
  <meta
    name="twitter:description"
    content="Synchronized YouTube watch parties with a shared queue — no accounts, ads, analytics, or tracking."
  />
</svelte:head>
<a class="skip" href="#main">Skip to content</a>
<header class="site-header">
  <a class="brand" href="/"><span aria-hidden="true">🐨</span> KoalaParty</a>
  <nav aria-label="Main navigation">
    <a href="/discover"><Compass size={17} weight="bold" />Discover</a><a href="/rooms"
      ><FilmSlate size={17} weight="bold" />My rooms</a
    ><a href="/friends"><UsersThree size={17} weight="bold" />Friends</a>{#if principal?.isAdmin}<a href="/admin"
        ><ShieldStar size={17} weight="bold" />Admin</a
      >{/if}<a href="/account"><UserCircle size={17} weight="bold" />Account</a>
  </nav>
  <div class="theme" role="group" aria-label="Theme">
    {#each themeOptions as option}<button
        type="button"
        class:active={theme === option.value}
        aria-pressed={theme === option.value}
        aria-label={option.label}
        title={option.label}
        onclick={() => setTheme(option.value)}
        >{#if option.value === 'system'}<Monitor size={16} weight="bold" />{:else if option.value === 'light'}<Sun
            size={16}
            weight="bold"
          />{:else}<Moon size={16} weight="bold" />{/if}</button
      >{/each}
  </div>
</header>
<div id="main">
  <svelte:boundary onerror={(error) => console.error('App error boundary:', error)}>
    {@render children()}
    {#snippet failed(error, reset)}
      <main class="boundary-error">
        <span aria-hidden="true">🐨</span>
        <h1>Something hiccuped</h1>
        <p>An unexpected error interrupted the page. Your room is safe — try again.</p>
        <div class="boundary-actions">
          <button onclick={reset}>Try again</button><a class="button secondary" href="/">Back home</a>
        </div>
        <pre>{error}</pre>
      </main>
    {/snippet}
  </svelte:boundary>
</div>
<footer>
  <span>KoalaParty · MIT licensed · No tracking. No ads.</span><span
    ><a href="/privacy">Privacy</a> · <a href="/imprint">Imprint</a> ·
    <a href="https://github.com/Shik3i/KoalaParty" target="_blank" rel="noopener noreferrer">GitHub</a> ·
    <a href="https://sync.koalastuff.net/" target="_blank" rel="noopener noreferrer">KoalaSync</a>{#if version}
      ·
      {#if /^\d+\.\d+\.\d+$/.test(version)}<a
          href="https://github.com/Shik3i/KoalaParty/releases/tag/v{version}"
          target="_blank"
          rel="noopener noreferrer">v{version}</a
        >{:else}<span class="version">{version}</span>{/if}{/if}</span
  >
</footer>

<style>
  .skip {
    position: fixed;
    top: -5rem;
    left: 1rem;
    z-index: 20;
    background: var(--surface-elevated);
    padding: 0.7rem;
  }
  .boundary-error {
    max-width: 560px;
    margin: 5rem auto;
    padding: 2.5rem clamp(1rem, 4vw, 2.5rem);
    text-align: center;
  }
  .boundary-error span {
    font-size: 3rem;
  }
  .boundary-error .boundary-actions {
    display: flex;
    gap: 0.7rem;
    justify-content: center;
    margin: 1.5rem 0;
    flex-wrap: wrap;
  }
  .boundary-error pre {
    text-align: left;
    overflow: auto;
    max-height: 8rem;
    padding: 0.8rem;
    background: var(--surface-panel);
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-sm);
    color: var(--text-muted);
    font-size: 0.75rem;
    white-space: pre-wrap;
  }
  .skip:focus {
    top: 1rem;
  }
  .site-header {
    height: 68px;
    display: flex;
    align-items: center;
    padding: 0 clamp(1rem, 4vw, 3rem);
    gap: 2rem;
    border-bottom: 1px solid var(--border-subtle);
    background: color-mix(in srgb, var(--surface-panel) 92%, transparent);
    position: sticky;
    top: 0;
    z-index: 10;
  }
  .brand {
    font-size: 1.08rem;
    font-weight: 850;
    text-decoration: none;
    color: var(--text-primary);
    white-space: nowrap;
  }
  .site-header nav {
    display: flex;
    gap: 1.25rem;
    margin-left: auto;
  }
  .site-header nav a {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    color: var(--text-secondary);
    text-decoration: none;
    font-weight: 650;
    font-size: 0.9rem;
    white-space: nowrap;
    transition: color 0.15s ease;
  }
  .site-header nav a :global(svg) {
    flex: 0 0 auto;
  }
  .site-header nav a:hover {
    color: var(--accent-primary);
  }
  .theme {
    display: inline-flex;
    gap: 2px;
    padding: 3px;
    border: 1px solid var(--border-subtle);
    border-radius: 999px;
    background: var(--surface-panel);
  }
  .theme button {
    padding: 0.34rem 0.5rem;
    background: transparent;
    color: var(--text-muted);
    border-radius: 999px;
    transition:
      background 0.15s ease,
      color 0.15s ease;
  }
  .theme button:hover {
    color: var(--text-primary);
    transform: none;
  }
  .theme button.active {
    background: var(--accent-muted);
    color: var(--text-primary);
  }
  footer {
    display: flex;
    justify-content: space-between;
    gap: 1rem;
    padding: 2rem clamp(1rem, 4vw, 3rem);
    color: var(--text-muted);
    font-size: 0.86rem;
  }
  @media (max-width: 700px) {
    .site-header {
      height: auto;
      min-height: 68px;
      flex-wrap: wrap;
      gap: 0.8rem;
      padding-top: 0.65rem;
      padding-bottom: 0.65rem;
    }
    .site-header nav {
      order: 3;
      width: 100%;
      margin-left: 0;
      gap: 1rem;
      overflow-x: auto;
      padding-bottom: 0.15rem;
      scrollbar-width: thin;
      min-width: 0;
    }
    .theme {
      margin-left: auto;
    }
    footer {
      flex-direction: column;
    }
  }
</style>
