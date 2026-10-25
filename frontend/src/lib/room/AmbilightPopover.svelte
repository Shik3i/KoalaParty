<script lang="ts">
  import { onMount } from 'svelte';
  import { Sparkle, X, ArrowCounterClockwise } from 'phosphor-svelte';
  import { t } from '$lib/i18n';
  import {
    ambilight,
    updateAmbilight,
    resetAmbilight,
    AMBILIGHT_MODES,
    AMBILIGHT_SWATCHES,
    type AmbilightMode,
  } from '$lib/ambilight';

  let { onClose }: { onClose: () => void } = $props();

  function selectMode(mode: AmbilightMode) {
    updateAmbilight({ mode });
  }

  function selectColor(color: string) {
    updateAmbilight({ mode: 'static', color });
  }
  let popoverEl: HTMLDivElement | undefined = $state();

  onMount(() => {
    function handleClickOutside(e: MouseEvent) {
      if (popoverEl && !popoverEl.contains(e.target as Node)) {
        const trigger = (e.target as HTMLElement).closest('.ambilight-anchor');
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

<div class="ambilight-popover" bind:this={popoverEl} role="dialog" aria-label={$t('ambilight.title')}>
  <div class="head">
    <span class="title">
      <Sparkle size={17} weight="duotone" />
      {$t('ambilight.title')}
    </span>
    <button class="ghost icon-button close-btn" onclick={onClose} aria-label={$t('common.close')}>
      <X size={15} weight="bold" />
    </button>
  </div>

  <div class="body">
    <!-- Mode Selection -->
    <div class="section">
      <span class="label">{$t('ambilight.mode')}</span>
      <div class="modes-grid">
        {#each AMBILIGHT_MODES as mode (mode.value)}
          <button
            type="button"
            class="mode-btn"
            class:active={$ambilight.mode === mode.value}
            onclick={() => selectMode(mode.value)}
          >
            {$t(mode.labelKey as any)}
          </button>
        {/each}
      </div>
    </div>

    <!-- Color picker (only active/relevant in static mode, or used to choose color) -->
    {#if $ambilight.mode === 'static'}
      <div class="section">
        <span class="label">{$t('ambilight.color')}</span>
        <div class="swatches">
          {#each AMBILIGHT_SWATCHES as swatch (swatch.value)}
            <button
              type="button"
              class="swatch-btn"
              class:selected={$ambilight.color.toLowerCase() === swatch.value.toLowerCase()}
              style="--swatch: {swatch.value};"
              title={swatch.label}
              onclick={() => selectColor(swatch.value)}
              aria-label={swatch.label}
            ></button>
          {/each}
          <label class="custom-color-picker" title={$t('ambilight.color')}>
            <input
              type="color"
              value={$ambilight.color}
              oninput={(e) => updateAmbilight({ color: e.currentTarget.value })}
            />
            <span class="picker-preview" style="--swatch: {$ambilight.color}"></span>
          </label>
        </div>
      </div>
    {/if}

    {#if $ambilight.mode !== 'off'}
      <!-- Brightness / Intensity -->
      <div class="section">
        <div class="slider-head">
          <span class="label">{$t('ambilight.intensity')}</span>
          <span class="val">{Math.round($ambilight.intensity * 100)}%</span>
        </div>
        <input
          class="slider"
          type="range"
          min="0.2"
          max="1.0"
          step="0.05"
          value={$ambilight.intensity}
          oninput={(e) => updateAmbilight({ intensity: Number(e.currentTarget.value) })}
          aria-label={$t('ambilight.intensity')}
        />
      </div>

      <!-- Blur / Spread -->
      <div class="section">
        <div class="slider-head">
          <span class="label">{$t('ambilight.blur')}</span>
          <span class="val">{$ambilight.blur}px</span>
        </div>
        <input
          class="slider"
          type="range"
          min="24"
          max="80"
          step="4"
          value={$ambilight.blur}
          oninput={(e) => updateAmbilight({ blur: Number(e.currentTarget.value) })}
          aria-label={$t('ambilight.blur')}
        />
      </div>
    {/if}
  </div>

  <div class="foot">
    <button class="ghost reset-btn" onclick={resetAmbilight}>
      <ArrowCounterClockwise size={14} weight="bold" />
      {$t('ambilight.reset')}
    </button>
  </div>
</div>

<style>
  .ambilight-popover {
    position: absolute;
    bottom: calc(100% + 0.55rem);
    right: 0;
    width: 290px;
    background: var(--surface-panel);
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-md);
    box-shadow: 0 14px 38px rgba(3, 12, 8, 0.35);
    backdrop-filter: blur(24px) saturate(140%);
    z-index: 50;
    padding: 0.85rem;
    display: flex;
    flex-direction: column;
    gap: 0.85rem;
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

  .close-btn {
    padding: 0.2rem;
    border-radius: 50%;
  }

  .body {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }

  .section {
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
  }

  .label {
    font-size: 0.75rem;
    font-weight: 700;
    color: var(--text-secondary);
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  .modes-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 0.35rem;
  }

  .mode-btn {
    padding: 0.35rem 0.5rem;
    font-size: 0.75rem;
    font-weight: 650;
    background: var(--surface-elevated);
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-sm);
    color: var(--text-secondary);
    cursor: pointer;
    text-align: center;
    transition: all 0.15s ease;
  }

  .mode-btn:hover {
    color: var(--text-primary);
    background: var(--surface-hover);
  }

  .mode-btn.active {
    background: var(--accent-muted);
    border-color: var(--accent-primary);
    color: var(--accent-primary);
    font-weight: 750;
  }

  .swatches {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    flex-wrap: wrap;
    padding-top: 0.2rem;
  }

  .swatch-btn {
    width: 1.6rem;
    height: 1.6rem;
    border-radius: 50%;
    background: var(--swatch);
    border: 2px solid transparent;
    cursor: pointer;
    padding: 0;
    transition:
      transform 0.15s ease,
      border-color 0.15s ease;
  }

  .swatch-btn:hover {
    transform: scale(1.15);
  }

  .swatch-btn.selected {
    border-color: var(--text-primary);
    transform: scale(1.15);
  }

  .custom-color-picker {
    position: relative;
    width: 1.6rem;
    height: 1.6rem;
    cursor: pointer;
  }

  .custom-color-picker input {
    position: absolute;
    inset: 0;
    opacity: 0;
    cursor: pointer;
    width: 100%;
    height: 100%;
  }

  .picker-preview {
    display: block;
    width: 100%;
    height: 100%;
    border-radius: 50%;
    background: conic-gradient(red, yellow, lime, aqua, blue, magenta, red);
    border: 2px solid var(--border-subtle);
  }

  .slider-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .val {
    font-size: 0.72rem;
    font-weight: 650;
    color: var(--text-muted);
  }

  .slider {
    width: 100%;
    accent-color: var(--accent-primary);
    margin: 0;
    cursor: pointer;
  }

  .foot {
    display: flex;
    justify-content: flex-end;
    padding-top: 0.2rem;
    border-top: 1px solid var(--border-subtle);
  }

  .reset-btn {
    font-size: 0.72rem;
    padding: 0.25rem 0.5rem;
    color: var(--text-muted);
    display: flex;
    align-items: center;
    gap: 0.3rem;
  }

  .reset-btn:hover {
    color: var(--text-primary);
  }

  @media (max-width: 480px) {
    .ambilight-popover {
      position: fixed;
      left: 1rem;
      right: 1rem;
      bottom: 5.5rem;
      width: auto;
    }
  }
</style>
