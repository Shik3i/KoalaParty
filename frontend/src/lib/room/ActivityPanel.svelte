<script lang="ts">
  import { locale, t } from '$lib/i18n';
  import { formatActivity, type Activity } from '$lib/room';

  let { events }: { events: Activity[] } = $props();
  const recent = $derived([...events].reverse());
</script>

<div class="events">
  {#if !recent.length}<p class="muted">{$t('activity.empty')}</p>{/if}
  {#each recent as event (event.id)}<article>
      <span class="dot"></span>
      <div>
        <p>{formatActivity(event, $t)}</p>
        <time datetime={event.createdAt}
          >{new Date(event.createdAt + 'Z').toLocaleTimeString($locale, { hour: '2-digit', minute: '2-digit' })}</time
        >
      </div>
    </article>{/each}
</div>

<style>
  .events {
    padding: 0.8rem 1rem;
  }
  article {
    display: flex;
    gap: 0.8rem;
    padding: 0.45rem;
  }
  p {
    margin: 0;
    font-size: 0.85rem;
  }
  time {
    font-size: 0.7rem;
    color: var(--text-muted);
  }
  .dot {
    width: 0.5rem;
    height: 0.5rem;
    border-radius: 50%;
    background: var(--accent-primary);
    margin-top: 0.35rem;
    flex: 0 0 auto;
  }
</style>
