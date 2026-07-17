<svelte:head>
  <title>Privacy Policy · KoalaParty</title>
  <meta
    name="description"
    content="How KoalaParty processes room, account, playback and technical data without analytics, ads or fingerprinting."
  />
</svelte:head>

<main class="legal panel">
  <header>
    <span aria-hidden="true">🛡️</span>
    <p>Security & privacy</p>
    <h1>Privacy Policy</h1>
    <small>Last updated: July 17, 2026</small>
  </header>

  <section>
    <h2>1. Controller and contact</h2>
    <p>
      KoalaParty is operated by Timo Schmidt, Germany. For privacy-related questions, contact
      <a href="mailto:admin@koalastuff.net">admin@koalastuff.net</a>.
    </p>
  </section>

  <section>
    <h2>2. Scope</h2>
    <p>
      This policy applies to the official KoalaParty website and service. KoalaParty can also be self-hosted; an
      independent host is responsible for its own deployment, logs, retention settings and privacy information.
    </p>
    <p>
      KoalaParty has no advertising, analytics, tracking pixels, browser fingerprinting, third-party fonts or marketing
      cookies. The source code is publicly reviewable on
      <a href="https://github.com/Shik3i/KoalaParty" target="_blank" rel="noopener noreferrer">GitHub</a>.
    </p>
  </section>

  <section>
    <h2>3. Hosting and access logs</h2>
    <p>
      The official service is hosted on infrastructure of Hetzner Online GmbH, Industriestr. 25, 91710 Gunzenhausen,
      Germany, at a server location in Finland within the EU/EEA. A data-processing agreement under Article 28 GDPR is
      in place.
    </p>
    <p>
      Standard access logs may contain the IP address, browser or user-agent, requested path and time. They are used
      only for security, stability and abuse prevention and are deleted automatically after seven days. Legal basis:
      Article 6(1)(f) GDPR.
    </p>
  </section>

  <section>
    <h2>4. Data processed by KoalaParty</h2>
    <p>Depending on the features you use, the service processes:</p>
    <ul>
      <li>a random anonymous identity, display name and avatar seed;</li>
      <li>a hash of the anonymous identity secret — never the plaintext secret;</li>
      <li>an optional username and Argon2id password hash; no email address is requested;</li>
      <li>hashed session tokens, CSRF tokens and expiry times;</li>
      <li>friend requests, accepted or blocked relationships;</li>
      <li>room ownership, membership, roles, permissions, invitations and bans;</li>
      <li>YouTube video IDs, supplied titles, queue order and synchronized playback state;</li>
      <li>structured room activity and reports submitted for operator review.</li>
    </ul>
    <p>
      Public rooms and their submitted titles can be visible to visitors. Room state is shared with eligible
      participants in the same room. Private and friends-only rooms require an account.
    </p>
  </section>

  <section>
    <h2>5. Purposes and legal bases</h2>
    <p>
      Identity, account, room and playback data is processed to provide the service requested by you, enforce room
      permissions, keep participants synchronized and prevent abuse. Legal basis: Article 6(1)(f) GDPR based on the
      legitimate interests of providing the requested functionality and maintaining secure, reliable operation and
      moderation.
    </p>
  </section>

  <section>
    <h2>6. Local storage and session cookie</h2>
    <p>
      Your browser stores the random identity UUID and secret, display name, avatar seed and theme in local storage.
      These values are required for identity continuity and interface preferences, not tracking. You can remove them
      through your browser settings, but doing so before linking an account permanently loses anonymous room ownership.
    </p>
    <p>
      The server sets the technically necessary <code>kp_session</code> cookie after authentication. It is HttpOnly and SameSite=Lax;
      production deployments additionally mark it Secure. The default session lifetime is seven days.
    </p>
  </section>

  <section>
    <h2>7. YouTube</h2>
    <p>
      KoalaParty uses locally hosted interface assets before playback consent. YouTube thumbnails, the iframe API and
      the privacy-enhanced player from <code>i.ytimg.com</code>, <code>youtube.com</code> and
      <code>youtube-nocookie.com</code> load only after you select “Start watching”.
    </p>
    <p>
      Google or YouTube may then receive your IP address, device and browser information, referrer, request time and
      player interactions and may process data outside the EU/EEA under its own privacy terms. Do not select “Start
      watching” if you do not want to load YouTube. Legal basis for initiating this optional connection: your consent
      under Article 6(1)(a) GDPR. Reload the page to stop the embedded player and withdraw consent for future requests;
      this does not affect processing that already occurred.
    </p>
  </section>

  <section>
    <h2>8. Retention</h2>
    <ul>
      <li>Expired sessions are deleted automatically; their default lifetime is seven days.</li>
      <li>Room activity is limited to 200 visible events per room and 30 days by default.</li>
      <li>Rooms inactive for 12 months are soft-deleted by default when nobody is connected.</li>
      <li>Accounts, identities, friendships, active bans and durable room state remain until deleted or anonymized.</li>
      <li>Reports remain available for operator review until resolved or deleted.</li>
    </ul>
    <p>
      The current version has no self-service account or room deletion interface. Deletion requests can be sent to the
      contact address above. Self-hosters can configure different retention periods.
    </p>
  </section>

  <section>
    <h2>9. Rate limiting and abuse prevention</h2>
    <p>
      Authentication and room-command endpoints temporarily count requests per IP address in memory. Entries expire
      after their short fixed window and are not written to the KoalaParty database or application logs. KoalaParty does
      not use fingerprinting or permanent IP bans; consequently, anonymous bans can be bypassed by clearing browser
      storage.
    </p>
  </section>

  <section>
    <h2>10. Recipients and transfers</h2>
    <p>
      Technical hosting data can be processed by Hetzner on behalf of the operator. Room data is disclosed only as
      required by the selected room visibility and permissions. YouTube receives data only as described above. No data
      is sold or disclosed to analytics providers, advertisers or data brokers.
    </p>
  </section>

  <section>
    <h2>11. Your rights</h2>
    <p>
      Subject to the GDPR, you may have rights to access, rectification, erasure, restriction, data portability and
      objection, and the right to withdraw consent with future effect. You may also lodge a complaint with a competent
      supervisory authority. KoalaParty does not collect extra identification data solely to identify a person making a
      request.
    </p>
  </section>

  <section>
    <h2>12. No profiling or automated decisions</h2>
    <p>KoalaParty does not use profiling or automated decision-making within the meaning of Article 22 GDPR.</p>
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
  header span {
    display: block;
    font-size: 3rem;
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
    margin-bottom: 0.6rem;
  }
  header small,
  .legal p,
  .legal li {
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
  p:last-child,
  ul:last-child {
    margin-bottom: 0;
  }
  li + li {
    margin-top: 0.35rem;
  }
  code {
    color: var(--text-primary);
  }
  @media (max-width: 860px) {
    .legal {
      margin-inline: 1rem;
    }
  }
</style>
