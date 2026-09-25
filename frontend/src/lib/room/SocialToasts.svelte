<script lang="ts" module>
  export interface SocialToast {
    id: string;
    badge: string;
    name: string;
    text: string;
    tone: 'join' | 'leave' | 'action' | 'chat' | 'reaction';
  }
</script>

<script lang="ts">
  import { fly, fade } from 'svelte/transition';
  import { flip } from 'svelte/animate';

  let { toasts, placement = 'corner' }: { toasts: SocialToast[]; placement?: 'corner' | 'overlay' } = $props();
</script>

<div class="social-toasts {placement}" aria-live="polite" aria-label="Room updates">
  {#each toasts as toast (toast.id)}<div
      class="toast {toast.tone}"
      animate:flip={{ duration: 220 }}
      in:fly={{ x: placement === 'corner' ? -24 : 0, y: placement === 'overlay' ? -10 : 0, duration: 260 }}
      out:fade={{ duration: 220 }}
    >
      <span class="badge" aria-hidden="true">{toast.badge}</span>
      <p><b>{toast.name}</b> {toast.text}</p>
    </div>{/each}
</div>

<style>
  .social-toasts {
    display: grid;
    gap: 0.4rem;
    pointer-events: none;
    z-index: 65;
  }
  .corner {
    position: fixed;
    left: 1rem;
    bottom: 1rem;
    width: min(21rem, calc(100vw - 2rem));
  }
  .overlay {
    position: absolute;
    left: 0.8rem;
    top: 0.8rem;
    width: min(24rem, 70%);
  }
  .toast {
    display: flex;
    align-items: center;
    gap: 0.55rem;
    padding: 0.45rem 0.8rem 0.45rem 0.45rem;
    border-radius: 999px;
    background: color-mix(in srgb, var(--surface-elevated) 88%, transparent);
    border: 1px solid var(--border-subtle);
    box-shadow: var(--shadow-panel);
    backdrop-filter: blur(10px);
    color: var(--text-primary);
  }
  .overlay .toast {
    background: rgba(8, 14, 11, 0.72);
    border-color: transparent;
    color: white;
  }
  .toast.join {
    border-color: color-mix(in srgb, var(--success) 50%, var(--border-subtle));
  }
  .toast.reaction {
    border-color: color-mix(in srgb, var(--accent-primary) 50%, var(--border-subtle));
  }
  .badge {
    flex: 0 0 auto;
    width: 1.9rem;
    height: 1.9rem;
    display: grid;
    place-content: center;
    border-radius: 50%;
    background: var(--accent-muted);
    font-size: 1.05rem;
  }
  .join .badge {
    animation: wave 0.9s ease 1;
  }
  @keyframes wave {
    30% {
      transform: rotate(-14deg) scale(1.15);
    }
    60% {
      transform: rotate(10deg) scale(1.1);
    }
  }
  p {
    margin: 0;
    font-size: 0.8rem;
    line-height: 1.3;
    overflow: hidden;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow-wrap: anywhere;
  }
  @media (max-width: 700px) {
    .corner {
      left: 50%;
      transform: translateX(-50%);
      bottom: calc(8.2rem + env(safe-area-inset-bottom));
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .join .badge {
      animation: none;
    }
  }
</style>
