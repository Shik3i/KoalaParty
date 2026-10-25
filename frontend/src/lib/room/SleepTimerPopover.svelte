<script lang="ts">
  import { onMount } from 'svelte';
  import { Moon, X, Check } from 'phosphor-svelte';
  import { t } from '$lib/i18n';
  import { sleepTimer, setSleepTimer, type SleepTimerMode } from '$lib/sleepTimer';

  let {
    onTrigger,
    onClose,
  }: {
    onTrigger: () => void;
    onClose: () => void;
  } = $props();

  let popoverEl: HTMLDivElement | undefined = $state();

  const options: Array<{ mode: SleepTimerMode; label: string }> = [
    { mode: 'off', label: $t('player.sleepOff') },
    { mode: 'after-video', label: $t('player.sleepAfterVideo') },
    { mode: 15, label: $t('player.sleepMinutes', { minutes: 15 }) },
    { mode: 30, label: $t('player.sleepMinutes', { minutes: 30 }) },
    { mode: 45, label: $t('player.sleepMinutes', { minutes: 45 }) },
    { mode: 60, label: $t('player.sleepMinutes', { minutes: 60 }) },
  ];

  function choose(mode: SleepTimerMode) {
    setSleepTimer(mode, onTrigger);
    onClose();
  }

  onMount(() => {
    function handleClickOutside(e: MouseEvent) {
      if (popoverEl && !popoverEl.contains(e.target as Node)) {
        const trigger = (e.target as HTMLElement).closest('.sleep-anchor');
        if (!trigger) onClose();
      }
    }
    function handleKeydown(e: KeyboardEvent) {
      if (e.key === 'Escape') {
        e.preventDefault();
        onClose();
      }
    }
    document.addEventListener('click', handleClickOutside, true);
    document.addEventListener('keydown', handleKeydown);
    return () => {
      document.removeEventListener('click', handleClickOutside, true);
      document.removeEventListener('keydown', handleKeydown);
    };
  });
</script>

<div class="sleep-popover" bind:this={popoverEl} role="dialog" aria-label={$t('player.sleepTimer')}>
  <div class="head">
    <span class="title">
      <Moon size={16} weight="fill" />
      {$t('player.sleepTimer')}
    </span>
    <button class="ghost icon-button close-btn" onclick={onClose} aria-label={$t('common.close')}>
      <X size={15} weight="bold" />
    </button>
  </div>
  <p class="hint muted">{$t('player.sleepTimerHint')}</p>
  <div class="options">
    {#each options as opt}
      <button
        type="button"
        class="opt-btn"
        class:active={$sleepTimer.mode === opt.mode}
        onclick={() => choose(opt.mode)}
      >
        <span>{opt.label}</span>
        {#if $sleepTimer.mode === opt.mode}
          <Check size={14} weight="bold" />
        {/if}
      </button>
    {/each}
  </div>
</div>

<style>
  .sleep-popover {
    position: absolute;
    bottom: calc(100% + 0.55rem);
    right: 0;
    width: 230px;
    background: var(--surface-panel);
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-md);
    box-shadow: 0 14px 38px rgba(3, 12, 8, 0.35);
    backdrop-filter: blur(24px) saturate(140%);
    z-index: 50;
    padding: 0.85rem;
    display: flex;
    flex-direction: column;
    gap: 0.6rem;
  }
  .head {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .title {
    font-size: 0.88rem;
    font-weight: 750;
    color: var(--text-primary);
    display: flex;
    align-items: center;
    gap: 0.45rem;
  }
  .hint {
    margin: 0;
    font-size: 0.78rem;
    line-height: 1.35;
  }
  .close-btn {
    padding: 0.2rem;
    border-radius: 50%;
  }
  .options {
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
  }
  .opt-btn {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0.45rem 0.65rem;
    font-size: 0.8rem;
    font-weight: 650;
    background: var(--surface-elevated);
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-sm);
    color: var(--text-secondary);
    cursor: pointer;
    transition: all 0.15s ease;
  }
  .opt-btn:hover {
    color: var(--text-primary);
    background: var(--surface-hover);
  }
  .opt-btn.active {
    background: var(--accent-muted);
    border-color: var(--accent-primary);
    color: var(--accent-primary);
    font-weight: 750;
  }
  @media (max-width: 480px) {
    .sleep-popover {
      position: fixed;
      left: 1rem;
      right: 1rem;
      bottom: 5.5rem;
      width: auto;
    }
  }
</style>
