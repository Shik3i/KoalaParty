<script lang="ts">
  import { locale } from '$lib/i18n';
</script>

<svelte:head>
  <title>{$locale === 'de' ? 'Datenschutzerklärung' : 'Privacy Policy'} · KoalaParty</title>
  <meta
    name="description"
    content="How KoalaParty processes room, account, playback and technical data without analytics, ads or fingerprinting."
  />
</svelte:head>

<main class="legal panel" lang={$locale}>
  {#if $locale === 'de'}
    <header>
      <span aria-hidden="true">🛡️</span>
      <p>Sicherheit & Datenschutz</p>
      <h1>Datenschutzerklärung</h1>
      <small>Stand: 26. September 2026</small>
    </header>

    <section>
      <h2>1. Verantwortlicher und Kontakt</h2>
      <p>
        KoalaParty wird von Timo Schmidt, Deutschland, betrieben. Bei Fragen zum Datenschutz erreichst du uns unter
        <a href="mailto:admin@koalastuff.net">admin@koalastuff.net</a>.
      </p>
    </section>

    <section>
      <h2>2. Geltungsbereich</h2>
      <p>
        Diese Erklärung gilt für die offizielle KoalaParty-Website und den offiziellen Dienst. KoalaParty kann auch
        selbst gehostet werden; unabhängige Betreiber sind für ihre eigene Installation, Protokolle,
        Aufbewahrungsfristen und Datenschutzhinweise selbst verantwortlich.
      </p>
      <p>
        KoalaParty nutzt keine Werbung, kein KoalaParty-Tracking, keine Tracking-Pixel, kein Browser-Fingerprinting,
        keine Schriften von Drittanbietern und keine Marketing-Cookies. Der Quellcode ist öffentlich auf
        <a href="https://github.com/Shik3i/KoalaParty" target="_blank" rel="noopener noreferrer">GitHub</a> einsehbar.
      </p>
    </section>

    <section>
      <h2>3. Hosting und Zugriffsprotokolle</h2>
      <p>
        Der offizielle Dienst läuft auf Infrastruktur der Hetzner Online GmbH, Industriestr. 25, 91710 Gunzenhausen,
        Deutschland, mit Serverstandort in Finnland innerhalb der EU/des EWR. Ein Vertrag zur Auftragsverarbeitung nach
        Art. 28 DSGVO besteht.
      </p>
      <p>
        Übliche Zugriffsprotokolle können IP-Adresse, Browser bzw. User-Agent, aufgerufenen Pfad und Zeitpunkt
        enthalten. Sie dienen ausschließlich der Sicherheit, Stabilität und Missbrauchsabwehr und werden nach sieben
        Tagen automatisch gelöscht. Rechtsgrundlage: Art. 6 Abs. 1 lit. f DSGVO.
      </p>
    </section>

    <section>
      <h2>4. Von KoalaParty verarbeitete Daten</h2>
      <p>Je nachdem, welche Funktionen du nutzt, verarbeitet der Dienst:</p>
      <ul>
        <li>eine zufällige anonyme Identität, einen Anzeigenamen und einen Avatar-Wert;</li>
        <li>einen Hash des Geheimnisses deiner anonymen Identität – nie das Geheimnis selbst;</li>
        <li>
          optional einen Benutzernamen und einen Argon2id-Passwort-Hash; eine E-Mail-Adresse wird nicht abgefragt;
        </li>
        <li>gehashte Sitzungstoken, CSRF-Token und Ablaufzeiten;</li>
        <li>Freundschaftsanfragen sowie bestätigte oder blockierte Beziehungen;</li>
        <li>Raumbesitz, Mitgliedschaften, Rollen, Berechtigungen, Einladungen und Verbannungen;</li>
        <li>
          Raumeinstellungen wie einen eigenen Namen, einen Kurzlink, den Raum-Modus, den Countdown und eine geplante
          Startzeit;
        </li>
        <li>bei Accounts gespeicherte Warteschlangen (deren Name und Video-IDs);</li>
        <li>
          YouTube-Video-IDs, vom Server ermittelte Titel, die Reihenfolge der Warteschlange und den synchronisierten
          Wiedergabestatus;
        </li>
        <li>strukturierte Raumaktivität sowie Meldungen zur Prüfung durch den Betreiber.</li>
      </ul>
      <p>
        Öffentliche Räume und ihre vom Server ermittelten Titel können für Besucher sichtbar sein. Der Raumzustand wird
        mit den berechtigten Teilnehmenden desselben Raums geteilt. Private und Nur-Freunde-Räume erfordern einen
        Account.
      </p>
      <p>
        Emoji-Reaktionen werden live an die verbundenen Teilnehmenden gesendet, sind begrenzt und werden sofort
        verworfen. Sie werden weder in der Raumaktivität noch in der Datenbank gespeichert.
      </p>
      <p>
        Chatnachrichten und der Player-Status (zum Beispiel „puffert“ oder „muss für Ton tippen“) werden an die Personen
        weitergegeben, die mit demselben Raum verbunden sind. Der Server hält höchstens die letzten 60 Chatnachrichten
        im Arbeitsspeicher, damit Neuankömmlinge mitlesen können; sie werden gelöscht, sobald die letzte Person den Raum
        verlässt oder der Server neu startet. Der Chat wird nie in die Datenbank, in Protokolle oder in die
        Raumaktivität geschrieben.
      </p>
      <p>
        Für die Reaktions-Heatmap zählt der Server Reaktionen je wenige Sekunden des aktuellen Videos im
        Arbeitsspeicher. Gespeichert werden nur Anzahlen – nicht, wer reagiert hat – und sie verfallen mit dem Raum. Der
        Party-Rückblick wird ausschließlich in deinem Browser berechnet.
      </p>
    </section>

    <section>
      <h2>5. Zwecke und Rechtsgrundlagen</h2>
      <p>
        Identitäts-, Account-, Raum- und Wiedergabedaten werden verarbeitet, um den von dir gewünschten Dienst
        bereitzustellen, Raumberechtigungen durchzusetzen, die Teilnehmenden synchron zu halten und Missbrauch zu
        verhindern. Rechtsgrundlage: Art. 6 Abs. 1 lit. f DSGVO, gestützt auf das berechtigte Interesse, die gewünschte
        Funktion bereitzustellen und einen sicheren, zuverlässigen Betrieb samt Moderation zu gewährleisten.
      </p>
    </section>

    <section>
      <h2>6. Lokaler Speicher und Sitzungs-Cookie</h2>
      <p>
        Dein Browser speichert die zufällige Identitäts-UUID samt Geheimnis, Anzeigename, Avatar-Wert, Sprache, Design
        und Farbschema sowie bis zu fünf Verknüpfungen zu zuletzt besuchten Räumen im lokalen Speicher. Diese Werte sind
        für eine durchgehende Identität, Oberflächeneinstellungen und die Rückkehr in Räume erforderlich, nicht für
        Tracking. Verknüpfungen zu Räumen kannst du auf der Startseite einzeln entfernen. Alle Werte lassen sich über
        die Browsereinstellungen löschen; geschieht das, bevor du einen Account verknüpft hast, geht der anonyme
        Raumbesitz dauerhaft verloren.
      </p>
      <p>
        Nach der Anmeldung setzt der Server das technisch notwendige Cookie <code>kp_session</code> (im Produktivbetrieb
        mit dem Präfix <code>__Host-</code>). Es ist HttpOnly und SameSite=Lax; im Produktivbetrieb zusätzlich Secure.
        Die Sitzung ist standardmäßig sieben Tage gültig.
      </p>
    </section>

    <section>
      <h2>7. YouTube und SponsorBlock</h2>
      <p>
        Die Oberfläche wird lokal ausgeliefert. Sobald in einem Raum ein Video läuft, lädt KoalaParty den
        datenschutzfreundlichen eingebetteten YouTube-Player und Vorschaubilder von <code>i.ytimg.com</code>,
        <code>youtube.com</code> und <code>youtube-nocookie.com</code>; ein kurzer Hinweis unter dem Player weist darauf
        hin. Ein leerer Raum stellt keine Verbindung zu YouTube her.
      </p>
      <p>
        Google bzw. YouTube kann dabei deine IP-Adresse, Geräte- und Browserinformationen, den Referrer, den Zeitpunkt
        der Anfrage und Interaktionen mit dem Player erhalten und Daten nach eigenen Bestimmungen auch außerhalb der
        EU/des EWR verarbeiten. Wenn du YouTube nicht laden möchtest, öffne keinen Raum mit laufendem Video. Durch
        Neuladen oder Verlassen des Raums beendest du den eingebetteten Player; bereits erfolgte Verarbeitungen bleiben
        davon unberührt.
      </p>
      <p>
        Wird einem Raum ein Video hinzugefügt, fragt der Server einmalig den öffentlichen oEmbed-Endpunkt von YouTube
        nach dem Titel. Diese Anfrage geht vom Server aus: YouTube erfährt die Video-ID und die IP-Adresse des Servers,
        nie deinen Browser oder deine IP. Betreiber können das mit <code>KOALAPARTY_YOUTUBE_METADATA=false</code> vollständig
        abschalten; dann wird statt des Titels die Video-ID angezeigt.
      </p>
      <p>
        Hat der Betreiber die Suche in der App aktiviert, werden von dir eingegebene Suchbegriffe und Playlist-IDs vom
        Server an die YouTube Data API gesendet, um Videos zu finden. Die Anfrage kommt vom Server, nicht von deinem
        Browser, und ist nicht mit deiner Identität verknüpft; Ergebnisse werden 15 Minuten im Arbeitsspeicher des
        Servers zwischengespeichert. Ohne diese Einstellung findet keine solche Anfrage statt.
      </p>
      <p>
        Ist SponsorBlock für einen Raum aktiviert, lädt der Server zusätzlich Überspring-Abschnitte für das aktuelle
        Video von <a href="https://sponsor.ajay.app" target="_blank" rel="noopener noreferrer">SponsorBlock</a>. Dabei
        wird der datenschutzfreundliche Hash-Präfix-Endpunkt genutzt: Übertragen werden nur die ersten vier Zeichen des
        SHA-256-Hashs der Video-ID – passend zu vielen Videos gleichzeitig –, nie die vollständige Video-ID, dein
        Browser oder deine IP. Die Abschnittsdaten stehen unter der Lizenz
        <a href="https://creativecommons.org/licenses/by-nc-sa/4.0/" target="_blank" rel="noopener noreferrer"
          >CC BY-NC-SA 4.0</a
        >. Besitzer und Admins können die Funktion pro Raum abschalten, Betreiber vollständig mit
        <code>KOALAPARTY_SPONSORBLOCK=false</code>.
      </p>
    </section>

    <section>
      <h2>8. Speicherdauer</h2>
      <ul>
        <li>Abgelaufene Sitzungen werden automatisch gelöscht; standardmäßig sind sie sieben Tage gültig.</li>
        <li>Die Raumaktivität ist standardmäßig auf 200 sichtbare Einträge pro Raum und 30 Tage begrenzt.</li>
        <li>
          Räume, die 12 Monate inaktiv waren, werden standardmäßig vorläufig gelöscht, wenn niemand verbunden ist.
        </li>
        <li>
          Accounts, Identitäten, Freundschaften, aktive Verbannungen, gespeicherte Warteschlangen und dauerhafter
          Raumzustand bleiben bis zur Löschung oder Anonymisierung erhalten.
        </li>
        <li>Meldungen bleiben zur Prüfung durch den Betreiber verfügbar, bis sie erledigt oder gelöscht sind.</li>
      </ul>
      <p>
        Mit einem Account kannst du deinen Account und deine Räume direkt in der Oberfläche löschen. Dabei werden alle
        Sitzungen beendet, verbleibende Verweise auf deine Identität anonymisiert und Räume geschlossen, deren Besitz
        nicht vorher übertragen wurde. Weitere Löschanfragen kannst du an die oben genannte Adresse richten. Selbst
        gehostete Installationen können andere Fristen festlegen.
      </p>
    </section>

    <section>
      <h2>9. Begrenzung von Anfragen und Missbrauchsabwehr</h2>
      <p>
        Anmelde- und Raumbefehls-Endpunkte zählen Anfragen pro IP-Adresse vorübergehend im Arbeitsspeicher. Die Einträge
        verfallen nach ihrem kurzen festen Zeitfenster und werden weder in die KoalaParty-Datenbank noch in
        Anwendungsprotokolle geschrieben. KoalaParty nutzt weder Fingerprinting noch dauerhafte IP-Sperren; anonyme
        Verbannungen lassen sich deshalb durch Löschen des Browserspeichers umgehen.
      </p>
    </section>

    <section>
      <h2>10. Empfänger und Übermittlungen</h2>
      <p>
        Technische Hosting-Daten können von Hetzner im Auftrag des Betreibers verarbeitet werden. Raumdaten werden nur
        im Rahmen der gewählten Sichtbarkeit und Berechtigungen offengelegt. YouTube erhält Daten nur wie oben
        beschrieben. Es werden keine Daten verkauft oder an Analyseanbieter, Werbetreibende oder Datenhändler
        weitergegeben.
      </p>
    </section>

    <section>
      <h2>11. Deine Rechte</h2>
      <p>
        Nach Maßgabe der DSGVO hast du gegebenenfalls das Recht auf Auskunft, Berichtigung, Löschung, Einschränkung der
        Verarbeitung, Datenübertragbarkeit und Widerspruch sowie das Recht, eine Einwilligung mit Wirkung für die
        Zukunft zu widerrufen. Außerdem kannst du dich bei einer zuständigen Aufsichtsbehörde beschweren. KoalaParty
        erhebt keine zusätzlichen Daten, nur um eine anfragende Person zu identifizieren.
      </p>
    </section>

    <section>
      <h2>12. Kein Profiling, keine automatisierten Entscheidungen</h2>
      <p>KoalaParty nutzt kein Profiling und keine automatisierten Entscheidungen im Sinne von Art. 22 DSGVO.</p>
    </section>
  {:else}
    <header>
      <span aria-hidden="true">🛡️</span>
      <p>Security & privacy</p>
      <h1>Privacy Policy</h1>
      <small>Last updated: September 26, 2026</small>
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
        KoalaParty has no advertising, KoalaParty analytics, tracking pixels, browser fingerprinting, third-party fonts
        or marketing cookies. The source code is publicly reviewable on
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
        <li>room settings such as a custom name, short link, room mode, countdown and a scheduled start time;</li>
        <li>for account holders, saved queues (their name and video IDs);</li>
        <li>YouTube video IDs, server-resolved titles, queue order and synchronized playback state;</li>
        <li>structured room activity and reports submitted for operator review.</li>
      </ul>
      <p>
        Public rooms and their server-resolved titles can be visible to visitors. Room state is shared with eligible
        participants in the same room. Private and friends-only rooms require an account.
      </p>
      <p>
        Emoji reactions are broadcast live to connected room participants, rate-limited and discarded immediately. They
        are not stored in room activity or the database.
      </p>
      <p>
        Room chat messages and player presence (for example “buffering” or “needs a tap for sound”) are relayed to the
        people connected to the same room. The server keeps at most the last 60 chat messages in memory so people who
        join can catch up; they are deleted as soon as the last person leaves the room or the server restarts. Chat is
        never written to the database, logs or room activity.
      </p>
      <p>
        For the reaction heatmap, the server counts reactions per few seconds of the current video in memory. Only
        counts are kept — not who reacted — and they are discarded with the room. The party recap is computed only in
        your browser.
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
        Your browser stores the random identity UUID and secret, display name, avatar seed, language, theme and color
        design, and up to five recently visited room shortcuts in local storage. These values are required for identity
        continuity, interface preferences and returning to rooms, not tracking. Recent-room shortcuts can be removed
        individually on the start page. You can remove all values through your browser settings, but doing so before
        linking an account permanently loses anonymous room ownership.
      </p>
      <p>
        The server sets the technically necessary <code>kp_session</code> cookie (prefixed <code>__Host-</code> in production)
        after authentication. It is HttpOnly and SameSite=Lax; production deployments additionally mark it Secure. The default
        session lifetime is seven days.
      </p>
    </section>

    <section>
      <h2>7. YouTube and SponsorBlock</h2>
      <p>
        Interface assets are hosted locally. When you open a room, KoalaParty loads YouTube's privacy-enhanced embedded
        player and thumbnails from <code>i.ytimg.com</code>, <code>youtube.com</code> and
        <code>youtube-nocookie.com</code> as soon as the room plays a video, and a short notice below the player states this.
        An empty room does not contact YouTube.
      </p>
      <p>
        Google or YouTube may then receive your IP address, device and browser information, referrer, request time and
        player interactions and may process data outside the EU/EEA under its own privacy terms. If you do not want to
        load YouTube, do not open a room. Reload or leave the room to stop the embedded player; this does not affect
        processing that already occurred.
      </p>
      <p>
        When a video is added to a room, the server makes one request to YouTube's public oEmbed endpoint to resolve the
        video's title. This is a server-to-YouTube request: it reveals the video ID and the server's IP address, never
        your browser or IP. Operators can disable it entirely with <code>KOALAPARTY_YOUTUBE_METADATA=false</code>, in
        which case titles fall back to the video ID.
      </p>
      <p>
        If the operator enables in-app search, search terms and playlist IDs you enter are sent from the server to the
        YouTube Data API to find videos. The request comes from the server, not from your browser, and is not linked to
        your identity; results are cached in server memory for 15 minutes. Without this configuration, no such request
        is made.
      </p>
      <p>
        When SponsorBlock is enabled for a room, the server also fetches skip segments for the current video from
        <a href="https://sponsor.ajay.app" target="_blank" rel="noopener noreferrer">SponsorBlock</a>. It uses
        SponsorBlock's privacy-preserving hash-prefix endpoint: only the first four characters of the video ID's SHA-256
        hash are sent — matching many videos at once — never the full video ID or your browser or IP. Segment data is
        licensed
        <a href="https://creativecommons.org/licenses/by-nc-sa/4.0/" target="_blank" rel="noopener noreferrer"
          >CC BY-NC-SA 4.0</a
        >. Owners and admins can turn it off per room, and operators can disable it entirely with
        <code>KOALAPARTY_SPONSORBLOCK=false</code>.
      </p>
    </section>

    <section>
      <h2>8. Retention</h2>
      <ul>
        <li>Expired sessions are deleted automatically; their default lifetime is seven days.</li>
        <li>Room activity is limited to 200 visible events per room and 30 days by default.</li>
        <li>Rooms inactive for 12 months are soft-deleted by default when nobody is connected.</li>
        <li>
          Accounts, identities, friendships, active bans and durable room state remain until deleted or anonymized.
        </li>
        <li>Reports remain available for operator review until resolved or deleted.</li>
      </ul>
      <p>
        Account holders can delete their account and owned rooms directly in the interface. Account deletion revokes all
        sessions, anonymizes retained identity references and closes rooms whose ownership was not transferred first.
        Additional deletion requests can be sent to the contact address above. Self-hosters can configure different
        retention periods.
      </p>
    </section>

    <section>
      <h2>9. Rate limiting and abuse prevention</h2>
      <p>
        Authentication and room-command endpoints temporarily count requests per IP address in memory. Entries expire
        after their short fixed window and are not written to the KoalaParty database or application logs. KoalaParty
        does not use fingerprinting or permanent IP bans; consequently, anonymous bans can be bypassed by clearing
        browser storage.
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
        supervisory authority. KoalaParty does not collect extra identification data solely to identify a person making
        a request.
      </p>
    </section>

    <section>
      <h2>12. No profiling or automated decisions</h2>
      <p>KoalaParty does not use profiling or automated decision-making within the meaning of Article 22 GDPR.</p>
    </section>
  {/if}
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
