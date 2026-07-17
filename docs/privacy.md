# Privacy model

KoalaParty has no analytics, ads, tracking pixels, third-party fonts, fingerprinting, or marketing cookies. Interface assets are served locally, and no YouTube host is contacted before explicit playback consent. Anonymous identity credentials remain in local browser storage; the server stores only a secret hash. Recent structured room activity is retained for at most 200 events and 30 days by default. Application logs exclude secrets, cookies, full WebSocket payloads, and long-term IP storage.

YouTube is the only browser-side third party. Loading its privacy-enhanced player can still allow Google/YouTube to process technical data. Private and friends-only rooms require accounts. Deleting browser storage before linking an account permanently loses anonymous ownership; there is no recovery key. Anonymous bans can be bypassed by clearing browser data, an accepted privacy trade-off.

The official deployment publishes its operator, hosting, retention, YouTube, legal-basis, and contact information on the `/privacy` and `/imprint` routes. Independent deployers must replace those deployment-specific details with their own jurisdiction-specific notices before public use.
