<svelte:head><title>Privacy · KoalaParty</title></svelte:head>
<main class="legal panel">
  <h1>Privacy by construction</h1>
  <p>
    KoalaParty does not track users, show ads, sell data, fingerprint browsers, or load analytics and third-party fonts.
  </p>
  <h2>What is stored</h2>
  <p>
    Your browser stores a random identity UUID, a secret, display name, avatar seed, and theme. The server stores only a
    secure hash of the identity secret, permanent room state, permissions, bans, accounts you choose to create, and
    recent structured activity. Activity is limited to 200 visible events and 30 days by default.
  </p>
  <h2>YouTube</h2>
  <p>
    The privacy-enhanced YouTube player is the only required browser-side third party. Loading it may allow Google or
    YouTube to process your IP address and technical device data under their own terms.
  </p>
  <h2>Anonymous ownership</h2>
  <p>
    Deleting browser data before linking an account permanently loses anonymous room ownership. Anonymous bans can be
    bypassed by clearing browser data; KoalaParty deliberately avoids invasive enforcement.
  </p>
  <h2>Operator information</h2>
  <p class="muted">
    The deployer must add jurisdiction-specific operator identity, contact, retention, and legal-basis information here
    before public production use.
  </p>
</main>

<style>
  .legal {
    max-width: 760px;
    margin: 4rem auto;
    padding: clamp(1.5rem, 5vw, 3rem);
    line-height: 1.7;
  }
</style>
