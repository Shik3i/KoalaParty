<script lang="ts">
  import { onDestroy } from 'svelte';
  import { fade } from 'svelte/transition';
  import { renderSVG } from 'uqr';
  import { Copy, Check, ShareNetwork, Link } from 'phosphor-svelte';
  import { t } from '$lib/i18n';
  import type { Snapshot } from '$lib/room';
  import Dialog from './Dialog.svelte';

  let {
    room,
    manager,
    onSetSlug,
    onNotice,
    onClose,
  }: {
    room: Snapshot;
    manager: boolean;
    onSetSlug: (slug: string) => Promise<boolean>;
    onNotice: (message: string, kind: 'success' | 'error') => void;
    onClose: () => void;
  } = $props();

  // Draft starts from the saved link when the dialog opens.
  // svelte-ignore state_referenced_locally
  let slugDraft = $state(room.slug ?? '');
  let saving = $state(false);
  let copied = $state(false);
  let slugSaved = $state(false);
  let copyTimeout: ReturnType<typeof setTimeout> | undefined;
  let slugTimeout: ReturnType<typeof setTimeout> | undefined;

  const link = $derived(`${location.origin}${room.slug ? `/r/${room.slug}` : `/room/${room.id}`}`);
  // Generated locally: the invite link never leaves the browser for a QR service.
  const qr = $derived(renderSVG(link, { border: 1, ecc: 'M' }));
  const canShare = typeof navigator !== 'undefined' && typeof navigator.share === 'function';
  const normalized = $derived(
    slugDraft
      .toLowerCase()
      .normalize('NFKD')
      .replace(/[̀-ͯ]/g, '')
      .replace(/[^a-z0-9]+/g, '-')
      .replace(/^-+|-+$/g, '')
      .slice(0, 32),
  );

  onDestroy(() => {
    if (copyTimeout) clearTimeout(copyTimeout);
    if (slugTimeout) clearTimeout(slugTimeout);
  });

  async function copy() {
    if (copied) return;
    try {
      await navigator.clipboard.writeText(link);
      copied = true;
      if (copyTimeout) clearTimeout(copyTimeout);
      copyTimeout = setTimeout(() => {
        copied = false;
      }, 2000);
      onNotice($t('invite.copied'), 'success');
    } catch {
      onNotice($t('invite.copyFailed'), 'error');
    }
  }

  async function share() {
    try {
      await navigator.share({
        title: $t('invite.shareTitle', { room: room.label }),
        text: $t('invite.shareText'),
        url: link,
      });
    } catch {
      /* dismissed */
    }
  }

  async function saveSlug(event: SubmitEvent) {
    event.preventDefault();
    saving = true;
    if (await onSetSlug(normalized)) {
      slugDraft = normalized;
      slugSaved = true;
      if (slugTimeout) clearTimeout(slugTimeout);
      slugTimeout = setTimeout(() => {
        slugSaved = false;
      }, 2500);
    }
    saving = false;
  }
</script>

