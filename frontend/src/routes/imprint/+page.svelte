<svelte:head>
  <title>Legal Notice · KoalaParty</title>
  <meta name="description" content="Operator, contact and legal information for KoalaParty." />
</svelte:head>

<main class="legal panel">
  <header>
    <img src="/icons/koalaparty-192.png" alt="" />
    <p>Contact & project information</p>
    <h1>Legal Notice</h1>
  </header>

  <section>
    <h2>Operator and contact</h2>
    <p>
      Timo Schmidt<br />
      Germany
    </p>
    <p>Email: <a href="mailto:admin@koalastuff.net">admin@koalastuff.net</a></p>
    <p>
      Private message:
      <a href="https://mastodon.social/@koalastuff" target="_blank" rel="me noopener noreferrer"
        >@koalastuff on Mastodon</a
      >
    </p>
    <p>
      Bug reports and security concerns:
      <a href="https://github.com/Shik3i/KoalaParty/issues" target="_blank" rel="noopener noreferrer">GitHub Issues</a>
    </p>
  </section>

  <section>
    <h2>Project information</h2>
    <p>
      KoalaParty is a free, non-commercial hobby project for synchronized YouTube rooms. Its source code is publicly
      available on <a href="https://github.com/Shik3i/KoalaParty" target="_blank" rel="noopener noreferrer">GitHub</a>.
      The source code is free and open-source software licensed under the
      <a href="https://github.com/Shik3i/KoalaParty/blob/main/LICENSE" target="_blank" rel="noopener noreferrer"
        >MIT License</a
      >.
    </p>
    <p>
      Details about hosting, stored room and account data, YouTube, retention and data-subject rights are provided in
      the
      <a href="/privacy">Privacy Policy</a>.
    </p>
  </section>

  <section>
    <h2>Responsibility for content</h2>
    <p>
      The contents of this website were created with care. However, I cannot guarantee that all information is always
      complete, accurate or current. As a service provider, I am responsible for my own content in accordance with the
      general laws.
    </p>
  </section>

  <section>
    <h2>Responsibility for links</h2>
    <p>
      This website links to external third-party websites whose content is outside my control. The respective provider
      or operator is responsible for linked pages. If I become aware of unlawful content, I will remove the link.
    </p>
  </section>

  <section>
    <h2>Copyright and trademarks</h2>
    <p>
      Project names, logos, graphics, website text and other assets are protected by applicable copyright and trademark
      law unless expressly stated otherwise. KoalaParty and KoalaSync are not affiliated with or endorsed by YouTube,
      Netflix, Disney+, Amazon, Twitch, Jellyfin, Emby or other listed media platforms. All third-party trademarks
      belong to their respective owners.
    </p>
  </section>

  <section>
    <h2>Self-hosted deployments</h2>
    <p>
      Independent KoalaParty deployments are operated by their respective hosts. Their operators are responsible for
      their own legal notice, privacy policy, configuration, content and moderation.
    </p>
  </section>
</main>

<style>
  .legal {
    max-width: 820px;
    margin: clamp(2rem, 7vw, 5rem) auto;
    padding: clamp(1.5rem, 5vw, 3rem);
    line-height: 1.72;
  }
  header {
    text-align: center;
    padding-bottom: 2rem;
    border-bottom: 1px solid var(--border-subtle);
  }
  header img {
    display: block;
    width: 4rem;
    height: 4rem;
    object-fit: contain;
    margin-inline: auto;
    margin-bottom: 0.5rem;
  }
  header p {
    color: var(--accent-primary);
    font-size: 0.75rem;
    font-weight: 800;
    letter-spacing: 0.12em;
    text-transform: uppercase;
  }
  header h1 {
    font-size: clamp(2.25rem, 7vw, 4rem);
    margin-bottom: 0;
  }
  .legal p {
    color: var(--text-secondary);
  }
  section {
    padding-top: 2rem;
  }
  section + section {
    border-top: 1px solid var(--border-subtle);
    margin-top: 2rem;
  }
  h2 {
    font-size: 1.25rem;
  }
  p:last-child {
    margin-bottom: 0;
  }
  @media (max-width: 860px) {
    .legal {
      margin-inline: 1rem;
    }
  }
</style>
