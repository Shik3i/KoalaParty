<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/state';
  import { api, establish, websocketURL } from '$lib/api';
  import YouTubePlayer from '$lib/YouTubePlayer.svelte';
  import { formatActivity, parseYouTube, type Snapshot, type Member } from '$lib/room';
  const roomId = (page.params.roomId ?? '').toUpperCase();
  let room: Snapshot | null = null;
  let error = '';
  let notice = '';
  let connected = false;
  let watching = false;
  let videoURL = '';
  let seekTo = 0;
  let mobileTab: 'queue' | 'people' | 'activity' = 'queue';
  let dragging: string | null = null;
  const me = () => room?.members.find((m) => m.identityId === room?.me);
  const can = (cap: string) => {
    const m = me();
    return !!m && (m.role === 'owner' || m.role === 'admin' || m.permissions[cap] !== false);
  };
  const manages = () => me()?.role === 'owner' || me()?.role === 'admin';
  onMount(async () => {
    try {
      await establish();
      room = await api(`/api/rooms/${roomId}`);
      connect();
    } catch (e) {
      error = e instanceof Error ? e.message : 'Could not join room.';
    }
  });
  function connect() {
    const ws = new WebSocket(websocketURL(`/api/rooms/${roomId}/ws`));
    ws.onopen = () => {
      connected = true;
      notice = 'Connected';
    };
    ws.onclose = () => {
      connected = false;
      notice = 'Connection lost. Reconnecting…';
      setTimeout(connect, 1500);
    };
    ws.onmessage = (event) => {
      const data = JSON.parse(event.data);
      if (data.type === 'snapshot') room = data.payload;
      else if (data.type === 'error') notice = data.message || 'The server denied that action.';
    };
  }
  async function command(type: string, payload: Record<string, unknown> = {}) {
    if (!room) return;
    notice = '';
    try {
      room = await api(`/api/rooms/${roomId}/commands`, {
        method: 'POST',
        body: JSON.stringify({
          type,
          requestId: crypto.randomUUID(),
          expectedRevision: room.playback.revision,
          payload,
        }),
      });
    } catch (e) {
      notice = e instanceof Error ? e.message : 'Action failed.';
    }
  }
  async function add(playNow = false) {
    const id = parseYouTube(videoURL);
    if (!id) {
      notice = 'Enter a valid YouTube video URL or video ID.';
      return;
    }
    await command(playNow ? 'queue.play_now' : 'queue.add', { videoId: id, title: `YouTube video ${id}` });
    videoURL = '';
  }
  function copyInvite() {
    navigator.clipboard.writeText(location.href);
    notice = 'Invite link copied.';
  }
  function drop(target: string) {
    if (!room || !dragging || dragging === target) return;
    const ids = room.queue.map((q) => q.id);
    const from = ids.indexOf(dragging),
      to = ids.indexOf(target);
    ids.splice(to, 0, ids.splice(from, 1)[0]);
    dragging = null;
    command('queue.reorder', { itemIds: ids });
  }
  async function memberAction(member: Member, action: 'kick' | 'ban' | 'role') {
    if (action === 'role')
      await command('member.role', {
        identityId: member.identityId,
        role: member.role === 'admin' ? 'member' : 'admin',
      });
    else if (confirm(`${action === 'ban' ? 'Ban' : 'Kick'} ${member.displayName}?`))
      await command(`member.${action}`, { identityId: member.identityId });
  }
</script>

