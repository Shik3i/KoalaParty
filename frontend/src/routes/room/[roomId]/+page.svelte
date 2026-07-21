<script lang="ts">
  import { onMount } from 'svelte';
  import { fly, scale, fade } from 'svelte/transition';
  import { flip } from 'svelte/animate';
  import { page } from '$app/state';
  import { api, establish, websocketURL, ApiError } from '$lib/api';
  import { randomUUID } from '$lib/identity';
  import { rememberRoom } from '$lib/recentRooms';
  import YouTubePlayer from '$lib/YouTubePlayer.svelte';
  import { formatActivity, parseYouTube, type Snapshot, type Member } from '$lib/room';
  import {
    LinkSimple,
    Gear,
    X,
    Play,
    Pause,
    SkipForward,
    Plus,
    ClipboardText,
    DotsSixVertical,
    DotsThreeVertical,
    CaretUp,
    CaretDown,
    CheckCircle,
    WarningCircle,
    Info,
    ArrowsOut,
    ArrowsIn,
    Pulse,
    Shuffle,
    Repeat,
    ThumbsUp,
    PictureInPicture,
  } from 'phosphor-svelte';
  const roomId = (page.params.roomId ?? '').toUpperCase();
  let room: Snapshot | null = null;
  // The playback anchor is only re-baselined when playback actually changes
  // (status, position, or media), so the extrapolated live position stays correct
  // across unrelated snapshots (a member joining, a queue edit, …).
  let playbackAnchor = { position: 0, status: 'paused', mediaId: '', at: Date.now() };
  let disposed = false;
  let socket: WebSocket | null = null;
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  let noticeTimer: ReturnType<typeof setTimeout> | null = null;
  let commandPending = false;
  let error = '';
  let joinAttempt = 0;
  let notice = '';
  let noticeKind: 'info' | 'success' | 'error' = 'info';
  let connected = false;
  let everConnected = false;
  // The embedded YouTube player loads as soon as you enter a room (a fresh room
  // always has a video cued). A small persistent disclosure replaces the old
  // click-to-consent gate.
  let watching = true;
  let theater = false;
  let miniPlayer = false;
  let diagnostics = { drift: 0, state: 'loading', correctedAt: null as number | null };
  let reactions: Array<{ id: string; emoji: string }> = [];
  let videoURL = '';
  let mobileTab: 'queue' | 'people' | 'activity' = 'queue';
  let dragging: string | null = null;
  let settingsOpen = false;
  let settingsLoading = false;
  let invites: { username: string; createdAt: string }[] = [];
  let inviteUsername = '';
  let seekTimer: ReturnType<typeof setTimeout> | null = null;
  let confirmDialog: { title: string; confirmLabel: string; danger: boolean; resolve: (ok: boolean) => void } | null =
    null;
  const me = () => room?.members.find((m) => m.identityId === room?.me);
  const can = (cap: string) => {
    const m = me();
    return !!m && (m.role === 'owner' || m.role === 'admin' || m.permissions[cap] !== false);
  };
  const manages = () => me()?.role === 'owner' || me()?.role === 'admin';
  function updateRoom(next: Snapshot) {
    if (room && next.revision < room.revision) return;
    const pb = next.playback;
    const mediaId = pb.media?.id ?? '';
    if (
      !room ||
      pb.position !== playbackAnchor.position ||
      pb.status !== playbackAnchor.status ||
      mediaId !== playbackAnchor.mediaId
    ) {
      if (mediaId !== playbackAnchor.mediaId) mediaDuration = 0;
      playbackAnchor = { position: pb.position, status: pb.status, mediaId, at: Date.now() };
    }
    room = next;
  }
  const livePosition = (now = Date.now()) =>
    playbackAnchor.status === 'playing'
      ? playbackAnchor.position + (now - playbackAnchor.at) / 1000
      : playbackAnchor.position;
  let mediaDuration = 0;
  let nowTick = Date.now();
  function fmtTime(seconds: number) {
    const s = Math.max(0, Math.floor(seconds));
    return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`;
  }
  // Names look like "🐸 Nimble Frog": use the leading emoji as the avatar badge
  // and show the rest as the label. Falls back to an initial for custom names.
  function nameParts(displayName: string): { badge: string; label: string } {
    const match = displayName.match(/^(\p{Extended_Pictographic}️?)\s+(.+)$/u);
    if (match) return { badge: match[1], label: match[2] };
    return { badge: (displayName.trim()[0] ?? '?').toUpperCase(), label: displayName };
  }
  function showNotice(message: string, clearAfter = 0, kind: 'info' | 'success' | 'error' = 'info') {
    if (noticeTimer) clearTimeout(noticeTimer);
    notice = message;
    noticeKind = kind;
    if (clearAfter > 0) noticeTimer = setTimeout(() => (notice = ''), clearAfter);
  }
  function ask(title: string, confirmLabel: string, danger = false): Promise<boolean> {
    return new Promise((resolve) => {
      confirmDialog = { title, confirmLabel, danger, resolve };
    });
  }
  function resolveConfirm(ok: boolean) {
    confirmDialog?.resolve(ok);
    confirmDialog = null;
  }
  function autofocus(node: HTMLElement) {
    node.focus();
  }
  // The member menu lives inside scrollable, clipped panels. Position it as a
  // fixed popover anchored to its trigger so it is never clipped or hidden behind
  // neighbouring content.
  function anchoredMenu(details: HTMLDetailsElement) {
    const menu = details.querySelector<HTMLElement>('.menu');
    const reposition = () => {
      if (!menu || !details.open) return;
      const rect = details.getBoundingClientRect();
      menu.style.top = `${rect.bottom + 4}px`;
      menu.style.right = `${window.innerWidth - rect.right}px`;
    };
    details.addEventListener('toggle', reposition);
    window.addEventListener('scroll', reposition, true);
    window.addEventListener('resize', reposition);
    return {
      destroy() {
        details.removeEventListener('toggle', reposition);
        window.removeEventListener('scroll', reposition, true);
        window.removeEventListener('resize', reposition);
      },
    };
  }
  function scheduleSeek(position: number) {
    if (seekTimer) clearTimeout(seekTimer);
    seekTimer = setTimeout(() => command('player.seek', { position }), 300);
  }
  function announceCreation() {
    try {
      const raw = sessionStorage.getItem('koalaparty.created');
      if (!raw) return;
      const info = JSON.parse(raw) as { id?: string; copied?: boolean };
      if (info.id !== roomId) return;
      sessionStorage.removeItem('koalaparty.created');
      showNotice(
        info.copied
          ? 'Room created — invite link copied. Share it to invite people!'
          : 'Room created — use “Copy invite” to share it.',
        4500,
        'success',
      );
    } catch {
      /* sessionStorage unavailable */
    }
  }
  // Entering a room retries on transient failures instead of dead-ending. A brief
  // 5xx / Bad Gateway (server restarting, proxy blip) or a network error is
  // retried with backoff; only a genuine client error (4xx: forbidden, not
  // found, banned) stops and shows the fatal screen.
  async function joinWithRetry() {
    while (!disposed) {
      try {
        await establish();
        if (disposed) return;
        const joinedRoom = await api<Snapshot>(`/api/rooms/${roomId}`);
        updateRoom(joinedRoom);
        rememberRoom({
          id: joinedRoom.id,
          label: joinedRoom.label,
          title: joinedRoom.playback.media?.title ?? '',
        });
        if (disposed) return;
        error = '';
        joinAttempt = 0;
        connect();
        announceCreation();
        return;
      } catch (e) {
        if (disposed) return;
        const status = e instanceof ApiError ? e.status : undefined;
        if (status !== undefined && status >= 400 && status < 500) {
          error = e instanceof Error ? e.message : 'Could not join room.';
          return;
        }
        joinAttempt++;
        await new Promise((resolve) => setTimeout(resolve, Math.min(1500 * joinAttempt, 6000)));
      }
    }
  }
  onMount(() => {
    void joinWithRetry();
    const progressTimer = setInterval(() => (nowTick = Date.now()), 500);
    return () => {
      disposed = true;
      if (reconnectTimer) clearTimeout(reconnectTimer);
      if (noticeTimer) clearTimeout(noticeTimer);
      if (seekTimer) clearTimeout(seekTimer);
      clearInterval(progressTimer);
      const activeSocket = socket;
      socket = null;
      activeSocket?.close();
    };
  });
  function connect() {
    if (disposed || socket) return;
    const ws = new WebSocket(websocketURL(`/api/rooms/${roomId}/ws`));
    socket = ws;
    ws.onopen = () => {
      if (socket !== ws) return;
      connected = true;
      if (everConnected) showNotice('Reconnected', 1800, 'success');
      everConnected = true;
    };
    ws.onclose = () => {
      if (socket !== ws) return;
      socket = null;
      connected = false;
      if (disposed) return;
      showNotice('Connection lost. Reconnecting…', 0, 'error');
      reconnectTimer = setTimeout(() => {
        reconnectTimer = null;
        connect();
      }, 1500);
    };
    ws.onmessage = (event) => {
      if (socket !== ws) return;
      try {
        const data = JSON.parse(event.data);
        if (data.type === 'snapshot') updateRoom(data.payload);
        else if (data.type === 'reaction') {
          const reaction = { id: randomUUID(), emoji: String(data.emoji) };
          reactions = [...reactions, reaction];
          setTimeout(() => (reactions = reactions.filter((item) => item.id !== reaction.id)), 2600);
        } else if (data.type === 'error') showNotice(data.message || 'The server denied that action.', 0, 'error');
      } catch {
        showNotice('Received an invalid room update. Reconnecting…', 0, 'error');
        ws.close();
      }
    };
  }
  function react(emoji: string) {
    if (!socket || socket.readyState !== WebSocket.OPEN) return;
    socket.send(JSON.stringify({ type: 'reaction.send', requestId: randomUUID(), payload: { emoji } }));
  }
  async function command(type: string, payload: Record<string, unknown> = {}) {
    if (!room || commandPending) return;
    commandPending = true;
    showNotice('');
    try {
      updateRoom(
        await api(`/api/rooms/${roomId}/commands`, {
          method: 'POST',
          body: JSON.stringify({
            type,
            requestId: randomUUID(),
            expectedRevision: room.revision,
            payload,
          }),
        }),
      );
    } catch (e) {
      showNotice(e instanceof Error ? e.message : 'Action failed.', 0, 'error');
    } finally {
      commandPending = false;
    }
  }
  async function add(playNow = false) {
    const id = parseYouTube(videoURL);
    if (!id) {
      showNotice('Enter a valid YouTube video URL or video ID.', 3000, 'error');
      return;
    }
    await command(playNow ? 'queue.play_now' : 'queue.add', { videoId: id, title: `YouTube video ${id}` });
    videoURL = '';
  }
  async function pasteFromClipboard() {
    try {
      const text = await navigator.clipboard.readText();
      videoURL = text.trim();
      showNotice('Pasted from clipboard!', 1200, 'success');
    } catch {
      showNotice('Clipboard permission denied or unavailable.', 2000, 'error');
    }
  }
  async function quickAdd(id: string) {
    videoURL = id;
    await add(false);
  }
  async function copyInvite() {
    try {
      await navigator.clipboard.writeText(location.href);
      showNotice('Invite link copied.', 2200, 'success');
    } catch {
      showNotice('Could not copy the invite link. Copy it from the address bar.', 0, 'error');
    }
  }
  function drop(target: string) {
    if (!room || !dragging || dragging === target) return;
    const ids = room.queue.map((q) => q.id);
    const from = ids.indexOf(dragging),
      to = ids.indexOf(target);
    if (from < 0 || to < 0) {
      dragging = null;
      return;
    }
    ids.splice(to, 0, ids.splice(from, 1)[0]);
    dragging = null;
    command('queue.reorder', { itemIds: ids });
  }
  function move(itemId: string, delta: number) {
    if (!room) return;
    const ids = room.queue.map((q) => q.id);
    const from = ids.indexOf(itemId);
    const to = from + delta;
    if (from < 0 || to < 0 || to >= ids.length) return;
    ids.splice(to, 0, ids.splice(from, 1)[0]);
    command('queue.reorder', { itemIds: ids });
  }
  async function memberAction(member: Member, action: 'kick' | 'ban' | 'role') {
    if (action === 'role')
      await command('member.role', {
        identityId: member.identityId,
        role: member.role === 'admin' ? 'member' : 'admin',
      });
    else if (
      await ask(`${action === 'ban' ? 'Ban' : 'Kick'} ${member.displayName}?`, action === 'ban' ? 'Ban' : 'Kick', true)
    )
      await command(`member.${action}`, { identityId: member.identityId });
  }
  async function loadInvites() {
    if (!manages() || settingsLoading) return;
    settingsLoading = true;
    try {
      invites = await api(`/api/rooms/${roomId}/invites`);
    } catch (e) {
      showNotice(e instanceof Error ? e.message : 'Could not load invitations.', 0, 'error');
    } finally {
      settingsLoading = false;
    }
  }
  async function addInvite() {
    if (!inviteUsername.trim()) return;
    try {
      await api(`/api/rooms/${roomId}/invites`, {
        method: 'POST',
        body: JSON.stringify({ username: inviteUsername.trim() }),
      });
      inviteUsername = '';
      await loadInvites();
      showNotice('Invitation added.', 2200, 'success');
    } catch (e) {
      showNotice(e instanceof Error ? e.message : 'Could not add invitation.', 0, 'error');
    }
  }
  async function revokeInvite(username: string) {
    try {
      await api(`/api/rooms/${roomId}/invites/${encodeURIComponent(username)}`, { method: 'DELETE' });
      invites = invites.filter((invite) => invite.username !== username);
      showNotice('Invitation revoked.', 2200, 'success');
    } catch (e) {
      showNotice(e instanceof Error ? e.message : 'Could not revoke invitation.', 0, 'error');
    }
  }
  async function leaveOrDelete() {
    const owner = me()?.role === 'owner';
    if (
      !(await ask(
        owner ? 'Delete this room permanently for everyone?' : 'Leave this room?',
        owner ? 'Delete room' : 'Leave room',
        true,
      ))
    )
      return;
    try {
      await api(`/api/rooms/${roomId}${owner ? '' : '/membership'}`, { method: 'DELETE' });
      location.href = '/rooms';
    } catch (e) {
      showNotice(e instanceof Error ? e.message : 'Room action failed.', 0, 'error');
    }
  }
  async function transfer(member: Member) {
    if (!(await ask(`Transfer ownership to ${member.displayName}? You will become an admin.`, 'Transfer'))) return;
    await command('room.transfer', { identityId: member.identityId });
  }
</script>

<svelte:head><title>{room?.label || roomId} · KoalaParty</title></svelte:head>
<svelte:window onkeydown={(e) => confirmDialog && e.key === 'Escape' && resolveConfirm(false)} />
{#if error}<main class="fatal panel">
    <span>🌧️</span>
    <h1>Couldn’t enter this room</h1>
    <p class="error">{error}</p>
    <a class="button" href="/">Back home</a>
  </main>{:else if !room}<main class="fatal loading" aria-busy="true">
    <div class="spinner" aria-hidden="true"></div>
    <p>{joinAttempt > 0 ? 'Reconnecting to the room…' : 'Joining room…'}</p>
    {#if joinAttempt > 1}<small class="muted">The server may be restarting — this will retry automatically.</small>{/if}
  </main>{:else}
  <main class="room-shell">
    <header class="room-header">
      <div>
        <small>Room</small>
        <h1>{room.label}</h1>
        <code>{room.id}</code>
      </div>
      <div class="room-actions">
        <span class:offline={!connected} class="connection" role="status">{connected ? 'Live' : 'Reconnecting'}</span
        ><span class="visibility">{room.visibility.replace('_', '-')}</span><button
          class="secondary"
          onclick={copyInvite}><LinkSimple size={16} weight="bold" />Copy invite</button
        ><button
          class="secondary"
          onclick={() => {
            settingsOpen = !settingsOpen;
            if (settingsOpen) loadInvites();
          }}
          >{#if settingsOpen}<X size={16} weight="bold" />Close settings{:else}<Gear size={16} weight="bold" />Room
            settings{/if}</button
        >
      </div>
    </header>
    {#if settingsOpen}<section class="settings panel" aria-label="Room settings">
        <div class="settings-grid">
          <div>
            <h2>Access</h2>
            <p class="muted">Choose who can enter this room. Invite lists apply to private rooms.</p>
            {#if manages()}<label
                >Visibility<select
                  value={room.visibility}
                  disabled={commandPending}
                  onchange={(e) => command('room.visibility', { visibility: e.currentTarget.value })}
                >
                  <option value="unlisted">Unlisted</option>{#if room.publicRoomsEnabled}<option value="public"
                      >Public</option
                    >{/if}<option value="private">Private</option><option value="friends_only">Friends only</option>
                </select></label
              >{/if}
          </div>
          {#if manages()}<div>
              <h2>Private invitations</h2>
              <form
                class="invite-form"
                onsubmit={(e) => {
                  e.preventDefault();
                  addInvite();
                }}
              >
                <label
                  >Account username<input
                    bind:value={inviteUsername}
                    pattern="[A-Za-z0-9_]+"
                    minlength="3"
                    maxlength="24"
                  /></label
                ><button disabled={settingsLoading}>Invite</button>
              </form>
              {#if settingsLoading}<p class="muted">Loading invitations…</p>{:else if !invites.length}<p class="muted">
                  No private invitations.
                </p>{:else}<ul class="invite-list">
                  {#each invites as invite}<li>
                      <span>{invite.username}</span><button class="ghost" onclick={() => revokeInvite(invite.username)}
                        >Revoke</button
                      >
                    </li>{/each}
                </ul>{/if}
            </div>{/if}
          {#if me()?.role === 'owner'}<div>
              <h2>Transfer ownership</h2>
              <p class="muted">Only account-linked members can become the permanent owner.</p>
              <ul class="transfer-list">
                {#each room.members.filter((member) => member.identityId !== room!.me && member.accountLinked) as member}<li
                  >
                    <span>{member.displayName}</span><button class="secondary" onclick={() => transfer(member)}
                      >Transfer</button
                    >
                  </li>{/each}
              </ul>
              {#if !room.members.some((member) => member.identityId !== room!.me && member.accountLinked)}<p
                  class="muted"
                >
                  No eligible member is currently in the room.
                </p>{/if}
            </div>{/if}
          <div class="danger-settings">
            <h2>{me()?.role === 'owner' ? 'Delete room' : 'Leave room'}</h2>
            <p class="muted">
              {me()?.role === 'owner'
                ? 'Permanently closes the room for every participant.'
                : 'Removes this room from your account.'}
            </p>
            <button class="danger" onclick={leaveOrDelete}
              >{me()?.role === 'owner' ? 'Delete room' : 'Leave room'}</button
            >
          </div>
        </div>
      </section>{/if}
    <section class="room-grid" class:theater>
      <div class="main-column">
        <div class="player-wrap" class:mini-player={miniPlayer}>
          <YouTubePlayer
            enabled={watching}
            videoId={room.playback.media?.providerId}
            status={room.playback.status}
            position={playbackAnchor.position}
            positionAt={playbackAnchor.at}
            canControl={can('playback.play_pause')}
            canSeek={can('playback.seek')}
            hasQueue={room.queue.length > 0}
            onPlay={(pos) => command('player.play', { position: pos })}
            onPause={(pos) => command('player.pause', { position: pos })}
            onSeek={scheduleSeek}
            onEnded={() => can('queue.skip') && command('queue.skip')}
            onSkip={can('queue.skip') ? () => command('queue.skip') : undefined}
            onDuration={(d) => (mediaDuration = d)}
            onDiagnostics={(value) => (diagnostics = value)}
          />{#if !room.playback.media && room.queue.length && can('queue.skip')}<button
              class="start"
              onclick={() => command('queue.skip')}
              disabled={commandPending}><Play size={18} weight="fill" />Play from queue</button
            >{/if}
        </div>
        <div class="player-bar">
          <div class="scrubber-area">
            {#if room.playback.media}{@const pos = livePosition(nowTick)}{@const pct =
                mediaDuration > 0 ? Math.min(100, (pos / mediaDuration) * 100) : 0}
              <div
                class="scrubber"
                role="progressbar"
                aria-label="Playback progress"
                aria-valuemin="0"
                aria-valuemax={Math.round(mediaDuration)}
                aria-valuenow={Math.round(pos)}
                aria-valuetext={mediaDuration > 0 ? `${fmtTime(pos)} of ${fmtTime(mediaDuration)}` : fmtTime(pos)}
              >
                <div class="scrubber-track"><div class="scrubber-fill" style="width:{pct}%"></div></div>
                <div class="scrubber-time">
                  <span>{fmtTime(pos)}</span><span>{mediaDuration > 0 ? fmtTime(mediaDuration) : '–:--'}</span>
                </div>
              </div>{/if}
          </div>
          <button
            class="secondary theater-toggle"
            aria-pressed={theater}
            aria-label={theater ? 'Exit theater mode' : 'Theater mode'}
            title={theater ? 'Exit theater mode' : 'Theater mode'}
            onclick={() => (theater = !theater)}
            >{#if theater}<ArrowsIn size={17} weight="bold" />{:else}<ArrowsOut size={17} weight="bold" />{/if}<span
              class="theater-label">{theater ? 'Exit theater' : 'Theater'}</span
            ></button
          ><button
            class="secondary theater-toggle"
            aria-pressed={miniPlayer}
            aria-label={miniPlayer ? 'Dock player' : 'Float mini-player'}
            title={miniPlayer ? 'Dock player' : 'Float mini-player'}
            onclick={() => (miniPlayer = !miniPlayer)}
            ><PictureInPicture size={17} weight="bold" /><span class="theater-label"
              >{miniPlayer ? 'Dock player' : 'Mini-player'}</span
            ></button
          >
        </div>
        <div class="sync-diagnostics" title="Estimated difference between this player and the shared room clock">
          <Pulse size={14} weight="bold" /><span
            >{!connected
              ? 'Offline'
              : diagnostics.state === 'buffering'
                ? 'Buffering'
                : Math.abs(diagnostics.drift) < 0.6
                  ? 'Perfectly synced'
                  : `${Math.abs(diagnostics.drift).toFixed(1)}s ${diagnostics.drift < 0 ? 'behind' : 'ahead'}`}</span
          >{#if diagnostics.correctedAt}<small
              >last corrected {Math.max(0, Math.round((nowTick - diagnostics.correctedAt) / 1000))}s ago</small
            >{/if}
        </div>
        <div class="reaction-bar" aria-label="Send a reaction">
          {#each ['❤️', '😂', '🔥', '👀', '😴', '👏'] as emoji}<button class="ghost" onclick={() => react(emoji)}
              >{emoji}</button
            >{/each}
        </div>
        <div class="reaction-overlay" aria-live="polite">
          {#each reactions as reaction (reaction.id)}<span transition:fly={{ y: 30, duration: 300 }}
              >{reaction.emoji}</span
            >{/each}
        </div>
        <p class="player-note">
          Opening a room loads YouTube's privacy-enhanced player from youtube-nocookie.com. <a href="/privacy"
            >Privacy details</a
          >
        </p>
        <div class="controls panel">
          <div class="transport">
            <button
              class="play-toggle"
              onclick={() =>
                command(room!.playback.status === 'playing' ? 'player.pause' : 'player.play', {
                  position: livePosition(),
                })}
              disabled={commandPending || !can('playback.play_pause')}
              >{#if room.playback.status === 'playing'}<Pause size={18} weight="fill" />Pause{:else}<Play
                  size={18}
                  weight="fill"
                />Play{/if}</button
            ><span class="transport-hint"
              >Play, pause and scrub with the video’s own controls — everyone stays in sync.</span
            >
          </div>
          <form
            class="add"
            onsubmit={(e) => {
              e.preventDefault();
              add(false);
            }}
          >
            <label
              ><span>YouTube URL</span>
              <div class="input-container">
                <input bind:value={videoURL} maxlength="2048" placeholder="https://youtube.com/watch?v=…" />
                <button
                  type="button"
                  class="ghost paste-btn"
                  onclick={pasteFromClipboard}
                  aria-label="Paste from clipboard"
                  title="Paste from clipboard"><ClipboardText size={18} weight="bold" /></button
                >
              </div>
            </label><button disabled={commandPending || !can('queue.add')}
              ><Plus size={16} weight="bold" />Add to queue</button
            ><button
              type="button"
              class="secondary"
              onclick={() => add(true)}
              disabled={commandPending || !can('media.play_now')}><Play size={16} weight="fill" />Play now</button
            >
          </form>
          <div class="presets">
            <span class="presets-label">Quick Add:</span>
            <button
              type="button"
              class="ghost preset-btn"
              onclick={() => quickAdd('dQw4w9WgXcQ')}
              disabled={commandPending || !can('queue.add')}>🍿 Rickroll</button
            >
            <button
              type="button"
              class="ghost preset-btn"
              onclick={() => quickAdd('jfKfPfyJRdk')}
              disabled={commandPending || !can('queue.add')}>🎵 Lofi Girl</button
            >
            <button
              type="button"
              class="ghost preset-btn"
              onclick={() => quickAdd('4xDzrJKXOOY')}
              disabled={commandPending || !can('queue.add')}>🌊 Synthwave</button
            >
            <button
              type="button"
              class="ghost preset-btn"
              onclick={() => quickAdd('aqz-KE-bpKQ')}
              disabled={commandPending || !can('queue.add')}>🐰 Bunny</button
            >
          </div>
          {#if room.playback.media}<div class="now">
              {#if watching}<img src={room.playback.media.thumbnail} alt="" />{:else}<span
                  class="thumbnail-placeholder"
                  aria-hidden="true">▶</span
                >{/if}
              <div><small>Now playing</small><b>{room.playback.media.title}</b></div>
            </div>{/if}
        </div>
      </div>
      <aside class="side-column panel">
        <div class="mobile-tabs" role="tablist">
          <button
            role="tab"
            aria-selected={mobileTab === 'queue'}
            class:active={mobileTab === 'queue'}
            onclick={() => (mobileTab = 'queue')}>Queue <span>{room.queue.length}</span></button
          ><button
            role="tab"
            aria-selected={mobileTab === 'people'}
            class:active={mobileTab === 'people'}
            onclick={() => (mobileTab = 'people')}>People <span>{room.members.length}</span></button
          ><button
            role="tab"
            aria-selected={mobileTab === 'activity'}
            class:active={mobileTab === 'activity'}
            onclick={() => (mobileTab = 'activity')}>Activity</button
          >
        </div>
        <section class:hidden-mobile={mobileTab !== 'queue'}>
          <header>
            <h2>Queue</h2>
            <div class="queue-tools">
              <button
                class="ghost"
                title="Shuffle queue"
                aria-label="Shuffle queue"
                onclick={() => command('queue.shuffle')}
                disabled={commandPending || room.queue.length < 2 || !can('queue.reorder')}
                ><Shuffle size={15} weight="bold" /></button
              ><button
                class="ghost"
                class:active={room.queueLoop}
                title="Loop played videos"
                aria-label="Loop queue"
                aria-pressed={room.queueLoop}
                onclick={() => command('queue.loop', { enabled: !room!.queueLoop })}
                disabled={commandPending || !can('queue.reorder')}><Repeat size={15} weight="bold" /></button
              ><button
                class="ghost"
                onclick={() => command('queue.skip')}
                disabled={commandPending || !room.queue.length || !can('queue.skip')}
                ><SkipForward size={15} weight="fill" />Skip next</button
              >
            </div>
          </header>
          {#if !room.queue.length}<div class="empty">
              <span>🎋</span>
              <p>The queue is empty.<br />Add a YouTube link together.</p>
            </div>{:else}<ol class="queue">
              {#each room.queue as item, i (item.id)}<li
                  animate:flip={{ duration: 260 }}
                  draggable={!commandPending && can('queue.reorder')}
                  ondragstart={() => (dragging = item.id)}
                  ondragover={(e) => e.preventDefault()}
                  ondrop={() => drop(item.id)}
                >
                  <span class="handle" aria-hidden="true"><DotsSixVertical size={16} weight="bold" /></span
                  >{#if watching}<img src={item.media.thumbnail} alt="" />{:else}<span
                      class="thumbnail-placeholder"
                      aria-hidden="true"><Play size={16} weight="fill" /></span
                    >{/if}
                  <div><small>{i + 1} · YouTube</small><b>{item.media.title}</b></div>
                  <button
                    class="ghost vote"
                    class:active={item.voted}
                    aria-label={`Vote for ${item.media.title}`}
                    onclick={() => command('queue.vote', { itemId: item.id })}
                    disabled={commandPending || !can('queue.vote')}
                    ><ThumbsUp size={14} weight={item.voted ? 'fill' : 'bold'} />{item.votes}</button
                  >
                  {#if can('queue.reorder')}<div class="reorder">
                      <button
                        class="ghost"
                        aria-label={`Move ${item.media.title} up`}
                        onclick={() => move(item.id, -1)}
                        disabled={commandPending || i === 0}><CaretUp size={14} weight="bold" /></button
                      ><button
                        class="ghost"
                        aria-label={`Move ${item.media.title} down`}
                        onclick={() => move(item.id, 1)}
                        disabled={commandPending || i === room.queue.length - 1}
                        ><CaretDown size={14} weight="bold" /></button
                      >
                    </div>{/if}<button
                    class="ghost icon"
                    aria-label={`Remove ${item.media.title}`}
                    onclick={() => command('queue.remove', { itemId: item.id })}
                    disabled={commandPending || !can('queue.remove')}><X size={16} weight="bold" /></button
                  >
                </li>{/each}
            </ol>{/if}
          {#if room.history.length}<details class="history">
              <summary>Recently played ({room.history.length})</summary>
              <ul>
                {#each room.history as item}<li><span>{item.title}</span></li>{/each}
              </ul>
            </details>{/if}
        </section>
        <section class:hidden-mobile={mobileTab !== 'people'}>
          <header>
            <h2>Participants</h2>
            <span class="online-count"
              ><span class="online-dot"></span>{room.members.filter((m) => m.active).length} online</span
            >
          </header>
          <ul class="members">
            {#each room.members as member}{@const parts = nameParts(member.displayName)}
              <li>
                <div class="avatar" class:offline={!member.active}>
                  <span aria-hidden="true">{parts.badge}</span><span
                    class="presence"
                    title={member.active ? 'Online' : 'Offline'}
                  ></span>
                </div>
                <div>
                  <b>{parts.label}{member.identityId === room.me ? ' (you)' : ''}</b><small>{member.role}</small>
                </div>
                {#if manages() && member.role !== 'owner' && member.identityId !== room.me}<details use:anchoredMenu>
                    <summary aria-label={`Manage ${member.displayName}`}
                      ><DotsThreeVertical size={18} weight="bold" /></summary
                    >
                    <div class="menu">
                      <button class="ghost" disabled={commandPending} onclick={() => memberAction(member, 'role')}
                        >{member.role === 'admin' ? 'Make member' : 'Make admin'}</button
                      ><button class="ghost" disabled={commandPending} onclick={() => memberAction(member, 'kick')}
                        >Kick</button
                      ><button class="danger" disabled={commandPending} onclick={() => memberAction(member, 'ban')}
                        >Ban</button
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
      <div class="activity-tabs"><Pulse size={16} weight="bold" /><b>Activity</b></div>
      {@render Activity(room.events)}
    </section>
    {#if notice}<div
        class="status status--{noticeKind}"
        role="status"
        aria-live="polite"
        transition:fly={{ y: 12, duration: 220 }}
      >
        {#if noticeKind === 'success'}<CheckCircle
            size={17}
            weight="fill"
          />{:else if noticeKind === 'error'}<WarningCircle size={17} weight="fill" />{:else}<Info
            size={17}
            weight="fill"
          />{/if}<span>{notice}</span>
      </div>{/if}
    {#if confirmDialog}<div class="modal-backdrop">
        <button
          class="modal-scrim"
          aria-label="Cancel"
          onclick={() => resolveConfirm(false)}
          transition:fade={{ duration: 160 }}
        ></button>
        <div
          class="modal panel"
          role="alertdialog"
          aria-modal="true"
          aria-label={confirmDialog.title}
          transition:scale={{ start: 0.94, duration: 180 }}
        >
          <p>{confirmDialog.title}</p>
          <div class="modal-actions">
            <button class="secondary" onclick={() => resolveConfirm(false)}>Cancel</button><button
              class={confirmDialog.danger ? 'danger' : ''}
              onclick={() => resolveConfirm(true)}
              use:autofocus>{confirmDialog.confirmLabel}</button
            >
          </div>
        </div>
      </div>{/if}
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
  .mini-player {
    position: fixed;
    right: 1rem;
    bottom: 1rem;
    width: min(420px, calc(100vw - 2rem));
    z-index: 20;
    box-shadow: 0 20px 60px rgba(0, 0, 0, 0.5);
  }
  .sync-diagnostics,
  .reaction-bar {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    color: var(--text-muted);
    font-size: 0.78rem;
    margin-top: 0.55rem;
  }
  .sync-diagnostics small {
    margin-left: auto;
  }
  .reaction-bar button {
    font-size: 1.1rem;
    padding: 0.35rem 0.5rem;
  }
  .reaction-overlay {
    position: fixed;
    right: clamp(1rem, 8vw, 8rem);
    bottom: 18%;
    z-index: 30;
    display: grid;
    pointer-events: none;
  }
  .reaction-overlay span {
    font-size: 2.3rem;
    filter: drop-shadow(0 5px 10px rgba(0, 0, 0, 0.4));
  }
  .queue-tools {
    display: flex;
    align-items: center;
    gap: 0.2rem;
  }
  .queue-tools .active,
  .vote.active {
    color: var(--accent-primary);
    background: var(--surface-hover);
  }
  .vote {
    display: inline-flex;
    gap: 0.25rem;
  }
  .history {
    margin-top: 1rem;
    color: var(--text-muted);
    font-size: 0.82rem;
  }
  .history ul {
    padding-left: 1.2rem;
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
  .room-grid {
    display: grid;
    grid-template-columns: minmax(0, 2.2fr) minmax(310px, 0.8fr);
    gap: 1rem;
  }
  .room-grid.theater {
    grid-template-columns: 1fr;
  }
  .room-grid.theater .main-column {
    width: 100%;
    max-width: min(100%, 142vh);
    margin-inline: auto;
  }
  /* The theater toggle sits directly under the player, sharing a row with the
     seek bar, so it is easy to reach instead of buried in the controls panel. */
  .player-bar {
    display: flex;
    align-items: center;
    gap: 0.9rem;
    margin-top: 0.6rem;
  }
  .scrubber-area {
    flex: 1;
    min-width: 0;
  }
  .theater-toggle {
    flex: 0 0 auto;
    padding: 0.5rem 0.8rem;
  }
  @media (max-width: 580px) {
    .theater-label {
      display: none;
    }
    .theater-toggle {
      padding: 0.5rem 0.6rem;
    }
  }
  .settings {
    padding: 1rem;
    margin-bottom: 1rem;
  }
  .settings-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 1rem;
  }
  .settings-grid > div {
    padding: 1rem;
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-sm);
  }
  .settings h2 {
    margin-top: 0;
    font-size: 1rem;
  }
  .invite-form {
    display: grid;
    grid-template-columns: 1fr auto;
    align-items: end;
    gap: 0.6rem;
  }
  .invite-list,
  .transfer-list {
    list-style: none;
    padding: 0;
    margin: 0.8rem 0 0;
  }
  .invite-list li,
  .transfer-list li {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 0.5rem;
    padding: 0.45rem 0;
    border-top: 1px solid var(--border-subtle);
  }
  .danger-settings {
    border-color: color-mix(in srgb, var(--danger) 45%, var(--border-subtle)) !important;
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
  /* Keep the centering transform on hover/active so the global lift does not
     yank the absolutely-positioned button out of place. */
  .start:hover {
    transform: translate(-50%, calc(-50% - 1px));
  }
  .start:active {
    transform: translate(-50%, -50%);
  }
  .player-note {
    margin: 0.55rem 0 0;
    color: var(--text-muted);
    font-size: 0.74rem;
    line-height: 1.4;
  }
  .scrubber-track {
    height: 6px;
    border-radius: 999px;
    background: var(--surface-hover);
    overflow: hidden;
  }
  .scrubber-fill {
    height: 100%;
    border-radius: 999px;
    background: linear-gradient(90deg, var(--accent-primary), var(--accent-hover));
    transition: width 0.5s linear;
  }
  .scrubber-time {
    display: flex;
    justify-content: space-between;
    margin-top: 0.35rem;
    font-size: 0.72rem;
    color: var(--text-muted);
    font-variant-numeric: tabular-nums;
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
  .transport {
    align-items: center;
  }
  .play-toggle {
    min-width: 6.5rem;
  }
  .transport-hint {
    color: var(--text-muted);
    font-size: 0.82rem;
    line-height: 1.4;
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
  .now img,
  .now .thumbnail-placeholder {
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
  .queue img,
  .queue .thumbnail-placeholder {
    width: 70px;
    aspect-ratio: 16/9;
    object-fit: cover;
    border-radius: 5px;
  }
  .thumbnail-placeholder {
    flex: 0 0 auto;
    display: grid;
    place-content: center;
    color: var(--text-muted);
    background: var(--player-background);
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
    position: relative;
    width: 2rem;
    height: 2rem !important;
    flex: 0 0 auto !important;
    border-radius: 50%;
    display: grid !important;
    place-content: center;
    background: var(--accent-muted);
    font-weight: 900;
    font-size: 1.05rem;
    line-height: 1;
  }
  .avatar .presence {
    position: absolute;
    bottom: -1px;
    right: -1px;
    width: 0.62rem;
    height: 0.62rem;
    border-radius: 50%;
    background: var(--success);
    border: 2px solid var(--surface-panel);
  }
  .avatar.offline .presence {
    background: var(--text-muted);
  }
  .avatar.offline {
    opacity: 0.6;
  }
  .online-count {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    font-size: 0.8rem;
    font-weight: 650;
    color: var(--text-secondary);
  }
  .online-dot {
    width: 0.5rem;
    height: 0.5rem;
    border-radius: 50%;
    background: var(--success);
    box-shadow: 0 0 0 3px color-mix(in srgb, var(--success) 25%, transparent);
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
    position: fixed;
    right: 0;
    top: 2rem;
    width: 145px;
    background: var(--surface-elevated);
    border: 1px solid var(--border-subtle);
    padding: 0.4rem;
    border-radius: var(--radius-sm);
    z-index: 60;
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
    align-items: center;
    gap: 0.5rem;
    padding: 0.9rem 1rem;
    border-bottom: 1px solid var(--border-subtle);
    color: var(--accent-primary);
  }
  .activity-tabs b {
    color: var(--text-primary);
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
    padding: 0.5rem 0.9rem;
    border-radius: 2rem;
    min-height: 2rem;
    font-size: 0.8rem;
    font-weight: 650;
    box-shadow: var(--shadow-panel);
    max-width: min(92vw, 30rem);
    z-index: 20;
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .status :global(svg) {
    flex: 0 0 auto;
  }
  .status--success {
    border-color: color-mix(in srgb, var(--success) 55%, var(--border-subtle));
    color: var(--success);
  }
  .status--error {
    border-color: color-mix(in srgb, var(--danger) 55%, var(--border-subtle));
    color: var(--danger);
  }
  .spinner {
    width: 2.2rem;
    height: 2.2rem;
    margin: 0 auto 1rem;
    border: 3px solid var(--border-subtle);
    border-top-color: var(--accent-primary);
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
  .reorder {
    display: flex;
    flex-direction: column;
    flex: 0 0 auto;
  }
  .reorder button {
    font-size: 0.7rem;
    padding: 0.1rem 0.35rem;
    line-height: 1.1;
  }
  .modal-backdrop {
    position: fixed;
    inset: 0;
    display: grid;
    place-items: center;
    padding: 1rem;
    z-index: 50;
  }
  .modal-scrim {
    position: fixed;
    inset: 0;
    border: 0;
    border-radius: 0;
    background: rgba(5, 8, 6, 0.55);
    cursor: default;
  }
  .modal-scrim:hover {
    background: rgba(5, 8, 6, 0.55);
  }
  .modal {
    position: relative;
    z-index: 1;
    width: 100%;
    max-width: 26rem;
    padding: 1.4rem;
    display: grid;
    gap: 1.2rem;
  }
  .modal p {
    margin: 0;
    font-size: 1.02rem;
    line-height: 1.5;
  }
  .modal-actions {
    display: flex;
    justify-content: flex-end;
    gap: 0.6rem;
  }
  .mobile-tabs,
  .hidden-desktop {
    display: none;
  }
  @media (max-width: 850px) {
    .settings-grid {
      grid-template-columns: 1fr;
    }
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
  .input-container {
    display: flex;
    align-items: center;
    position: relative;
    width: 100%;
  }
  .input-container input {
    padding-right: 2.5rem;
    width: 100%;
  }
  .paste-btn {
    position: absolute;
    right: 0.2rem;
    background: none !important;
    border: none !important;
    font-size: 1.1rem;
    cursor: pointer;
    padding: 0.4rem;
    opacity: 0.7;
    transition: opacity 0.2s;
  }
  .paste-btn:hover {
    opacity: 1;
  }
  .presets {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    flex-wrap: wrap;
    margin-top: 0.5rem;
    font-size: 0.8rem;
  }
  .presets-label {
    color: var(--text-muted);
    font-weight: 600;
  }
  .preset-btn {
    background: var(--surface-elevated) !important;
    border: 1px solid var(--border-subtle) !important;
    border-radius: var(--radius-sm);
    padding: 0.25rem 0.5rem !important;
    font-size: 0.78rem !important;
    cursor: pointer;
    transition: all 0.2s;
  }
  .preset-btn:hover {
    border-color: var(--accent-primary) !important;
    color: var(--accent-primary) !important;
  }
</style>
