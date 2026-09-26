<script lang="ts">
  import { renderSVG } from 'uqr';
  import { Copy, ShareNetwork, Link } from 'phosphor-svelte';
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

  async function copy() {
    try {
      await navigator.clipboard.writeText(link);
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
    if (await onSetSlug(normalized)) slugDraft = normalized;
    saving = false;
  }
</script>

<Dialog label={$t('invite.title')} wide {onClose}>
  <h2>{$t('invite.title')}</h2>
  <p class="muted">{$t('invite.body')}</p>
  <div class="link-row">
    <input readonly value={link} aria-label={$t('invite.link')} onfocus={(e) => e.currentTarget.select()} />
    <button onclick={copy}><Copy size={16} weight="bold" />{$t('invite.copy')}</button>
  </div>
  <div class="qr-row">
    <!-- eslint-disable-next-line svelte/no-at-html-tags -- SVG generated locally from the invite URL -->
    <div class="qr" role="img" aria-label={$t('invite.qr')}>{@html qr}</div>
    <p class="muted">{$t('invite.qrHint')}</p>
  </div>
  {#if manager}<form class="slug-form" onsubmit={saveSlug}>
      <label for="room-slug"><Link size={15} weight="bold" />{$t('invite.shortLink')}</label>
      <div class="slug-input">
        <span>{location.host}/r/</span><input
          id="room-slug"
          bind:value={slugDraft}
          maxlength="40"
          placeholder={$t('invite.slugPlaceholder')}
          autocomplete="off"
        />
      </div>
      <small class="muted">{$t('invite.slugHint')}</small>
      <div class="modal-actions">
        {#if room.slug}<button type="button" class="ghost" disabled={saving} onclick={() => onSetSlug('')}
            >{$t('invite.removeSlug')}</button
          >{/if}
        <button class="secondary" disabled={saving || normalized.length < 3 || normalized === room.slug}
          >{$t('invite.saveSlug')}</button
        >
      </div>
    </form>{/if}
  <div class="modal-actions">
    {#if canShare}<button class="secondary" onclick={share}
        ><ShareNetwork size={16} weight="bold" />{$t('invite.share')}</button
      >{/if}
    <button onclick={onClose}>{$t('common.done')}</button>
  </div>
</Dialog>

<style>
  .link-row {
    display: flex;
    gap: 0.5rem;
  }
  .link-row input {
    flex: 1;
    min-width: 0;
    font-size: 0.85rem;
  }
  .qr-row {
    display: flex;
    align-items: center;
    gap: 1rem;
  }
  .qr {
    width: 132px;
    flex: 0 0 auto;
    padding: 6px;
    border-radius: 10px;
    background: white;
  }
  .qr :global(svg) {
    display: block;
    width: 100%;
    height: auto;
  }
  .qr :global(svg rect),
  .qr :global(svg path) {
    shape-rendering: crispEdges;
  }
  .slug-form {
    display: grid;
    gap: 0.4rem;
    padding-top: 0.9rem;
    border-top: 1px solid var(--border-subtle);
  }
  .slug-form label {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    font-weight: 700;
  }
  .slug-input {
    display: flex;
    align-items: center;
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    background: var(--surface-elevated);
    overflow: hidden;
  }
  .slug-input span {
    padding-left: 0.7rem;
    color: var(--text-muted);
    font-size: 0.85rem;
    white-space: nowrap;
  }
  .slug-input input {
    border: 0;
    background: transparent;
    padding-left: 0.1rem;
  }
  @media (max-width: 480px) {
    .qr-row {
      flex-direction: column;
      text-align: center;
    }
  }
</style>
