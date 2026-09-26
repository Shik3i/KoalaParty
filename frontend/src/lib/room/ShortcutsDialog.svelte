<script lang="ts">
  import { t } from '$lib/i18n';
  import Dialog from './Dialog.svelte';

  let { onClose }: { onClose: () => void } = $props();
</script>

<Dialog label={$t('shortcuts.title')} {onClose}>
  <h2>{$t('shortcuts.title')}</h2>
  <dl>
    <dt><kbd>Ctrl</kbd>+<kbd>V</kbd></dt>
    <dd>{$t('shortcuts.paste')}</dd>
    <dt><kbd>K</kbd> / <kbd>Space</kbd></dt>
    <dd>{$t('shortcuts.play')}</dd>
    <dt><kbd>←</kbd> <kbd>→</kbd></dt>
    <dd>{$t('shortcuts.seek')}</dd>
    <dt><kbd>/</kbd> · <kbd>A</kbd></dt>
    <dd>{$t('shortcuts.add')}</dd>
    <dt><kbd>C</kbd></dt>
    <dd>{$t('shortcuts.chat')}</dd>
    <dt><kbd>1</kbd>…<kbd>0</kbd></dt>
    <dd>{$t('shortcuts.react')}</dd>
    <dt><kbd>F</kbd> · <kbd>T</kbd> · <kbd>M</kbd></dt>
    <dd>{$t('shortcuts.views')}</dd>
    <dt><kbd>Shift</kbd>+<kbd>Enter</kbd></dt>
    <dd>{$t('shortcuts.playNow')}</dd>
  </dl>
  <div class="modal-actions"><button onclick={onClose}>{$t('common.gotIt')}</button></div>
</Dialog>

<style>
  dl {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: 0.55rem 1rem;
    margin: 0;
    font-size: 0.88rem;
  }
  dt {
    white-space: nowrap;
  }
  dd {
    margin: 0;
    color: var(--text-secondary);
  }
  kbd {
    display: inline-block;
    padding: 0.1rem 0.4rem;
    border: 1px solid var(--border-strong);
    border-bottom-width: 2px;
    border-radius: 5px;
    font: inherit;
    font-size: 0.78rem;
    font-weight: 700;
    background: var(--surface-elevated);
  }
</style>
