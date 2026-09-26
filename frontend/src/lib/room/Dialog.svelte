<script lang="ts">
  import type { Snippet } from 'svelte';
  import { fade, scale } from 'svelte/transition';
  import { t } from '$lib/i18n';
  import { focusTrap } from './actions';

  // Modal shell shared by room dialogs: scrim, focus trap, Escape to close.
  let {
    label,
    role = 'dialog',
    wide = false,
    closeLabel = '',
    onClose,
    children,
  }: {
    label: string;
    role?: 'dialog' | 'alertdialog';
    wide?: boolean;
    closeLabel?: string;
    onClose: () => void;
    children: Snippet;
  } = $props();
</script>

<div class="modal-backdrop">
  <button
    class="modal-scrim"
    aria-label={closeLabel || $t('common.close')}
    onclick={onClose}
    transition:fade={{ duration: 160 }}
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
    {@render children()}
  </div>
</div>

<style>
  .wide {
    max-width: 32rem;
  }
</style>
