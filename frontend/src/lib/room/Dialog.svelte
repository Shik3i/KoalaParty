<script lang="ts">
  import type { Snippet } from 'svelte';
  import { fade, scale } from 'svelte/transition';
  import { X } from 'phosphor-svelte';
  import { t } from '$lib/i18n';
  import { focusTrap } from './actions';

  // Modal shell shared by room dialogs: scrim, focus trap, Escape to close.
  let {
    label,
    role = 'dialog',
    wide = false,
    closeLabel = '',
    showCloseButton = true,
    onClose,
    children,
  }: {
    label: string;
    role?: 'dialog' | 'alertdialog';
    wide?: boolean;
    closeLabel?: string;
    showCloseButton?: boolean;
    onClose: () => void;
    children: Snippet;
  } = $props();
</script>

<div class="modal-backdrop">
  <button class="modal-scrim" tabindex="-1" aria-hidden="true" onclick={onClose} transition:fade={{ duration: 160 }}
  ></button>
  <div
    class="modal panel"
    class:wide
    {role}
    aria-modal="true"
    aria-label={label}
    use:focusTrap={onClose}
    transition:scale={{ start: 0.94, duration: 180 }}
  >
    {#if showCloseButton && role !== 'alertdialog'}
      <button
        type="button"
        class="ghost modal-close-btn"
        aria-label={closeLabel || $t('common.close')}
        onclick={onClose}
      >
        <X size={18} weight="bold" />
      </button>
    {/if}
    {@render children()}
  </div>
</div>

<style>
  .wide {
    max-width: 32rem;
  }
  .modal-close-btn {
    position: absolute;
    top: 1rem;
    right: 1rem;
    width: 2.1rem;
    height: 2.1rem;
    padding: 0;
    border-radius: var(--radius-sm);
    color: var(--text-muted);
    display: inline-flex;
    align-items: center;
    justify-content: center;
    z-index: 2;
    transition:
      color 0.15s ease,
      background-color 0.15s ease,
      transform 0.15s ease;
  }
  .modal-close-btn:hover {
    color: var(--text-primary);
    background: var(--surface-hover);
    transform: none;
  }
  .modal-close-btn:active {
    transform: scale(0.94);
  }
</style>
