<script lang="ts" module>
  export interface RecapStats {
    roomLabel: string;
    minutes: number;
    videos: string[];
    reactions: Record<string, number>;
    chatters: Record<string, number>;
    messages: number;
    peakPeople: number;
  }

  export function topEntry(record: Record<string, number>): [string, number] | null {
    const entries = Object.entries(record).sort((a, b) => b[1] - a[1]);
    return entries.length ? entries[0] : null;
  }
</script>

<script lang="ts">
  import { DownloadSimple, ShareNetwork } from 'phosphor-svelte';
  import { t } from '$lib/i18n';
  import Dialog from './Dialog.svelte';

  let { stats, onClose }: { stats: RecapStats; onClose: () => void } = $props();
  const topReaction = $derived(topEntry(stats.reactions));
  const topChatter = $derived(topEntry(stats.chatters));
  const reactionTotal = $derived(Object.values(stats.reactions).reduce((sum, n) => sum + n, 0));
  const lines = $derived([
    { label: $t('recap.time'), value: $t('recap.minutes', { count: stats.minutes }) },
    { label: $t('recap.videos'), value: String(stats.videos.length) },
    { label: $t('recap.people'), value: String(stats.peakPeople) },
    {
      label: $t('recap.reactions'),
      value: topReaction ? `${reactionTotal} · ${topReaction[0]} ×${topReaction[1]}` : '0',
    },
    { label: $t('recap.chat'), value: topChatter ? `${stats.messages} · ${topChatter[0]}` : String(stats.messages) },
  ]);

  // Draws the recap as a shareable 1080×1350 image, entirely in the browser.
  function render(): HTMLCanvasElement {
    const canvas = document.createElement('canvas');
    canvas.width = 1080;
    canvas.height = 1350;
    const ctx = canvas.getContext('2d')!;
    const gradient = ctx.createLinearGradient(0, 0, 1080, 1350);
    gradient.addColorStop(0, '#16352a');
    gradient.addColorStop(1, '#0b1611');
    ctx.fillStyle = gradient;
    ctx.fillRect(0, 0, 1080, 1350);
    ctx.fillStyle = 'rgba(143,197,157,0.18)';
    ctx.beginPath();
    ctx.arc(900, 160, 320, 0, Math.PI * 2);
    ctx.fill();
    ctx.fillStyle = '#8fc59d';
    ctx.font = '700 44px Inter, system-ui, sans-serif';
    ctx.fillText('KoalaParty', 90, 140);
    ctx.fillStyle = '#eef5ef';
    ctx.font = '850 84px Inter, system-ui, sans-serif';
    ctx.fillText($t('recap.heading'), 90, 260);
    ctx.font = '600 44px Inter, system-ui, sans-serif';
    ctx.fillStyle = '#cfe3d5';
    ctx.fillText(stats.roomLabel.slice(0, 36), 90, 330);
    let y = 470;
    for (const line of lines) {
      ctx.fillStyle = 'rgba(255,255,255,0.07)';
      ctx.beginPath();
      ctx.roundRect(80, y - 70, 920, 110, 28);
      ctx.fill();
      ctx.fillStyle = '#9fb9a8';
      ctx.font = '600 34px Inter, system-ui, sans-serif';
      ctx.fillText(line.label, 120, y);
      ctx.fillStyle = '#ffffff';
      ctx.font = '800 44px Inter, system-ui, sans-serif';
      const width = ctx.measureText(line.value).width;
      ctx.fillText(line.value, 960 - width, y + 2);
      y += 140;
    }
    if (stats.videos.length) {
      ctx.fillStyle = '#9fb9a8';
      ctx.font = '600 30px Inter, system-ui, sans-serif';
      ctx.fillText(`▶ ${stats.videos[stats.videos.length - 1].slice(0, 52)}`, 90, 1240);
    }
    ctx.fillStyle = '#8fc59d';
    ctx.font = '700 30px Inter, system-ui, sans-serif';
    ctx.fillText('party.koalastuff.net', 90, 1295);
    return canvas;
  }
  function toBlob(): Promise<Blob | null> {
    return new Promise((resolve) => render().toBlob(resolve, 'image/png'));
  }
  async function download() {
    const blob = await toBlob();
    if (!blob) return;
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = 'koalaparty-recap.png';
    link.click();
    URL.revokeObjectURL(url);
  }
  async function share() {
    const blob = await toBlob();
    if (!blob) return;
    const file = new File([blob], 'koalaparty-recap.png', { type: 'image/png' });
    try {
      if (navigator.canShare?.({ files: [file] })) await navigator.share({ files: [file], title: $t('recap.heading') });
      else await download();
    } catch {
      /* dismissed */
    }
  }
</script>

<Dialog label={$t('recap.heading')} wide {onClose}>
  <div class="recap">
    <span class="spark" aria-hidden="true">🎉</span>
    <h2>{$t('recap.heading')}</h2>
    <p class="muted">{stats.roomLabel}</p>
    <dl>
      {#each lines as line (line.label)}<div>
          <dt>{line.label}</dt>
          <dd>{line.value}</dd>
        </div>{/each}
    </dl>
    {#if stats.videos.length}<details>
        <summary>{$t('recap.watched')}</summary>
        <ol>
          {#each stats.videos as title, index (`${index}-${title}`)}<li>{title}</li>{/each}
        </ol>
      </details>{/if}
  </div>
  <div class="modal-actions">
    <button class="secondary" onclick={download}
      ><DownloadSimple size={16} weight="bold" />{$t('recap.download')}</button
    >
    <button onclick={share}><ShareNetwork size={16} weight="bold" />{$t('recap.share')}</button>
  </div>
</Dialog>

<style>
  .recap {
    display: grid;
    gap: 0.4rem;
    text-align: center;
  }
  .spark {
    font-size: 2.4rem;
    animation: pop 0.6s cubic-bezier(0.2, 0.9, 0.3, 1.4);
  }
  @keyframes pop {
    from {
      transform: scale(0.3) rotate(-20deg);
    }
  }
  dl {
    display: grid;
    gap: 0.4rem;
    margin: 0.6rem 0 0;
  }
  dl div {
    display: flex;
    justify-content: space-between;
    gap: 1rem;
    padding: 0.6rem 0.8rem;
    border-radius: var(--radius-sm);
    background: var(--surface-hover);
  }
  dt {
    color: var(--text-muted);
  }
  dd {
    margin: 0;
    font-weight: 800;
  }
  details {
    text-align: left;
    font-size: 0.85rem;
    margin-top: 0.4rem;
  }
  ol {
    margin: 0.4rem 0 0;
    padding-left: 1.2rem;
    color: var(--text-secondary);
  }
  @media (prefers-reduced-motion: reduce) {
    .spark {
      animation: none;
    }
  }
</style>