<Dialog label={$t('invite.title')} wide {onClose}>
  <div class="dialog-header">
    <h2>{$t('invite.title')}</h2>
    <p class="muted">{$t('invite.body')}</p>
  </div>

  <div class="link-row">
    <input
      readonly
      class:copied
      value={link}
      aria-label={$t('invite.link')}
      onfocus={(e) => e.currentTarget.select()}
      onclick={(e) => e.currentTarget.select()}
    />
    <button type="button" class="copy-btn" class:copied disabled={copied} onclick={copy} aria-live="polite">
      {#if copied}
        <Check size={16} weight="bold" />
        <span>{$t('invite.copiedButton')}</span>
      {:else}
        <Copy size={16} weight="bold" />
        <span>{$t('invite.copy')}</span>
      {/if}
    </button>
  </div>

  <div class="qr-row">
    <!-- eslint-disable-next-line svelte/no-at-html-tags -- SVG generated locally from the invite URL -->
    <div class="qr" role="img" aria-label={$t('invite.qr')}>{@html qr}</div>
    <div class="qr-info">
      <p class="qr-hint">{$t('invite.qrHint')}</p>
    </div>
  </div>

  {#if manager}
    <form class="slug-form" onsubmit={saveSlug}>
      <div class="slug-heading">
        <label for="room-slug"><Link size={15} weight="bold" />{$t('invite.shortLink')}</label>
        {#if slugSaved}
          <span class="slug-saved-badge" transition:fade={{ duration: 150 }}>
            <Check size={13} weight="bold" />
            {$t('invite.slugSaved')}
          </span>
        {/if}
      </div>
      <div class="slug-input">
        <span class="slug-prefix">{location.host}/r/</span>
        <input
          id="room-slug"
          bind:value={slugDraft}
          maxlength="40"
          placeholder={$t('invite.slugPlaceholder')}
          autocomplete="off"
        />
      </div>
      <small class="muted">{$t('invite.slugHint')}</small>
      <div class="slug-actions">
        {#if room.slug}
          <button type="button" class="ghost" disabled={saving} onclick={() => onSetSlug('')}>
            {$t('invite.removeSlug')}
          </button>
        {/if}
        <button class="secondary" disabled={saving || normalized.length < 3 || normalized === room.slug}>
          {#if saving}
            {$t('invite.slugSaving')}
          {:else}
            {$t('invite.saveSlug')}
          {/if}
        </button>
      </div>
    </form>
  {/if}

  <div class="modal-actions">
    {#if canShare}
      <button type="button" class="secondary" onclick={share}>
        <ShareNetwork size={16} weight="bold" />
        {$t('invite.share')}
      </button>
    {/if}
    <button type="button" onclick={onClose}>{$t('common.done')}</button>
  </div>
</Dialog>

<style>
  .dialog-header {
    display: grid;
    gap: 0.35rem;
  }
  .dialog-header h2 {
    margin: 0;
    font-size: 1.15rem;
    font-weight: 750;
    letter-spacing: -0.015em;
  }
  .dialog-header p {
    margin: 0;
    font-size: 0.9rem;
    line-height: 1.45;
  }
  .link-row {
    display: flex;
    gap: 0.5rem;
    align-items: stretch;
  }
  .link-row input {
    flex: 1;
    min-width: 0;
    font-size: 0.86rem;
    font-family: inherit;
    text-overflow: ellipsis;
    transition:
      border-color 0.18s ease,
      box-shadow 0.18s ease;
  }
  .link-row input.copied {
    border-color: color-mix(in srgb, var(--success) 65%, var(--border-subtle));
    box-shadow: 0 0 0 1px color-mix(in srgb, var(--success) 35%, transparent);
  }
  .copy-btn {
    flex: 0 0 auto;
    min-width: 6.8rem;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 0.45rem;
    font-size: 0.88rem;
    white-space: nowrap;
    transition:
      background-color 0.2s cubic-bezier(0.2, 0.8, 0.2, 1),
      border-color 0.2s cubic-bezier(0.2, 0.8, 0.2, 1),
      color 0.2s cubic-bezier(0.2, 0.8, 0.2, 1),
      box-shadow 0.2s ease,
      transform 0.15s ease;
  }
  .copy-btn:not(.copied):active {
    transform: scale(0.97);
  }
  .copy-btn.copied {
    background: var(--success);
    border-color: var(--success);
    color: var(--surface-page);
    opacity: 1 !important;
    cursor: default;
    transform: none !important;
    box-shadow: 0 4px 14px color-mix(in srgb, var(--success) 35%, transparent);
  }
  .qr-row {
    display: flex;
    align-items: center;
    gap: 1.1rem;
    padding: 0.85rem;
    border-radius: var(--radius-sm);
    background: color-mix(in srgb, var(--surface-panel) 60%, transparent);
    border: 1px solid var(--border-subtle);
  }
  .qr {
    width: 118px;
    height: 118px;
    flex: 0 0 auto;
    padding: 8px;
    border-radius: 10px;
    background: #ffffff;
    box-shadow: 0 4px 14px rgba(0, 0, 0, 0.25);
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .qr :global(svg) {
    display: block;
    width: 100%;
    height: 100%;
  }
  .qr :global(svg rect),
  .qr :global(svg path) {
    shape-rendering: crispEdges;
  }
  .qr-info {
    flex: 1;
    min-width: 0;
  }
  .qr-hint {
    margin: 0;
    color: var(--text-secondary);
    font-size: 0.88rem;
    line-height: 1.45;
  }
  .slug-form {
    display: grid;
    gap: 0.55rem;
    padding: 0.85rem;
    border-radius: var(--radius-sm);
    background: color-mix(in srgb, var(--surface-panel) 60%, transparent);
    border: 1px solid var(--border-subtle);
  }
  .slug-heading {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
  }
  .slug-heading label {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    font-weight: 700;
    margin: 0;
  }
  .slug-saved-badge {
    display: inline-flex;
    align-items: center;
    gap: 0.3rem;
    font-size: 0.78rem;
    font-weight: 650;
    color: var(--success);
  }
  .slug-input {
    display: flex;
    align-items: center;
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    background: var(--surface-elevated);
    overflow: hidden;
    transition:
      border-color 0.18s ease,
      box-shadow 0.18s ease;
  }
  .slug-input:focus-within {
    border-color: var(--accent-primary);
    box-shadow: var(--focus-ring);
  }
  .slug-prefix {
    padding-left: 0.75rem;
    color: var(--text-muted);
    font-size: 0.85rem;
    white-space: nowrap;
    user-select: none;
  }
  .slug-input input {
    border: 0;
    background: transparent;
    padding-left: 0.2rem;
    box-shadow: none;
  }
  .slug-input input:focus {
    box-shadow: none;
  }
  .slug-actions {
    display: flex;
    justify-content: flex-end;
    align-items: center;
    gap: 0.5rem;
    padding-top: 0.2rem;
  }
  @media (max-width: 480px) {
    .qr-row {
      flex-direction: column;
      text-align: center;
      gap: 0.75rem;
    }
  }
</style>
