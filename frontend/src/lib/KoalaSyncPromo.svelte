<script lang="ts">
  const platforms = [
    { name: 'Netflix', icon: '/assets/platforms/netflix.svg' },
    { name: 'YouTube', icon: '/assets/platforms/youtube.svg' },
    { name: 'Twitch', icon: '/assets/platforms/twitch.svg' },
    { name: 'Prime Video', icon: '/assets/platforms/prime-video.svg', wide: true },
    { name: 'Disney+', icon: '/assets/platforms/disney-plus.svg', wide: true },
    { name: 'Jellyfin', icon: '/assets/platforms/jellyfin.svg' },
    { name: 'Emby', icon: '/assets/platforms/emby.svg' },
  ];
</script>

<section class="sync-promo panel" aria-labelledby="koalasync-title">
  <div class="copy">
    <p class="eyebrow">Need more than YouTube?</p>
    <h2 id="koalasync-title">Take the watch party to almost any video site.</h2>
    <p>
      Want to watch Netflix, Disney+, Prime Video, Twitch or your own media server together? KoalaSync synchronizes the
      player directly in every participant's browser.
    </p>
    <a class="button" href="https://sync.koalastuff.net/" target="_blank" rel="noopener noreferrer">
      See KoalaSync <span aria-hidden="true">↗</span>
    </a>
  </div>

  <div class="platforms" aria-label="Platforms supported by KoalaSync">
    {#each platforms as platform}
      <div class:wide={platform.wide} class="platform">
        <img src={platform.icon} alt="" width={platform.wide ? 54 : 28} height="28" loading="lazy" />
        <span>{platform.name}</span>
      </div>
    {/each}
    <div class="platform more"><strong>+</strong><span>many more</span></div>
  </div>

  <p class="trademark">
    KoalaParty and KoalaSync are not affiliated with or endorsed by the listed platforms. Their trademarks belong to
    their respective owners.
  </p>
</section>

<style>
  .sync-promo {
    max-width: 1180px;
    margin: 0 auto 5rem;
    padding: clamp(1.5rem, 4vw, 3rem);
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(320px, 0.85fr);
    gap: clamp(2rem, 6vw, 5rem);
    align-items: center;
    overflow: hidden;
    position: relative;
  }
  .sync-promo::after {
    content: '';
    position: absolute;
    width: 20rem;
    height: 20rem;
    right: -8rem;
    top: -10rem;
    border-radius: 50%;
    background: var(--accent-muted);
    filter: blur(1px);
    pointer-events: none;
  }
  .copy,
  .platforms,
  .trademark {
    position: relative;
    z-index: 1;
  }
  .eyebrow {
    color: var(--accent-primary);
    font-size: 0.76rem;
    font-weight: 800;
    letter-spacing: 0.12em;
    text-transform: uppercase;
    margin-bottom: 0.8rem;
  }
  h2 {
    font-size: clamp(1.75rem, 4vw, 3rem);
    line-height: 1.08;
    margin-bottom: 1rem;
  }
  .copy > p:not(.eyebrow) {
    color: var(--text-secondary);
    line-height: 1.65;
    max-width: 38rem;
  }
  .button {
    margin-top: 0.5rem;
  }
  .platforms {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0.65rem;
  }
  .platform {
    min-height: 4rem;
    display: flex;
    align-items: center;
    gap: 0.7rem;
    padding: 0.8rem 0.9rem;
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-sm);
    background: color-mix(in srgb, var(--surface-elevated) 92%, transparent);
    color: var(--text-secondary);
    font-size: 0.84rem;
    font-weight: 700;
  }
  .platform img {
    width: 28px;
    height: 28px;
    object-fit: contain;
    flex: 0 0 auto;
  }
  .platform.wide img {
    width: 54px;
  }
  .more strong {
    display: grid;
    place-items: center;
    width: 28px;
    height: 28px;
    border-radius: 50%;
    background: var(--accent-muted);
    color: var(--accent-primary);
    font-size: 1.2rem;
  }
  .trademark {
    grid-column: 1 / -1;
    margin: -1rem 0 0;
    color: var(--text-muted);
    font-size: 0.72rem;
    line-height: 1.5;
    text-align: center;
  }
  @media (max-width: 800px) {
    .sync-promo {
      margin-inline: 1rem;
      grid-template-columns: minmax(0, 1fr);
    }
  }
  @media (max-width: 440px) {
    .platforms {
      grid-template-columns: minmax(0, 1fr);
    }
  }
</style>
