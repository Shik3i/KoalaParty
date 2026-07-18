<script lang="ts">
  import '../lib/styles/tokens.css';
  import '../lib/styles/themes/light.css';
  import '../lib/styles/themes/dark.css';
  import '../lib/styles/base.css';
  import { onMount } from 'svelte';
  import { Compass, FilmSlate, UsersThree, UserCircle, ShieldStar } from 'phosphor-svelte';
  import { applyTheme, initialTheme, type Theme } from '$lib/theme';
  import { establish, type Principal } from '$lib/api';
  let { children } = $props();
  let theme: Theme = $state('system');
  let principal: Principal | null = $state(null);
  onMount(async () => {
    theme = initialTheme();
    applyTheme(theme);
    try {
      principal = await establish();
    } catch {}
  });
  function change() {
    applyTheme(theme);
  }
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
  <label class="theme"
    ><span class="sr-only">Theme</span><select bind:value={theme} onchange={change}
      ><option value="system">System theme</option><option value="light">Light theme</option><option value="dark"
        >Dark theme</option
      ></select
    ></label
  >
</header>
<div id="main">{@render children()}</div>
<footer>
  <span>KoalaParty · MIT licensed · No tracking. No ads.</span><span
    ><a href="/privacy">Privacy</a> · <a href="/imprint">Imprint</a> ·
    <a href="https://github.com/Shik3i/KoalaParty" target="_blank" rel="noopener noreferrer">GitHub</a> ·
    <a href="https://sync.koalastuff.net/" target="_blank" rel="noopener noreferrer">KoalaSync</a></span
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
    transition: color 0.15s ease;
  }
  .site-header nav a:hover {
    color: var(--accent-primary);
  }
  .theme {
    width: 9.2rem;
  }
  .theme select {
    padding: 0.5rem;
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
      width: 7.5rem;
      min-width: 0;
    }
    footer {
      flex-direction: column;
    }
  }
</style>