<svelte:head><title>{room?.label || roomId} · KoalaParty</title></svelte:head>
{#if error}<main class="fatal panel">
    <span>🌧️</span>
    <h1>Couldn’t enter this room</h1>
    <p class="error">{error}</p>
    <a class="button" href="/">Back home</a>
  </main>{:else if !room}<main class="fatal"><p>Joining room…</p></main>{:else}
  <main class="room-shell">
    <header class="room-header">
      <div>
        <small>Room</small>
        <h1>{room.label}</h1>
        <code>{room.id}</code>
      </div>
      <div class="room-actions">
        <span class:offline={!connected} class="connection">{connected ? 'Live' : 'Reconnecting'}</span><span
          class="visibility">{room.visibility.replace('_', '-')}</span
        ><button class="secondary" onclick={copyInvite}>Copy invite</button>{#if manages()}<label
            class="visibility-select"
            ><span class="sr-only">Room visibility</span><select
              value={room.visibility}
              onchange={(e) => command('room.visibility', { visibility: e.currentTarget.value })}
              ><option value="unlisted">Unlisted</option><option value="public">Public</option><option value="private"
                >Private</option
              ><option value="friends_only">Friends only</option></select
            ></label
          >{/if}
      </div>
    </header>
    <section class="room-grid">
      <div class="main-column">
        <div class="player-wrap">
          <YouTubePlayer
            enabled={watching}
            videoId={room.playback.media?.providerId}
            status={watching ? room.playback.status : 'paused'}
            position={room.playback.position}
            onEnded={() => can('queue.skip') && command('queue.skip')}
          />{#if !watching}<button
              class="start"
              onclick={() => {
                watching = true;
                notice = 'Playback enabled';
              }}>▶ Start watching</button
            >{/if}
        </div>
        <div class="controls panel">
          <div class="transport">
            <button
              onclick={() =>
                command(room!.playback.status === 'playing' ? 'player.pause' : 'player.play', {
                  position: room!.playback.position,
                })}
              disabled={!can('playback.play_pause')}>{room.playback.status === 'playing' ? 'Pause' : 'Play'}</button
            ><label class="seek"
              ><span>Seek in seconds</span><input
                type="number"
                min="0"
                max="604800"
                bind:value={seekTo}
                disabled={!can('playback.seek')}
              /></label
            ><button
              class="secondary"
              onclick={() => command('player.seek', { position: Number(seekTo) })}
              disabled={!can('playback.seek')}>Seek</button
            ><span class="revision">Revision {room.playback.revision}</span>
          </div>
          <form
            class="add"
            onsubmit={(e) => {
              e.preventDefault();
              add(false);
            }}
          >
            <label
              ><span>YouTube URL</span><input
                bind:value={videoURL}
                maxlength="2048"
                placeholder="https://youtube.com/watch?v=…"
              /></label
            ><button disabled={!can('queue.add')}>Add to queue</button><button
              type="button"
              class="secondary"
              onclick={() => add(true)}
              disabled={!can('media.play_now')}>Play now</button
            >
          </form>
          {#if room.playback.media}<div class="now">
              <img src={room.playback.media.thumbnail} alt="" />
              <div><small>Now playing</small><b>{room.playback.media.title}</b></div>
            </div>{/if}
        </div>
      </div>
      <aside class="side-column panel">
        <div class="mobile-tabs" role="tablist">
          <button class:active={mobileTab === 'queue'} onclick={() => (mobileTab = 'queue')}
            >Queue <span>{room.queue.length}</span></button
          ><button class:active={mobileTab === 'people'} onclick={() => (mobileTab = 'people')}
            >People <span>{room.members.length}</span></button
          ><button class:active={mobileTab === 'activity'} onclick={() => (mobileTab = 'activity')}>Activity</button>
        </div>
        <section class:hidden-mobile={mobileTab !== 'queue'}>
          <header>
            <h2>Queue</h2>
            <button
              class="ghost"
              onclick={() => command('queue.skip')}
              disabled={!room.queue.length || !can('queue.skip')}>Skip next</button
            >
          </header>
          {#if !room.queue.length}<div class="empty">
              <span>🎋</span>
              <p>The queue is empty.<br />Add a YouTube link together.</p>
            </div>{:else}<ol class="queue">
              {#each room.queue as item, i}<li
                  draggable={can('queue.reorder')}
                  ondragstart={() => (dragging = item.id)}
                  ondragover={(e) => e.preventDefault()}
                  ondrop={() => drop(item.id)}
                >
                  <span class="handle" aria-hidden="true">⠿</span><img src={item.media.thumbnail} alt="" />
                  <div><small>{i + 1} · YouTube</small><b>{item.media.title}</b></div>
                  <button
                    class="ghost icon"
                    aria-label={`Remove ${item.media.title}`}
                    onclick={() => command('queue.remove', { itemId: item.id })}
                    disabled={!can('queue.remove')}>×</button
                  >
                </li>{/each}
            </ol>{/if}
        </section>
        <section class:hidden-mobile={mobileTab !== 'people'}>
          <header>
            <h2>Participants</h2>
            <span>{room.members.length}</span>
          </header>
          <ul class="members">
            {#each room.members as member}<li>
                <div class="avatar">{member.displayName.slice(0, 1).toUpperCase()}</div>
                <div>
                  <b>{member.displayName}{member.identityId === room.me ? ' (you)' : ''}</b><small>{member.role}</small>
                </div>
                {#if manages() && member.role !== 'owner' && member.identityId !== room.me}<details>
                    <summary aria-label={`Manage ${member.displayName}`}>•••</summary>
                    <div class="menu">
                      <button class="ghost" onclick={() => memberAction(member, 'role')}
                        >{member.role === 'admin' ? 'Make member' : 'Make admin'}</button
                      ><button class="ghost" onclick={() => memberAction(member, 'kick')}>Kick</button><button
                        class="danger"
                        onclick={() => memberAction(member, 'ban')}>Ban</button
                      >
                    </div>
                  </details>{/if}
              </li>{/each}
          </ul>
        </section>
        <section class="activity hidden-desktop" class:hidden-mobile={mobileTab !== 'activity'}>
          {@render Activity(room.events)}
        </section>
      </aside>
    </section>
    <section class="activity-panel panel">
      <div class="activity-tabs"><b>Activity</b><span>Chat <small>Later</small></span></div>
      {@render Activity(room.events)}
    </section>
    <div class="status" aria-live="polite">{notice}</div>
  </main>{/if}
{#snippet Activity(events: Snapshot['events'])}<div class="events">
    {#if !events.length}<p class="muted">No activity yet.</p>{/if}{#each [...events].reverse() as event}<article>
        <span class="dot"></span>
        <div>
          <p>{formatActivity(event)}</p>
          <time datetime={event.createdAt}
            >{new Date(event.createdAt + 'Z').toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}</time
          >
        </div>
      </article>{/each}
  </div>{/snippet}

<style>
  .fatal {
    max-width: 620px;
    margin: 6rem auto;
    padding: 3rem;
    text-align: center;
  }
  .fatal span {
    font-size: 3rem;
  }
  .room-shell {
    max-width: 1500px;
    margin: auto;
    padding: 1.2rem clamp(0.7rem, 2vw, 2rem) 3rem;
  }
  .room-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    margin-bottom: 1rem;
  }
  .room-header small,
  .room-header code {
    color: var(--text-muted);
  }
  .room-header h1 {
    font-size: 1.35rem;
    margin: 0.1rem 0;
  }
  .room-actions {
    display: flex;
    align-items: center;
    gap: 0.55rem;
    flex-wrap: wrap;
    justify-content: flex-end;
  }
  .connection,
  .visibility {
    font-size: 0.72rem;
    font-weight: 800;
    padding: 0.3rem 0.55rem;
    border-radius: 2rem;
    background: var(--accent-muted);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }
  .connection::before {
    content: '●';
    color: var(--success);
    margin-right: 0.3rem;
  }
  .connection.offline::before {
    color: var(--warning);
  }
  .visibility-select {
    width: 8.5rem;
  }
  .visibility-select select {
    padding: 0.55rem;
  }
  .room-grid {
    display: grid;
    grid-template-columns: minmax(0, 2.2fr) minmax(310px, 0.8fr);
    gap: 1rem;
  }
  .main-column {
    display: grid;
    gap: 1rem;
    min-width: 0;
  }
  .player-wrap {
    position: relative;
  }
  .start {
    position: absolute;
    inset: 50% auto auto 50%;
    transform: translate(-50%, -50%);
    font-size: 1.05rem;
    padding: 1rem 1.4rem;
  }
  .controls {
    padding: 1rem;
  }
  .transport,
  .add {
    display: flex;
    gap: 0.7rem;
    align-items: end;
  }
  .seek {
    grid-template-columns: auto 6rem;
    align-items: center;
  }
  .seek input {
    width: 7rem;
  }
  .revision {
    margin-left: auto;
    color: var(--text-muted);
    font-size: 0.75rem;
  }
  .add {
    border-top: 1px solid var(--border-subtle);
    margin-top: 1rem;
    padding-top: 1rem;
  }
  .add label {
    flex: 1;
  }
  .now {
    display: flex;
    gap: 0.7rem;
    align-items: center;
    margin-top: 1rem;
    padding: 0.7rem;
    background: var(--activity-background);
    border-radius: var(--radius-sm);
  }
  .now img {
    width: 75px;
    aspect-ratio: 16/9;
    object-fit: cover;
    border-radius: 5px;
  }
  .now b,
  .now small {
    display: block;
  }
  .side-column {
    overflow: hidden;
  }
  .side-column section {
    border-bottom: 1px solid var(--border-subtle);
  }
  .side-column section > header {
    padding: 0.9rem 1rem;
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .side-column h2 {
    font-size: 1rem;
    margin: 0;
  }
  .empty {
    text-align: center;
    padding: 3rem 1rem;
    color: var(--text-muted);
  }
  .empty span {
    font-size: 2rem;
  }
  .queue,
  .members {
    list-style: none;
    padding: 0;
    margin: 0;
    max-height: 315px;
    overflow: auto;
  }
  .queue li,
  .members li {
    display: flex;
    align-items: center;
    gap: 0.65rem;
    padding: 0.7rem 1rem;
    border-top: 1px solid var(--border-subtle);
  }
  .queue li:hover,
  .members li:hover {
    background: var(--surface-hover);
  }
  .queue img {
    width: 70px;
    aspect-ratio: 16/9;
    object-fit: cover;
    border-radius: 5px;
  }
  .queue li > div,
  .members li > div {
    min-width: 0;
    flex: 1;
  }
  .queue b,
  .queue small,
  .members b,
  .members small {
    display: block;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .queue small,
  .members small {
    font-size: 0.7rem;
    color: var(--text-muted);
    text-transform: capitalize;
  }
  .handle {
    color: var(--text-muted);
    cursor: grab;
  }
  .icon {
    font-size: 1.3rem;
    padding: 0.3rem;
  }
  .avatar {
    width: 2rem;
    height: 2rem !important;
    flex: 0 0 auto !important;
    border-radius: 50%;
    display: grid !important;
    place-content: center;
    background: var(--accent-muted);
    font-weight: 900;
  }
  details {
    position: relative;
  }
  summary {
    cursor: pointer;
    list-style: none;
    padding: 0.4rem;
  }
  .menu {
    position: absolute;
    right: 0;
    top: 2rem;
    width: 145px;
    background: var(--surface-elevated);
    border: 1px solid var(--border-subtle);
    padding: 0.4rem;
    border-radius: var(--radius-sm);
    z-index: 4;
    box-shadow: var(--shadow-panel);
  }
  .menu button {
    width: 100%;
    justify-content: flex-start;
  }
  .activity-panel {
    margin-top: 1rem;
    overflow: hidden;
  }
  .activity-tabs {
    display: flex;
    gap: 1.2rem;
    padding: 0.9rem 1rem;
    border-bottom: 1px solid var(--border-subtle);
  }
  .activity-tabs span {
    color: var(--text-muted);
  }
  .activity-tabs small {
    background: var(--surface-hover);
    padding: 0.15rem 0.3rem;
    border-radius: 4px;
  }
  .events {
    padding: 0.8rem 1rem;
    max-height: 260px;
    overflow: auto;
  }
  .events article {
    display: flex;
    gap: 0.8rem;
    padding: 0.45rem;
  }
  .events p {
    margin: 0;
    font-size: 0.88rem;
  }
  .events time {
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
  .status {
    position: fixed;
    bottom: 1rem;
    left: 50%;
    transform: translateX(-50%);
    background: var(--surface-elevated);
    border: 1px solid var(--border-subtle);
    padding: 0.5rem 0.8rem;
    border-radius: 2rem;
    min-height: 2rem;
    font-size: 0.8rem;
  }
  .mobile-tabs,
  .hidden-desktop {
    display: none;
  }
  @media (max-width: 850px) {
    .room-header {
      align-items: flex-start;
    }
    .room-grid {
      grid-template-columns: 1fr;
    }
    .side-column {
      min-height: 330px;
    }
    .mobile-tabs {
      display: grid;
      grid-template-columns: repeat(3, 1fr);
      padding: 0.4rem;
    }
    .mobile-tabs button {
      background: transparent;
      color: var(--text-secondary);
      padding: 0.6rem;
    }
    .mobile-tabs button.active {
      background: var(--accent-muted);
      color: var(--text-primary);
    }
    .hidden-mobile {
      display: none;
    }
    .hidden-desktop:not(.hidden-mobile) {
      display: block;
    }
    .activity-panel {
      display: none;
    }
    .queue,
    .members {
      max-height: 360px;
    }
    .add {
      display: grid;
      grid-template-columns: 1fr 1fr;
    }
    .add label {
      grid-column: 1/-1;
    }
  }
  @media (max-width: 580px) {
    .room-header {
      display: grid;
    }
    .room-actions {
      justify-content: flex-start;
    }
    .transport {
      display: grid;
      grid-template-columns: auto 1fr auto;
    }
    .seek {
      display: none;
    }
    .revision {
      margin: 0;
    }
    .room-shell {
      padding: 0.7rem;
    }
    .add {
      grid-template-columns: 1fr;
    }
    .add label {
      grid-column: auto;
    }
  }
</style>
