<script lang="ts">
  import { onMount } from 'svelte';
  import { fly, fade } from 'svelte/transition';
  import { flip } from 'svelte/animate';
  import { page } from '$app/state';
  import { api, establish, websocketURL, ApiError } from '$lib/api';
  import { randomUUID, updateDisplayName } from '$lib/identity';
  import { rememberRoom } from '$lib/recentRooms';
  import { errorText, t } from '$lib/i18n';
  import YouTubePlayer from '$lib/YouTubePlayer.svelte';
  import AddBar from '$lib/room/AddBar.svelte';
  import ChatPanel from '$lib/room/ChatPanel.svelte';
  import SocialToasts, { type SocialToast } from '$lib/room/SocialToasts.svelte';
  import RoomHeader from '$lib/room/RoomHeader.svelte';
  import SettingsPanel from '$lib/room/SettingsPanel.svelte';
  import QueuePanel from '$lib/room/QueuePanel.svelte';
  import PeoplePanel from '$lib/room/PeoplePanel.svelte';
  import ActivityPanel from '$lib/room/ActivityPanel.svelte';
  import PlayerBar from '$lib/room/PlayerBar.svelte';
  import InviteDialog from '$lib/room/InviteDialog.svelte';
  import ShortcutsDialog from '$lib/room/ShortcutsDialog.svelte';
  import RecapDialog, { type RecapStats } from '$lib/room/RecapDialog.svelte';
  import Dialog from '$lib/room/Dialog.svelte';
  import '$lib/room/room.css';
  import { anchorTime, measureClockOffset } from '$lib/clock';
  import {
    automaticEndDelay,
    formatDuration,
    parseYouTubeInput,
    participantNameParts,
    partyCalendar,
    remainingEndReportLease,
    reconnectDelay,
    REACTION_EMOJIS,
    SKIPPED_SPONSOR_CATEGORIES,
    SPONSOR_CATEGORY_LABELS,
    untilParts,
    type AddMode,
    type ChatMessage,
    type Activity as RoomEvent,
    type Member,
    type PresenceState,
    type QueueItem,
    type Snapshot,
    type SponsorSegment,
    type VideoRequest,
  } from '$lib/room';
  import { shouldReanchorPlayback } from '$lib/playerSync';
  import { formatDiagnosticEvents, type DiagnosticEvent } from '$lib/diagnostics';
  import { CalendarPlus, BellRinging, CheckCircle, Info, Play, ShareNetwork, WarningCircle, X } from 'phosphor-svelte';

  const roomId = (page.params.roomId ?? '').toUpperCase();
  const ERROR_MS = 6000;
  const COUNTDOWN_PREROLL_MS = 500;
  let room: Snapshot | null = null;
  // The playback anchor is only re-baselined when playback actually changes
  // (status, position, or media), so the extrapolated live position stays correct
  // across unrelated snapshots (a member joining, a queue edit, …).
  let playbackAnchor = { position: 0, status: 'paused', mediaId: '', rate: 1, revision: -1, at: Date.now() };
  let disposed = false;
  let socket: WebSocket | null = null;
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  let joinWaitTimer: ReturnType<typeof setTimeout> | null = null;
  let resolveJoinWait: (() => void) | null = null;
  let noticeTimer: ReturnType<typeof setTimeout> | null = null;
  let commandPending = false;
  let error = '';
  let joinAttempt = 0;
  let joinInFlight = false;
  let suspended = false;
  let notice = '';
  let noticeKind: 'info' | 'success' | 'error' = 'info';
  let noticeAction: { label: string; run: () => void } | null = null;
  let connected = false;
  let everConnected = false;
  let theater = false;
  let miniPlayer = false;
  let fullscreen = false;
  let diagnostics = { drift: 0, state: 'loading', correctedAt: null as number | null };
  let diagnosticEvents: DiagnosticEvent[] = [];
  let online = true;
  let visible = true;
  let reactions: Array<{ id: string; emoji: string; badge: string; x: number }> = [];
  let bubbles: Array<{ id: string; badge: string; name: string; text: string }> = [];
  let socialToasts: SocialToast[] = [];
  let combo: { id: string; emoji: string; count: number } | null = null;
  let comboTimer: ReturnType<typeof setTimeout> | null = null;
  const recentReactions: Array<{ emoji: string; at: number }> = [];
  const leaveTimers: Record<string, ReturnType<typeof setTimeout>> = {};
  // Event IDs already shown (or present before joining); plain bookkeeping.
  let seenEvents: Record<string, true> | null = null;
  let hiddenUnread = 0;
  // Server-minus-client clock offset; anchors snapshots independent of latency.
  let clockOffset: number | null = null;
  let clockReady: Promise<void> = Promise.resolve();
  // While a countdown runs every player holds at the start position until this
  // client-clock moment, then starts together.
  let countdownEnd = 0;
  let countdownTimer: ReturnType<typeof setTimeout> | null = null;
  // Reaction heatmap for the current video: bucket index -> count.
  let heat: { mediaId: string; buckets: Record<number, number> } = { mediaId: '', buckets: {} };
  // Wait-for-everyone bookkeeping: when each person started buffering.
  let bufferingSince: Record<string, number> = {};
  let lastAutoWait = 0;
  let waitLog: Record<string, number[]> = {};
  let autoPausedAt = 0;
  let scheduleStartedFor = 0;
  let reminderTimer: ReturnType<typeof setTimeout> | null = null;
  let reminderSet = false;
  type SideTab = 'queue' | 'chat' | 'people' | 'activity';
  const sideTabs: SideTab[] = ['queue', 'chat', 'people', 'activity'];
  let sideTab: SideTab = 'queue';
  let settingsOpen = false;
  let shortcutsOpen = false;
  let inviteOpen = false;
  let recapOpen = false;
  let recapShown = false;
  let seekTimer: ReturnType<typeof setTimeout> | null = null;
  let progressTimer: ReturnType<typeof setInterval> | null = null;
  let endedReportTimer: ReturnType<typeof setTimeout> | null = null;
  let splashTimer: ReturnType<typeof setTimeout> | null = null;
  let syncRequest = 0;
  let chat: ChatMessage[] = [];
  let unread = 0;
  let presence: Record<string, PresenceState> = {};
  let myPresence: PresenceState = 'idle';
  let sentPresence = '';
  let splash: string | null = null;
  let namePrompt = false;
  let editingName = false;
  let nameDraft = '';
  let dropActive = false;
  let dragDepth = 0;
  let addInput: HTMLInputElement | null = null;
  let chatInput: HTMLTextAreaElement | null = null;
  let playerWrap: HTMLDivElement;
  let pendingAdd = page.url.searchParams.get('add');
  let confirmDialog: { title: string; confirmLabel: string; danger: boolean; resolve: (ok: boolean) => void } | null =
    null;
  // What this viewer experienced, for the party recap. Never leaves the browser.
  const sessionStart = Date.now();
  let recap = {
    videos: [] as string[],
    reactions: {} as Record<string, number>,
    chatters: {} as Record<string, number>,
    messages: 0,
    peakPeople: 1,
  };
  // Derived as reactive values so the template re-renders when a snapshot changes
  // roles, permissions or names. Script code reads them through the helpers.
  const CAPABILITIES = [
    'playback.play_pause',
    'playback.seek',
    'media.play_now',
    'queue.add',
    'queue.remove',
    'queue.reorder',
    'queue.skip',
    'queue.vote',
    'chat.send',
  ];
  let self: Member | undefined;
  let manager: boolean;
  let caps: Record<string, boolean>;
  $: self = room?.members.find((m) => m.identityId === room?.me);
  $: manager = self?.role === 'owner' || self?.role === 'admin';
  $: caps = Object.fromEntries(
    CAPABILITIES.map((cap) => [cap, !!self && (manager || self.permissions[cap] !== false)]),
  );
  // Script code may run right after `room` changes, before reactive values
  // update, so these read the snapshot directly.
  const me = () => room?.members.find((m) => m.identityId === room?.me);
  const can = (cap: string) => {
    const m = me();
    return !!m && (m.role === 'owner' || m.role === 'admin' || m.permissions[cap] !== false);
  };
  const queueIndex = (items: Snapshot['queue'], itemId: string) => items.findIndex((item) => item.id === itemId);
  // Commands whose result depends on the exact state the user saw hold a shared
  // lock, so a double click cannot apply them twice. Adding, voting and removing
  // are independent intents and never wait on each other.
  const EXCLUSIVE_COMMANDS = new Set([
    'player.play',
    'player.pause',
    'player.rate',
    'queue.reorder',
    'queue.shuffle',
    'queue.skip',
    'member.role',
    'member.kick',
    'member.ban',
    'member.permission',
    'room.visibility',
    'room.transfer',
  ]);
  const inviteLink = () => `${location.origin}${room?.slug ? `/r/${room.slug}` : `/room/${roomId}`}`;

  function updateMediaSession(next: Snapshot) {
    if (typeof navigator === 'undefined' || !('mediaSession' in navigator)) return;
    try {
      const media = next.playback.media;
      navigator.mediaSession.metadata = media
        ? new MediaMetadata({
            title: media.title,
            artist: next.label,
            album: 'KoalaParty',
            artwork: media.thumbnail ? [{ src: media.thumbnail }] : [],
          })
        : null;
      navigator.mediaSession.playbackState = media
        ? next.playback.status === 'playing'
          ? 'playing'
          : 'paused'
        : 'none';
    } catch {
      /* Media Session metadata support varies independently from the API object. */
    }
  }
  function updateMediaPosition() {
    if (typeof navigator === 'undefined' || !('mediaSession' in navigator) || mediaDuration <= 0) return;
    try {
      navigator.mediaSession.setPositionState({
        duration: mediaDuration,
        playbackRate: playbackAnchor.rate,
        position: Math.min(mediaDuration, Math.max(0, livePosition())),
      });
    } catch {
      /* Media Session position support varies by browser and live-stream type. */
    }
  }
  function pushToast(badge: string, name: string, text: string, tone: SocialToast['tone'], kind = '') {
    const last = socialToasts.at(-1);
    // Coalesce bursts such as repeated seeks by the same person into one toast.
    if (last && kind === 'seek' && last.name === name && last.id.startsWith('seek:')) {
      socialToasts = socialToasts.slice(0, -1);
    }
    const toast = { id: `${kind}:${randomUUID()}`, badge, name, text, tone };
    socialToasts = [...socialToasts, toast].slice(-4);
    const timer = setTimeout(() => {
      socialToasts = socialToasts.filter((item) => item.id !== toast.id);
      reactionTimers = reactionTimers.filter((t) => t !== timer);
    }, 4500);
    reactionTimers.push(timer);
  }
  const clockTime = (seconds: unknown) => formatDuration(Number(seconds) || 0);
  function mediaTitle(next: Snapshot, videoId: unknown) {
    const id = String(videoId ?? '');
    const queued = next.queue.find((item) => item.media.providerId === id)?.media.title;
    const current = next.playback.media?.providerId === id ? next.playback.media.title : undefined;
    const title = queued ?? current ?? next.history.find((item) => item.providerId === id)?.title;
    return title && !title.startsWith('YouTube video ') ? `“${title}”` : $t('activity.aVideo');
  }
  // Turns another person's room activity into a short toast line.
  function describeEvent(event: RoomEvent, next: Snapshot): string | null {
    const payload = event.payload ?? {};
    switch (event.type) {
      case 'queue.add': {
        const count = Number(payload.count || 0);
        if (count > 1) return $t('toast.addedMany', { count });
        return payload.next
          ? $t('toast.addedNext', { title: mediaTitle(next, payload.videoId) })
          : $t('toast.added', { title: mediaTitle(next, payload.videoId) });
      }
      case 'media.activated':
        return $t('toast.started', { title: mediaTitle(next, payload.videoId) });
      case 'player.play':
        return $t('toast.play');
      case 'player.pause':
        return payload.reason === 'wait' ? null : $t('toast.pause', { time: clockTime(payload.position) });
      case 'player.seek':
        return $t('toast.seek', { time: clockTime(payload.position) });
      case 'player.rate':
        return $t('toast.rate', { rate: Number(payload.rate || 1) });
      case 'queue.skip':
        return $t('toast.skip');
      case 'media.skip_voted':
        return $t('toast.skipVoted', { votes: next.playback.skipVotes, needed: next.playback.skipNeeded });
      case 'media.vote_skipped':
        return $t('toast.voteSkipped');
      case 'queue.shuffle':
        return $t('toast.shuffle');
      case 'room.rename':
        return payload.name ? $t('toast.renamed', { name: String(payload.name) }) : $t('toast.renameReset');
      case 'room.mode':
        return $t('toast.mode', { mode: $t(`settings.mode.${String(payload.mode)}` as never) });
      default:
        return null;
    }
  }
  function announceChanges(previous: Snapshot, next: Snapshot) {
    const nextMedia = next.playback.media;
    if (nextMedia && nextMedia.id !== (previous.playback.media?.id ?? '')) {
      if (splashTimer) clearTimeout(splashTimer);
      splash = nextMedia.id;
      splashTimer = setTimeout(() => (splash = null), 3200);
      recap.videos = [...recap.videos, nextMedia.title];
    }
    // A finished queue is the natural end of a party: offer the recap once.
    if (!nextMedia && previous.playback.media && !next.queue.length && recap.videos.length >= 2 && !recapShown) {
      recapShown = true;
      recapOpen = true;
    }
    const wasActive = new Set(previous.members.filter((m) => m.active).map((m) => m.identityId));
    for (const member of next.members) {
      if (member.identityId === next.me) continue;
      const parts = participantNameParts(member.displayName);
      if (member.active && !wasActive.has(member.identityId)) {
        // A quick reload looks like leave + join; stay quiet about it.
        const pendingLeave = leaveTimers[member.identityId];
        if (pendingLeave) {
          clearTimeout(pendingLeave);
          delete leaveTimers[member.identityId];
        } else {
          pushToast(parts.badge, parts.label, $t('toast.joined'), 'join');
        }
      } else if (!member.active && wasActive.has(member.identityId) && !leaveTimers[member.identityId]) {
        const id = member.identityId;
        leaveTimers[id] = setTimeout(() => {
          delete leaveTimers[id];
          if (!room?.members.some((m) => m.identityId === id && m.active))
            pushToast(parts.badge, parts.label, $t('toast.left'), 'leave');
        }, 6000);
      }
    }
    recap.peakPeople = Math.max(recap.peakPeople, next.members.filter((m) => m.active).length);
    const seen = seenEvents ?? {};
    for (const event of next.events) {
      if (seen[event.id]) continue;
      seen[event.id] = true;
      if (!seenEvents || !event.actorId || event.actorId === next.me) continue;
      const parts = participantNameParts(event.actorName || $t('activity.someone'));
      // Titles are resolved just after a video is added; wait for them briefly.
      if (event.type === 'queue.add' || event.type === 'media.activated') {
        const timer = setTimeout(() => {
          reactionTimers = reactionTimers.filter((t) => t !== timer);
          const text = room ? describeEvent(event, room) : null;
          if (text) pushToast(parts.badge, parts.label, text, 'action');
        }, 1200);
        reactionTimers.push(timer);
        continue;
      }
      const text = describeEvent(event, next);
      if (text) pushToast(parts.badge, parts.label, text, 'action', event.type === 'player.seek' ? 'seek' : '');
    }
    seenEvents = seen;
  }
  function scheduleCountdown(end: number) {
    if (countdownTimer) clearTimeout(countdownTimer);
    countdownTimer = null;
    countdownEnd = end;
    if (!end) return;
    // Release the player slightly early: YouTube needs a moment to buffer after
    // play, so starting half a second ahead lands everyone on the shared start.
    countdownTimer = setTimeout(
      () => {
        countdownTimer = null;
        countdownEnd = 0;
        updateProgressTimer();
      },
      Math.max(0, end - Date.now() - COUNTDOWN_PREROLL_MS),
    );
    updateProgressTimer();
  }
  function updateRoom(next: Snapshot) {
    if (room && next.revision < room.revision) return;
    const pb = next.playback;
    const mediaId = pb.media?.id ?? '';
    if (mediaId !== playbackAnchor.mediaId && seekTimer) {
      clearTimeout(seekTimer);
      seekTimer = null;
    }
    if ((mediaId !== playbackAnchor.mediaId || pb.revision !== playbackAnchor.revision) && endedReportTimer) {
      clearTimeout(endedReportTimer);
      endedReportTimer = null;
    }
    const rate = pb.rate || 1;
    if (!room || shouldReanchorPlayback(playbackAnchor, { mediaId, revision: pb.revision })) {
      if (mediaId !== playbackAnchor.mediaId) mediaDuration = 0;
      const now = Date.now();
      // A future start is a countdown; convert it to this client's clock.
      const countdown =
        pb.status === 'playing' && pb.startsAt && next.serverTime && pb.startsAt > next.serverTime
          ? clockOffset !== null
            ? pb.startsAt - clockOffset
            : now + (pb.startsAt - next.serverTime)
          : 0;
      playbackAnchor = {
        position: pb.position,
        status: pb.status,
        mediaId,
        rate,
        revision: pb.revision,
        at: countdown > now ? countdown : anchorTime(next.serverTime, clockOffset, now),
      };
      scheduleCountdown(countdown > now ? countdown : 0);
    }
    if (room) announceChanges(room, next);
    else seenEvents = Object.fromEntries(next.events.map((event) => [event.id, true as const]));
    if (heat.mediaId && heat.mediaId !== mediaId) heat = { mediaId: '', buckets: {} };
    room = next;
    updateMediaSession(next);
    updateMediaPosition();
    updateProgressTimer();
  }
  const livePosition = (now = Date.now()) =>
    playbackAnchor.status === 'playing'
      ? playbackAnchor.position + (Math.max(0, now - playbackAnchor.at) / 1000) * playbackAnchor.rate
      : playbackAnchor.position;
  let mediaDuration = 0;
  let nowTick = Date.now();
  function updateProgressTimer() {
    if (progressTimer) clearInterval(progressTimer);
    progressTimer = null;
    nowTick = Date.now();
    if (!visible) return;
    if (countdownEnd) {
      progressTimer = setInterval(() => (nowTick = Date.now()), 200);
      return;
    }
    if (playbackAnchor.status !== 'playing' || !playbackAnchor.mediaId) return;
    progressTimer = setInterval(() => (nowTick = Date.now()), 500);
  }
  function handleDuration(duration: number) {
    mediaDuration = duration;
    updateMediaPosition();
  }
  function showNotice(
    message: string,
    clearAfter = 0,
    kind: 'info' | 'success' | 'error' = 'info',
    action: { label: string; run: () => void } | null = null,
  ) {
    if (noticeTimer) clearTimeout(noticeTimer);
    noticeTimer = null;
    notice = message;
    noticeKind = kind;
    noticeAction = action;
    if (clearAfter > 0)
      noticeTimer = setTimeout(() => {
        notice = '';
        noticeAction = null;
      }, clearAfter);
  }
  const quickNotice = (message: string, kind: 'info' | 'success' | 'error') =>
    showNotice(message, kind === 'error' ? ERROR_MS : 2600, kind);
  function dismissNotice() {
    if (noticeTimer) clearTimeout(noticeTimer);
    notice = '';
    noticeAction = null;
  }
  function pushBubble(badge: string, name: string, text: string) {
    const bubble = { id: randomUUID(), badge, name, text };
    bubbles = [...bubbles, bubble].slice(-3);
    const timer = setTimeout(() => {
      bubbles = bubbles.filter((item) => item.id !== bubble.id);
      reactionTimers = reactionTimers.filter((t) => t !== timer);
    }, 5000);
    reactionTimers.push(timer);
  }
  function cancelJoinWait() {
    if (joinWaitTimer) clearTimeout(joinWaitTimer);
    joinWaitTimer = null;
    resolveJoinWait?.();
    resolveJoinWait = null;
  }
  function waitForReconnect(delay: number) {
    return new Promise<void>((resolve) => {
      resolveJoinWait = resolve;
      joinWaitTimer = setTimeout(() => {
        joinWaitTimer = null;
        resolveJoinWait = null;
        resolve();
      }, delay);
    });
  }
  function recordDiagnostic(event: DiagnosticEvent) {
    diagnosticEvents = [...diagnosticEvents, event].slice(-60);
    if (['error', 'retry', 'stalled', 'offline'].includes(event.event)) {
      console.warn('[KoalaParty diagnostic]', event);
    }
  }
  function diagnosticReport() {
    return formatDiagnosticEvents(diagnosticEvents, {
      roomId,
      online,
      visible,
      connected,
      roomRevision: room?.revision ?? null,
      playbackRevision: room?.playback.revision ?? null,
      userAgent: typeof navigator === 'undefined' ? null : navigator.userAgent,
    });
  }
  async function copyDiagnostics() {
    try {
      await navigator.clipboard.writeText(diagnosticReport());
      showNotice($t('notice.diagnosticsCopied'), 2600, 'success');
    } catch {
      showNotice($t('notice.diagnosticsCopyFailed'), ERROR_MS, 'error');
    }
  }
  function downloadDiagnostics() {
    const url = URL.createObjectURL(new Blob([diagnosticReport()], { type: 'text/plain;charset=utf-8' }));
    const link = document.createElement('a');
    link.href = url;
    link.download = `koalaparty-${roomId.toLowerCase()}-diagnostics.txt`;
    link.click();
    URL.revokeObjectURL(url);
    showNotice($t('notice.diagnosticsDownloaded'), 2600, 'success');
  }
  function syncNow() {
    syncRequest += 1;
    showNotice($t('notice.syncing'), 1600, 'info');
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
  function selectTab(tab: SideTab) {
    sideTab = tab;
    if (tab === 'chat') unread = 0;
  }
  function onTabKeydown(event: KeyboardEvent) {
    const current = sideTabs.indexOf(sideTab);
    let next: number;
    if (event.key === 'ArrowRight') next = (current + 1) % sideTabs.length;
    else if (event.key === 'ArrowLeft') next = (current - 1 + sideTabs.length) % sideTabs.length;
    else if (event.key === 'Home') next = 0;
    else if (event.key === 'End') next = sideTabs.length - 1;
    else return;
    event.preventDefault();
    selectTab(sideTabs[next]);
    requestAnimationFrame(() => document.getElementById(`room-tab-${sideTab}`)?.focus());
  }
  function scheduleSeek(position: number) {
    const mediaId = room?.playback.media?.id;
    if (!mediaId) return;
    if (seekTimer) clearTimeout(seekTimer);
    seekTimer = setTimeout(() => {
      seekTimer = null;
      if (room?.playback.media?.id !== mediaId) return;
      void playbackCommand('player.seek', { position });
    }, 300);
  }
  // Segments the room actually skips: only the acted-on categories, and only while the
  // room has SponsorBlock enabled. Passed to the player, which performs the jump.
  const sponsorSegments = (): SponsorSegment[] =>
    room?.sponsorBlock
      ? (room.playback.segments ?? []).filter((s) => SKIPPED_SPONSOR_CATEGORIES.includes(s.category))
      : [];
  // The player already jumped locally; broadcast the seek so everyone skips in sync
  // (stale races are harmless — the winner's snapshot syncs the rest) and surface a
  // brief, attributed notice at the point of use, as the SponsorBlock licence requires.
  function skipSponsor(segment: SponsorSegment) {
    command('player.seek', { position: segment.end }, { silentStale: true, bypassPending: true });
    showNotice(
      $t('notice.sponsorSkipped', { segment: SPONSOR_CATEGORY_LABELS[segment.category] ?? 'Segment' }),
      2200,
      'info',
    );
  }
  function announceCreation() {
    try {
      const raw = sessionStorage.getItem('koalaparty.created');
      if (!raw) return;
      const info = JSON.parse(raw) as { id?: string; copied?: boolean };
      if (info.id !== roomId) return;
      sessionStorage.removeItem('koalaparty.created');
      showNotice(info.copied ? $t('notice.createdCopied') : $t('notice.created'), 4500, 'success');
    } catch {
      /* sessionStorage unavailable */
    }
  }
  function prepareNamePrompt() {
    const current = me();
    if (!current || current.accountLinked) return;
    try {
      namePrompt = localStorage.getItem('koalaparty.nameChosen') !== '1';
    } catch {
      namePrompt = false;
    }
  }
  // Entering a room retries on transient failures instead of dead-ending. A brief
  // 5xx / Bad Gateway (server restarting, proxy blip) or a network error is
  // retried with backoff; only a genuine client error (4xx: forbidden, not
  // found, banned) stops and shows the fatal screen.
  async function joinWithRetry() {
    if (disposed || suspended || joinInFlight || socket || !navigator.onLine) return;
    joinInFlight = true;
    try {
      while (!disposed && !suspended && navigator.onLine && !socket) {
        try {
          await establish();
          if (disposed || socket) return;
          const firstJoin = !room;
          if (firstJoin) await Promise.race([clockReady, new Promise((resolve) => setTimeout(resolve, 1500))]);
          const joinedRoom = await api<Snapshot>(`/api/rooms/${roomId}`);
          updateRoom(joinedRoom);
          rememberRoom({
            id: joinedRoom.id,
            label: joinedRoom.label,
            title: joinedRoom.playback.media?.title ?? '',
          });
          if (disposed || socket) return;
          error = '';
          joinAttempt = 0;
          connect();
          if (firstJoin) {
            announceCreation();
            prepareNamePrompt();
            consumePendingAdd();
          }
          return;
        } catch (e) {
          if (disposed || !navigator.onLine) return;
          const status = e instanceof ApiError ? e.status : undefined;
          if (status !== undefined && status >= 400 && status < 500) {
            error = errorText(e);
            return;
          }
          joinAttempt++;
          await waitForReconnect(reconnectDelay(joinAttempt));
        }
      }
    } finally {
      joinInFlight = false;
    }
  }
  // Links shared to KoalaParty (share target, bookmarklet) arrive as ?add=…
  function consumePendingAdd() {
    if (!pendingAdd) return;
    const text = pendingAdd;
    pendingAdd = null;
    history.replaceState(history.state, '', `/room/${roomId}`);
    if (!handleExternalText(text)) showNotice($t('notice.sharedNotYouTube'), ERROR_MS, 'error');
  }
  // Theater mode is a per-device viewing preference, so persist it across reloads.
  function setTheater(value: boolean) {
    theater = value;
    try {
      localStorage.setItem('koalaparty.theater', value ? '1' : '0');
    } catch {
      /* localStorage unavailable */
    }
  }
  async function toggleFullscreen() {
    try {
      if (document.fullscreenElement) await document.exitFullscreen();
      else await playerWrap?.requestFullscreen();
    } catch {
      showNotice($t('notice.fullscreenUnavailable'), 2500, 'error');
    }
  }
  function isTyping(target: EventTarget | null) {
    return (
      target instanceof HTMLElement &&
      !!target.closest('input, textarea, select, [contenteditable="true"], [contenteditable=""]')
    );
  }
  function handleExternalText(text: string): boolean {
    if (!room || !can('queue.add')) return false;
    const parsed = parseYouTubeInput(text);
    if (parsed.videos.length) {
      void addVideos(parsed.videos, 'queue').then((ok) => {
        if (ok && parsed.playlistId && room?.searchEnabled) offerPlaylist(parsed.playlistId);
      });
      return true;
    }
    if (parsed.playlistId) {
      void importPlaylist(parsed.playlistId);
      return true;
    }
    return false;
  }
  function syncClock() {
    clockReady = measureClockOffset().then((offset) => {
      if (offset !== null) clockOffset = offset;
    });
  }
  // The first connected person able to control playback coordinates automatic
  // actions (waiting for buffering viewers, starting a scheduled party), so
  // several browsers never race each other.
  function isCoordinator() {
    if (!room) return false;
    const rank = (member: Member) => (member.role === 'owner' ? 0 : member.role === 'admin' ? 1 : 2);
    const controllers = room.members
      .filter(
        (member) =>
          member.active &&
          (member.role === 'owner' || member.role === 'admin' || member.permissions['playback.play_pause'] !== false),
      )
      .sort((a, b) => rank(a) - rank(b) || a.identityId.localeCompare(b.identityId));
    return controllers[0]?.identityId === room.me;
  }
  function tickAutomation() {
    if (!room || !connected || !isCoordinator()) return;
    const now = Date.now();
    const active = new Set(room.members.filter((member) => member.active).map((member) => member.identityId));
    // Someone whose connection keeps stalling would otherwise pause the room every
    // few seconds; after two waits in five minutes the party carries on without them.
    const patient = (id: string) => (waitLog[id] ?? []).filter((at) => now - at < 300_000).length < 2;
    const slow = Object.entries(bufferingSince).filter(
      ([id, since]) => active.has(id) && now - since > 2500 && patient(id),
    );
    const pb = room.playback;
    // Waiting only makes sense with company: never auto-pause a viewer who is alone.
    if (
      room.waitForAll &&
      active.size > 1 &&
      pb.status === 'playing' &&
      !countdownEnd &&
      pb.media &&
      slow.length &&
      now - lastAutoWait > 20_000
    ) {
      lastAutoWait = now;
      for (const [id] of slow) waitLog[id] = [...(waitLog[id] ?? []).filter((at) => now - at < 300_000), now];
      void command(
        'player.pause',
        { position: livePosition(), reason: 'wait' },
        { silentStale: true, bypassPending: true },
      );
    } else if (pb.status === 'paused' && pb.autoPaused && pb.media) {
      if (!autoPausedAt) autoPausedAt = now;
      const stillBuffering = Object.keys(bufferingSince).some((id) => active.has(id) && patient(id));
      // Resume together once everyone is ready, but never wait forever.
      if ((!stillBuffering && now - autoPausedAt > 1000) || now - autoPausedAt > 20_000) {
        autoPausedAt = 0;
        void command(
          'player.play',
          { position: pb.position, countdown: Math.max(1, room.countdownSeconds ?? 3) },
          { silentStale: true, bypassPending: true },
        );
      }
    } else autoPausedAt = 0;
    // Start a scheduled party on time (server clock) if something is queued. The
    // window tolerates background tabs whose timers browsers slow to once a minute.
    const serverNow = now + (clockOffset ?? 0);
    if (
      room.scheduledAt &&
      serverNow >= room.scheduledAt &&
      serverNow - room.scheduledAt < 5 * 60_000 &&
      !pb.media &&
      room.queue.length &&
      scheduleStartedFor !== room.scheduledAt
    ) {
      scheduleStartedFor = room.scheduledAt;
      void command('queue.skip', {}, { silentStale: true, bypassPending: true });
    }
  }
  let reactionTimers: ReturnType<typeof setTimeout>[] = [];
  onMount(() => {
    syncClock();
    const clockTimer = setInterval(syncClock, 10 * 60_000);
    const automationTimer = setInterval(tickAutomation, 1000);
    online = navigator.onLine;
    visible = document.visibilityState === 'visible';
    const siteHeader = document.querySelector<HTMLElement>('.site-header');
    const headerObserver =
      siteHeader && 'ResizeObserver' in window
        ? new ResizeObserver(() =>
            document.documentElement.style.setProperty('--site-header-height', `${siteHeader.offsetHeight}px`),
          )
        : null;
    if (siteHeader) headerObserver?.observe(siteHeader);
    const onOnline = () => {
      online = true;
      cancelJoinWait();
      recordDiagnostic({ at: new Date().toISOString(), source: 'room', event: 'online', details: {} });
      if (reconnectTimer) clearTimeout(reconnectTimer);
      reconnectTimer = null;
      if (!socket) void joinWithRetry();
    };
    const onOffline = () => {
      online = false;
      cancelJoinWait();
      if (reconnectTimer) clearTimeout(reconnectTimer);
      reconnectTimer = null;
      const activeSocket = socket;
      socket = null;
      connected = false;
      activeSocket?.close();
      recordDiagnostic({ at: new Date().toISOString(), source: 'room', event: 'offline', details: {} });
      showNotice($t('notice.offline'), 0, 'error');
    };
    const onVisibility = () => {
      visible = document.visibilityState === 'visible';
      if (visible) {
        hiddenUnread = 0;
        syncClock();
      }
      updateProgressTimer();
      recordDiagnostic({
        at: new Date().toISOString(),
        source: 'room',
        event: visible ? 'visible' : 'hidden',
        details: {},
      });
      if (visible && !socket) void joinWithRetry();
    };
    const onKeydown = (event: KeyboardEvent) => {
      if (event.ctrlKey || event.metaKey || event.altKey || event.repeat || !room) return;
      if (event.key === 'Escape') {
        if (shortcutsOpen) shortcutsOpen = false;
        else if (miniPlayer) miniPlayer = false;
        return;
      }
      if (isTyping(event.target) || confirmDialog || shortcutsOpen || inviteOpen || recapOpen) return;
      const onControl = event.target instanceof HTMLElement && !!event.target.closest('button, a, summary');
      const key = event.key.toLowerCase();
      const media = room.playback.media;
      if ((key === 'k' || (key === ' ' && !onControl)) && media && can('playback.play_pause')) {
        event.preventDefault();
        void togglePlay();
      } else if ((event.key === 'ArrowLeft' || event.key === 'ArrowRight') && media && can('playback.seek')) {
        if (onControl && event.target instanceof HTMLElement && event.target.getAttribute('role') === 'tab') return;
        event.preventDefault();
        const delta = event.key === 'ArrowLeft' ? -5 : 5;
        void command('player.seek', { position: Math.max(0, livePosition() + delta) });
      } else if (key === '/' || key === 'a') {
        event.preventDefault();
        addInput?.focus();
      } else if (key === 'c') {
        event.preventDefault();
        selectTab('chat');
        requestAnimationFrame(() => chatInput?.focus());
      } else if (key === 't') {
        setTheater(!theater);
      } else if (key === 'f') {
        void toggleFullscreen();
      } else if (key === 'm') {
        miniPlayer = !miniPlayer;
      } else if (/^[0-9]$/.test(key)) {
        const emoji = REACTION_EMOJIS[(Number(key) + 9) % 10];
        if (emoji) react(emoji);
      } else if (key === '?') {
        shortcutsOpen = true;
      }
    };
    // Paste a YouTube link anywhere in the room to queue it — no need to find the box.
    const onPaste = (event: ClipboardEvent) => {
      if (isTyping(event.target)) return;
      const text = event.clipboardData?.getData('text/plain') || event.clipboardData?.getData('text') || '';
      if (text && handleExternalText(text)) event.preventDefault();
    };
    const carriesText = (event: DragEvent) =>
      !!event.dataTransfer &&
      !event.dataTransfer.types.includes('Files') &&
      (event.dataTransfer.types.includes('text/uri-list') || event.dataTransfer.types.includes('text/plain'));
    const onDragEnter = (event: DragEvent) => {
      if (!carriesText(event) || !can('queue.add') || (event.target as HTMLElement)?.closest?.('.queue')) return;
      dragDepth += 1;
      dropActive = true;
    };
    const onDragLeave = () => {
      if (!dropActive) return;
      dragDepth = Math.max(0, dragDepth - 1);
      if (!dragDepth) dropActive = false;
    };
    const onDragOver = (event: DragEvent) => {
      if (dropActive) event.preventDefault();
    };
    const onDrop = (event: DragEvent) => {
      if (!dropActive) return;
      event.preventDefault();
      dropActive = false;
      dragDepth = 0;
      const text = event.dataTransfer?.getData('text/uri-list') || event.dataTransfer?.getData('text/plain') || '';
      if (!handleExternalText(text)) showNotice($t('notice.dropNotYouTube'), 3000, 'error');
    };
    const onFullscreenChange = () => (fullscreen = document.fullscreenElement === playerWrap);
    const onPageHide = () => {
      suspended = true;
      cancelJoinWait();
      if (reconnectTimer) clearTimeout(reconnectTimer);
      reconnectTimer = null;
      const activeSocket = socket;
      socket = null;
      connected = false;
      activeSocket?.close();
    };
    const onPageShow = () => {
      suspended = false;
      if (!socket) void Promise.resolve().then(joinWithRetry);
    };
    const mediaActions: MediaSessionAction[] = [];
    const setMediaAction = (action: MediaSessionAction, handler: MediaSessionActionHandler) => {
      if (!('mediaSession' in navigator)) return;
      try {
        navigator.mediaSession.setActionHandler(action, handler);
        mediaActions.push(action);
      } catch {
        /* Unsupported Media Session action in this browser. */
      }
    };
    setMediaAction('play', () => {
      if (room?.playback.media && can('playback.play_pause')) void command('player.play', { position: livePosition() });
    });
    setMediaAction('pause', () => {
      if (room?.playback.media && can('playback.play_pause'))
        void command('player.pause', { position: livePosition() });
    });
    setMediaAction('seekbackward', (details) => {
      if (room?.playback.media && can('playback.seek'))
        void command('player.seek', { position: Math.max(0, livePosition() - (details.seekOffset ?? 10)) });
    });
    setMediaAction('seekforward', (details) => {
      if (room?.playback.media && can('playback.seek'))
        void command('player.seek', { position: Math.max(0, livePosition() + (details.seekOffset ?? 10)) });
    });
    setMediaAction('seekto', (details) => {
      if (room?.playback.media && can('playback.seek') && details.seekTime !== undefined)
        void command('player.seek', { position: Math.max(0, details.seekTime) });
    });
    setMediaAction('nexttrack', () => {
      if (room?.queue.length && can('queue.skip')) void command('queue.skip', {}, { silentStale: true });
    });
    window.addEventListener('online', onOnline);
    window.addEventListener('offline', onOffline);
    document.addEventListener('visibilitychange', onVisibility);
    document.addEventListener('keydown', onKeydown);
    document.addEventListener('paste', onPaste);
    document.addEventListener('dragenter', onDragEnter);
    document.addEventListener('dragleave', onDragLeave);
    document.addEventListener('dragover', onDragOver);
    document.addEventListener('drop', onDrop);
    document.addEventListener('fullscreenchange', onFullscreenChange);
    window.addEventListener('pagehide', onPageHide);
    window.addEventListener('pageshow', onPageShow);
    try {
      theater = localStorage.getItem('koalaparty.theater') === '1';
    } catch {
      /* localStorage unavailable */
    }
    void joinWithRetry();
    return () => {
      disposed = true;
      clearInterval(clockTimer);
      clearInterval(automationTimer);
      if (comboTimer) clearTimeout(comboTimer);
      if (countdownTimer) clearTimeout(countdownTimer);
      if (reminderTimer) clearTimeout(reminderTimer);
      Object.values(leaveTimers).forEach(clearTimeout);
      headerObserver?.disconnect();
      if (reconnectTimer) clearTimeout(reconnectTimer);
      cancelJoinWait();
      if (noticeTimer) clearTimeout(noticeTimer);
      if (seekTimer) clearTimeout(seekTimer);
      if (progressTimer) clearInterval(progressTimer);
      if (endedReportTimer) clearTimeout(endedReportTimer);
      if (splashTimer) clearTimeout(splashTimer);
      reactionTimers.forEach(clearTimeout);
      reactionTimers = [];
      const activeSocket = socket;
      socket = null;
      activeSocket?.close();
      window.removeEventListener('online', onOnline);
      window.removeEventListener('offline', onOffline);
      document.removeEventListener('visibilitychange', onVisibility);
      document.removeEventListener('keydown', onKeydown);
      document.removeEventListener('paste', onPaste);
      document.removeEventListener('dragenter', onDragEnter);
      document.removeEventListener('dragleave', onDragLeave);
      document.removeEventListener('dragover', onDragOver);
      document.removeEventListener('drop', onDrop);
      document.removeEventListener('fullscreenchange', onFullscreenChange);
      window.removeEventListener('pagehide', onPageHide);
      window.removeEventListener('pageshow', onPageShow);
      if ('mediaSession' in navigator) {
        for (const action of mediaActions) navigator.mediaSession.setActionHandler(action, null);
        navigator.mediaSession.metadata = null;
        navigator.mediaSession.playbackState = 'none';
      }
    };
  });
  function connect() {
    if (disposed || socket) return;
    const ws = new WebSocket(websocketURL(`/api/rooms/${roomId}/ws`));
    socket = ws;
    ws.onopen = () => {
      if (socket !== ws) return;
      if (reconnectTimer) clearTimeout(reconnectTimer);
      reconnectTimer = null;
      connected = true;
      recordDiagnostic({ at: new Date().toISOString(), source: 'websocket', event: 'open', details: {} });
      if (everConnected) showNotice($t('notice.reconnected'), 1800, 'success');
      everConnected = true;
      sentPresence = '';
      flushPresence();
    };
    ws.onclose = () => {
      if (socket !== ws) return;
      socket = null;
      connected = false;
      recordDiagnostic({ at: new Date().toISOString(), source: 'websocket', event: 'close', details: {} });
      if (disposed || suspended) return;
      showNotice($t('notice.connectionLost'), 0, 'error');
      if (!navigator.onLine) return;
      reconnectTimer = setTimeout(() => {
        reconnectTimer = null;
        void joinWithRetry();
      }, reconnectDelay(++joinAttempt));
    };
    ws.onerror = () =>
      recordDiagnostic({ at: new Date().toISOString(), source: 'websocket', event: 'error', details: {} });
    ws.onmessage = (event) => {
      if (socket !== ws) return;
      try {
        const data = JSON.parse(event.data);
        if (data.type === 'snapshot') {
          recordDiagnostic({ at: new Date().toISOString(), source: 'websocket', event: 'snapshot', details: {} });
          updateRoom(data.payload);
        } else if (data.type === 'reaction') {
          receiveReaction(String(data.emoji), String(data.identityId ?? ''), data.mediaId, data.bucket);
        } else if (data.type === 'heatmap') {
          const buckets: Record<number, number> = {};
          for (const [bucket, count] of Object.entries((data.buckets ?? {}) as Record<string, number>))
            buckets[Number(bucket)] = Number(count);
          heat = { mediaId: String(data.mediaId ?? ''), buckets };
        } else if (data.type === 'chat.history') {
          chat = Array.isArray(data.messages) ? data.messages : [];
        } else if (data.type === 'chat') {
          receiveChat(data.message as ChatMessage);
        } else if (data.type === 'presence.all') {
          presence = data.states && typeof data.states === 'object' ? data.states : {};
          bufferingSince = {};
          for (const [id, state] of Object.entries(presence))
            if (state === 'buffering') bufferingSince[id] = Date.now();
        } else if (data.type === 'presence') {
          const id = String(data.identityId);
          const state = data.state as PresenceState;
          presence = { ...presence, [id]: state };
          if (state === 'buffering') bufferingSince = { [id]: bufferingSince[id] ?? Date.now(), ...bufferingSince };
          else if (bufferingSince[id]) {
            const next = { ...bufferingSince };
            delete next[id];
            bufferingSince = next;
          }
        } else if (data.type === 'error') {
          recordDiagnostic({
            at: new Date().toISOString(),
            source: 'websocket',
            event: 'command_error',
            details: { code: typeof data.code === 'string' ? data.code : null },
          });
          showNotice(errorText(Object.assign(new Error(data.message || ''), { code: data.code })), ERROR_MS, 'error');
        }
      } catch {
        recordDiagnostic({ at: new Date().toISOString(), source: 'websocket', event: 'invalid_message', details: {} });
        showNotice($t('notice.invalidUpdate'), 0, 'error');
        ws.close();
      }
    };
  }
  function receiveReaction(emoji: string, identityId: string, mediaId?: string, bucket?: number) {
    const sender = room?.members.find((member) => member.identityId === identityId);
    const badge = sender ? participantNameParts(sender.displayName).badge : '';
    const reaction = { id: randomUUID(), emoji, badge, x: Math.round(Math.random() * 60) };
    reactions = [...reactions, reaction].slice(-24);
    const timer = setTimeout(() => {
      reactions = reactions.filter((item) => item.id !== reaction.id);
      reactionTimers = reactionTimers.filter((t) => t !== timer);
    }, 2600);
    reactionTimers.push(timer);
    recap.reactions = { ...recap.reactions, [emoji]: (recap.reactions[emoji] ?? 0) + 1 };
    if (mediaId && typeof bucket === 'number') {
      const buckets = heat.mediaId === mediaId ? { ...heat.buckets } : {};
      buckets[bucket] = (buckets[bucket] ?? 0) + 1;
      heat = { mediaId, buckets };
    }
    // Three or more of the same emoji within a moment become a room-wide combo.
    const now = Date.now();
    recentReactions.push({ emoji, at: now });
    while (recentReactions.length && now - recentReactions[0].at > 4000) recentReactions.shift();
    const count = recentReactions.filter((item) => item.emoji === emoji).length;
    if (count >= 3) {
      combo = { id: randomUUID(), emoji, count };
      if (comboTimer) clearTimeout(comboTimer);
      comboTimer = setTimeout(() => (combo = null), 1800);
    }
  }
  function receiveChat(message: ChatMessage) {
    chat = [...chat, message].slice(-200);
    const parts = participantNameParts(message.name);
    recap.messages += 1;
    recap.chatters = { ...recap.chatters, [parts.label]: (recap.chatters[parts.label] ?? 0) + 1 };
    const chatVisible = sideTab === 'chat' && !theater && !fullscreen && !miniPlayer;
    if (sideTab !== 'chat') unread += 1;
    if (document.visibilityState !== 'visible' && message.identityId !== room?.me) hiddenUnread += 1;
    if (!chatVisible && message.identityId !== room?.me) pushBubble(parts.badge, parts.label, message.text);
  }
  function sendChat(text: string): boolean {
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      showNotice($t('notice.notConnected'), 3000, 'error');
      return false;
    }
    socket.send(JSON.stringify({ type: 'chat.send', requestId: randomUUID(), payload: { text } }));
    return true;
  }
  function reportPresence(state: PresenceState) {
    myPresence = state;
    flushPresence();
  }
  function flushPresence() {
    if (!socket || socket.readyState !== WebSocket.OPEN || sentPresence === myPresence) return;
    sentPresence = myPresence;
    socket.send(JSON.stringify({ type: 'presence.state', requestId: randomUUID(), payload: { state: myPresence } }));
  }
  function react(emoji: string) {
    if (!socket || socket.readyState !== WebSocket.OPEN) return;
    socket.send(JSON.stringify({ type: 'reaction.send', requestId: randomUUID(), payload: { emoji } }));
  }
  async function command(
    type: string,
    payload: Record<string, unknown> = {},
    opts: { silentStale?: boolean; bypassPending?: boolean } = {},
  ): Promise<boolean> {
    if (!room) return false;
    // bypassPending lets an automatic action (a SponsorBlock skip) fire even while a
    // user command is in flight, so the synchronized seek is never silently dropped.
    // Such commands leave the shared pending lock untouched to avoid clobbering it.
    const managePending = !opts.bypassPending && EXCLUSIVE_COMMANDS.has(type);
    const requestId = randomUUID();
    if (managePending) {
      if (commandPending) return false;
      commandPending = true;
    }
    try {
      recordDiagnostic({
        at: new Date().toISOString(),
        source: 'command',
        event: 'started',
        details: { type, requestId },
      });
      updateRoom(
        await api(`/api/rooms/${roomId}/commands`, {
          method: 'POST',
          body: JSON.stringify({
            type,
            requestId,
            expectedRevision: room.revision,
            expectedPlaybackRevision: room.playback.revision,
            payload,
          }),
        }),
      );
      recordDiagnostic({
        at: new Date().toISOString(),
        source: 'command',
        event: 'succeeded',
        details: { type, requestId },
      });
      return true;
    } catch (e) {
      // A stale-revision race on an automatic action (e.g. every client that can
      // skip firing at end-of-video, or several skipping the same sponsor) is
      // expected and harmless: the winner's snapshot reconciles everyone. Suppress
      // that toast so it never surfaces as an error.
      if (opts.silentStale && e instanceof ApiError && e.status === 409) return false;
      recordDiagnostic({
        at: new Date().toISOString(),
        source: 'command',
        event: 'failed',
        details: {
          type,
          requestId,
          status: e instanceof ApiError ? e.status : null,
          code: e instanceof ApiError ? (e.code ?? null) : null,
        },
      });
      showNotice(errorText(e), ERROR_MS, 'error');
      return false;
    } finally {
      if (managePending) commandPending = false;
    }
  }
  const settingsCommand = (type: string, payload: Record<string, unknown> = {}) => command(type, payload);

  async function playbackCommand(type: string, payload: Record<string, unknown>) {
    if (!(await command(type, payload, { silentStale: true, bypassPending: true }))) syncRequest += 1;
  }
  function togglePlay() {
    if (!room) return;
    return command(room.playback.status === 'playing' ? 'player.pause' : 'player.play', { position: livePosition() });
  }

  function reportEnded(mediaId: string, position: number, duration: number, attempt = 0) {
    if (!room || room.playback.media?.id !== mediaId) return;
    const revision = room.playback.revision;
    const delay = automaticEndDelay(room);
    if (delay === null) return;
    if (endedReportTimer) clearTimeout(endedReportTimer);
    endedReportTimer = setTimeout(() => {
      endedReportTimer = null;
      if (room?.playback.media?.id !== mediaId || room.playback.revision !== revision) return;
      const signature = `${mediaId}:${revision}`;
      const storageKey = `koalaparty.ended.${roomId}`;
      try {
        const stored = JSON.parse(localStorage.getItem(storageKey) ?? 'null') as {
          signature?: string;
          at?: number;
        } | null;
        const leaseDelay = remainingEndReportLease(stored, signature);
        if (leaseDelay > 0) {
          endedReportTimer = setTimeout(() => reportEnded(mediaId, position, duration, attempt), leaseDelay + 25);
          return;
        }
        localStorage.setItem(storageKey, JSON.stringify({ signature, at: Date.now() }));
      } catch {
        /* localStorage coordination is best-effort; server revisions remain authoritative */
      }
      void command('playback.ended', { mediaId, position, duration }, { silentStale: true, bypassPending: true }).then(
        (success) => {
          if (success || room?.playback.media?.id !== mediaId || room.playback.revision !== revision) return;
          try {
            const stored = JSON.parse(localStorage.getItem(storageKey) ?? 'null') as { signature?: string } | null;
            if (stored?.signature === signature) localStorage.removeItem(storageKey);
          } catch {
            /* localStorage cleanup is best-effort */
          }
          if (attempt < 2) {
            endedReportTimer = setTimeout(
              () => reportEnded(mediaId, position, duration, attempt + 1),
              reconnectDelay(attempt + 1),
            );
          }
        },
      );
    }, delay);
  }
  async function addVideos(videos: VideoRequest[], mode: AddMode): Promise<boolean> {
    if (!room || !videos.length) return false;
    const idle = !room.playback.media;
    let ok: boolean;
    if (mode === 'now') {
      ok = await command('queue.play_now', { ...videos[0] });
      if (ok && videos.length > 1) await command('queue.add', { items: videos.slice(1), position: 0 });
    } else {
      const payload: Record<string, unknown> = videos.length === 1 ? { ...videos[0] } : { items: videos };
      if (mode === 'next') payload.position = 0;
      ok = await command('queue.add', payload);
    }
    if (ok) {
      showNotice(
        mode === 'now' || idle
          ? $t('notice.nowPlaying')
          : videos.length > 1
            ? $t('notice.addedMany', { count: videos.length })
            : mode === 'next'
              ? $t('notice.queuedNext')
              : $t('notice.added'),
        2200,
        'success',
      );
    }
    return ok;
  }
  function offerPlaylist(listId: string) {
    showNotice($t('notice.partOfPlaylist'), 8000, 'info', {
      label: $t('notice.addPlaylist'),
      run: () => void importPlaylist(listId),
    });
  }
  async function importPlaylist(listId: string) {
    if (!room?.searchEnabled) {
      showNotice($t('notice.playlistDisabled'), ERROR_MS, 'error');
      return;
    }
    showNotice($t('notice.loadingPlaylist'), 0, 'info');
    try {
      const playlist = await api<{ title: string; items: { videoId: string }[] }>(
        `/api/youtube/playlist?list=${encodeURIComponent(listId)}`,
      );
      const items = playlist.items.map((item) => ({ videoId: item.videoId, start: 0 }));
      if (!items.length) {
        showNotice($t('notice.playlistEmpty'), ERROR_MS, 'error');
        return;
      }
      if (await command('queue.add', { items }))
        showNotice($t('notice.playlistAdded', { title: playlist.title, count: items.length }), 3500, 'success');
    } catch (e) {
      showNotice(errorText(e), ERROR_MS, 'error');
    }
  }
  // Invite copies the link at once (no extra click) and shows the QR code and
  // short-link options alongside.
  async function invite() {
    inviteOpen = true;
    if (typeof matchMedia === 'function' && matchMedia('(pointer: coarse)').matches) return;
    try {
      await navigator.clipboard.writeText(inviteLink());
      showNotice($t('invite.copied'), 2600, 'success');
    } catch {
      /* the dialog still shows the link */
    }
  }
  async function setSlug(slug: string) {
    const ok = await command('room.slug', { slug });
    if (ok) showNotice(slug ? $t('invite.slugSaved') : $t('invite.slugRemoved'), 2600, 'success');
    return ok;
  }
  async function saveName() {
    const name = nameDraft.trim();
    if (!name) return;
    try {
      await api('/api/account/profile', { method: 'PATCH', body: JSON.stringify({ displayName: name }) });
      updateDisplayName(name);
      try {
        localStorage.setItem('koalaparty.nameChosen', '1');
      } catch {
        /* localStorage unavailable */
      }
      namePrompt = false;
      editingName = false;
      showNotice($t('notice.nameChanged', { name }), 2200, 'success');
    } catch (e) {
      showNotice(errorText(e), ERROR_MS, 'error');
    }
  }
  function startEditingName() {
    nameDraft = me()?.displayName ?? '';
    editingName = true;
  }
  function dismissNamePrompt() {
    namePrompt = false;
    try {
      localStorage.setItem('koalaparty.nameChosen', '1');
    } catch {
      /* localStorage unavailable */
    }
  }
  function moveTo(itemId: string, to: number) {
    if (!room) return;
    const ids = room.queue.map((q) => q.id);
    const from = ids.indexOf(itemId);
    if (from < 0 || to < 0 || to >= ids.length || from === to) return;
    ids.splice(to, 0, ids.splice(from, 1)[0]);
    command('queue.reorder', { itemIds: ids });
  }
  async function removeItem(item: QueueItem) {
    if (!room) return;
    const index = queueIndex(room.queue, item.id);
    if (await command('queue.remove', { itemId: item.id })) {
      showNotice($t('notice.removed', { title: item.media.title }), 5000, 'info', {
        label: $t('common.undo'),
        run: () => void command('queue.add', { videoId: item.media.providerId, start: item.start, position: index }),
      });
    }
  }
  async function playItemNow(item: QueueItem) {
    if (await command('queue.remove', { itemId: item.id }))
      await command('queue.play_now', { videoId: item.media.providerId, start: item.start });
  }
  async function memberAction(member: Member, action: 'kick' | 'ban' | 'role' | 'mute') {
    if (action === 'role')
      await command('member.role', {
        identityId: member.identityId,
        role: member.role === 'admin' ? 'member' : 'admin',
      });
    else if (action === 'mute')
      await command('member.permission', {
        identityId: member.identityId,
        permission: 'chat.send',
        allowed: member.permissions['chat.send'] === false,
      });
    else if (
      await ask(
        action === 'ban'
          ? $t('people.confirmBan', { name: member.displayName })
          : $t('people.confirmKick', { name: member.displayName }),
        action === 'ban' ? $t('people.ban') : $t('people.kick'),
        true,
      )
    )
      await command(`member.${action}`, { identityId: member.identityId });
  }
  async function leaveOrDelete() {
    const owner = me()?.role === 'owner';
    if (
      !(await ask(
        owner ? $t('settings.confirmDelete') : $t('settings.confirmLeave'),
        owner ? $t('settings.delete') : $t('settings.leave'),
        true,
      ))
    )
      return;
    try {
      await api(`/api/rooms/${roomId}${owner ? '' : '/membership'}`, { method: 'DELETE' });
      location.href = '/rooms';
    } catch (e) {
      showNotice(errorText(e), ERROR_MS, 'error');
    }
  }
  async function transfer(member: Member) {
    if (!(await ask($t('settings.confirmTransfer', { name: member.displayName }), $t('settings.transferButton'))))
      return;
    await command('room.transfer', { identityId: member.identityId });
  }
  function toggleSettings() {
    settingsOpen = !settingsOpen;
  }
  function downloadCalendar() {
    if (!room?.scheduledAt) return;
    const url = URL.createObjectURL(
      new Blob([partyCalendar({ title: `KoalaParty · ${room.label}`, url: inviteLink(), start: room.scheduledAt })], {
        type: 'text/calendar;charset=utf-8',
      }),
    );
    const link = document.createElement('a');
    link.href = url;
    link.download = 'koalaparty.ics';
    link.click();
    URL.revokeObjectURL(url);
  }
  // A browser reminder while this tab stays open; the calendar entry covers the rest.
  async function remindMe() {
    if (!room?.scheduledAt || typeof Notification === 'undefined') return;
    const permission =
      Notification.permission === 'default' ? await Notification.requestPermission() : Notification.permission;
    if (permission !== 'granted') {
      showNotice($t('schedule.notificationsBlocked'), ERROR_MS, 'error');
      return;
    }
    if (reminderTimer) clearTimeout(reminderTimer);
    const title = room.label;
    reminderTimer = setTimeout(
      () => new Notification($t('schedule.reminderTitle'), { body: title, icon: '/icons/koalaparty-192.png' }),
      Math.max(0, room.scheduledAt - Date.now() - 60_000),
    );
    reminderSet = true;
    showNotice($t('schedule.reminderSet'), 2600, 'success');
  }
  $: activeMembers = room?.members.filter((member) => member.active) ?? [];
  $: waitingFor = room
    ? activeMembers.filter(
        (member) =>
          member.identityId !== room!.me &&
          (presence[member.identityId] === 'buffering' || presence[member.identityId] === 'blocked'),
      )
    : [];
  $: countdownLeft = countdownEnd ? Math.max(1, Math.ceil((countdownEnd - nowTick) / 1000)) : 0;
  $: syncLabel = !connected
    ? online
      ? $t('sync.reconnecting')
      : $t('sync.offline')
    : !room?.playback.media
      ? $t('sync.ready')
      : countdownEnd
        ? $t('sync.starting')
        : room.playback.autoPaused
          ? $t('sync.waiting')
          : diagnostics.state === 'buffering'
            ? $t('sync.buffering')
            : diagnostics.state === 'live'
              ? $t('sync.live')
              : Math.abs(diagnostics.drift) < 0.6
                ? $t('sync.inSync')
                : $t(diagnostics.drift < 0 ? 'sync.behind' : 'sync.ahead', {
                    seconds: Math.abs(diagnostics.drift).toFixed(1),
                  });
  $: syncNote = diagnostics.correctedAt
    ? `${syncLabel} · ${$t('sync.corrected', { seconds: Math.max(0, Math.round((nowTick - diagnostics.correctedAt) / 1000)) })}`
    : syncLabel;
  $: scheduleLeft = room?.scheduledAt && room.scheduledAt > nowTick ? untilParts(room.scheduledAt - nowTick) : null;
  $: heatBuckets = heat.mediaId && heat.mediaId === room?.playback.media?.id ? heat.buckets : {};
  $: recapStats = {
    roomLabel: room?.label ?? '',
    minutes: Math.max(1, Math.round((nowTick - sessionStart) / 60_000)),
    ...recap,
  } satisfies RecapStats;
</script>

<svelte:head><title>{hiddenUnread ? `(${hiddenUnread}) ` : ''}{room?.label || roomId} · KoalaParty</title></svelte:head>
<svelte:window onkeydown={(e) => confirmDialog && e.key === 'Escape' && resolveConfirm(false)} />
{#if error}<main class="fatal panel">
    <img src="/icons/koalaparty-192.png" alt="" />
    <h1>{$t('room.cannotEnter')}</h1>
    <p class="error">{error}</p>
    <a class="button" href="/">{$t('common.backHome')}</a>
  </main>{:else if !room}<main class="fatal loading" aria-busy="true">
    <div class="spinner" aria-hidden="true"></div>
    <p>{joinAttempt > 0 ? $t('room.reconnectingRoom') : $t('room.joining')}</p>
    {#if joinAttempt > 1}<small class="muted">{$t('room.serverRestarting')}</small>{/if}
  </main>{:else}
  <main class="room-shell" class:theater>
    <RoomHeader
      {room}
      {connected}
      {syncLabel}
      {syncNote}
      {activeMembers}
      {manager}
      {settingsOpen}
      onInvite={invite}
      onToggleSettings={toggleSettings}
      onRename={(name) => command('room.rename', { name })}
      onShowPeople={() => selectTab('people')}
      onSyncNow={syncNow}
      onCopyDiagnostics={copyDiagnostics}
      onDownloadDiagnostics={downloadDiagnostics}
      onShortcuts={() => (shortcutsOpen = true)}
      onRecap={() => (recapOpen = true)}
    />
    {#if settingsOpen}<SettingsPanel
        {room}
        {self}
        {manager}
        {commandPending}
        command={settingsCommand}
        onNotice={quickNotice}
        onLeaveOrDelete={leaveOrDelete}
        onTransfer={transfer}
        onClose={toggleSettings}
      />{/if}
    <section class="room-grid" class:theater>
      <div class="main-column">
        <div class="stage">
          {#if room.playback.media?.thumbnail}{#key room.playback.media.id}<div
                class="ambient"
                aria-hidden="true"
                style={`background-image:url("${room.playback.media.thumbnail}")`}
                transition:fade={{ duration: 900 }}
              ></div>{/key}{/if}
          <div class="player-wrap" class:mini-player={miniPlayer} class:fullscreen bind:this={playerWrap}>
            <YouTubePlayer
              enabled={true}
              videoId={room.playback.media?.providerId}
              mediaId={room.playback.media?.id}
              playbackRevision={room.playback.revision}
              {syncRequest}
              status={countdownEnd ? 'paused' : room.playback.status}
              position={playbackAnchor.position}
              positionAt={playbackAnchor.at}
              rate={playbackAnchor.rate}
              segments={sponsorSegments()}
              canControl={caps['playback.play_pause']}
              canSeek={caps['playback.seek']}
              hasQueue={room.queue.length > 0}
              showEmpty={false}
              onPlay={(pos) => playbackCommand('player.play', { position: pos })}
              onPause={(pos) => playbackCommand('player.pause', { position: pos })}
              onSeek={scheduleSeek}
              onRate={(newRate, pos) => playbackCommand('player.rate', { rate: newRate, position: pos })}
              onSponsorSkip={skipSponsor}
              onEnded={reportEnded}
              onSkip={caps['queue.skip']
                ? (brokenMediaId) => command('queue.skip', { mediaId: brokenMediaId, discardCurrent: true })
                : undefined}
              onDuration={handleDuration}
              onDiagnostics={(value) => (diagnostics = value)}
              onDiagnosticEvent={recordDiagnostic}
              onPresence={reportPresence}
            />
            {#if !room.playback.media}<div class="empty-stage">
                {#if scheduleLeft}<span class="empty-emoji" aria-hidden="true">🗓️</span>
                  <h2>{$t('schedule.startsIn')}</h2>
                  <p class="big-countdown" aria-live="off">
                    {#if scheduleLeft.days}{scheduleLeft.days}d
                    {/if}{String(scheduleLeft.hours).padStart(2, '0')}:{String(scheduleLeft.minutes).padStart(
                      2,
                      '0',
                    )}:{String(scheduleLeft.seconds).padStart(2, '0')}
                  </p>
                  <p>
                    {new Date(room.scheduledAt ?? 0).toLocaleString(undefined, {
                      dateStyle: 'full',
                      timeStyle: 'short',
                    })}
                  </p>
                  <div class="schedule-actions">
                    <button class="secondary" onclick={downloadCalendar}
                      ><CalendarPlus size={16} weight="bold" />{$t('schedule.calendar')}</button
                    >{#if typeof Notification !== 'undefined'}<button
                        class="secondary"
                        disabled={reminderSet}
                        onclick={remindMe}><BellRinging size={16} weight="bold" />{$t('schedule.remind')}</button
                      >{/if}
                  </div>
                  <p class="hint">{room.queue.length ? $t('schedule.autoStart') : $t('schedule.fillQueue')}</p>
                {:else if room.queue.length && caps['queue.skip']}<button
                    class="start"
                    onclick={() => command('queue.skip')}
                    disabled={commandPending}><Play size={18} weight="fill" />{$t('room.playFromQueue')}</button
                  >{:else}
                  <span class="empty-emoji" aria-hidden="true">🍿</span>
                  <h2>{$t('room.startParty')}</h2>
                  <p>{room.searchEnabled ? $t('room.startHintSearch') : $t('room.startHint')}</p>
                  {#if caps['queue.add']}<div class="empty-add">
                      <AddBar
                        variant="hero"
                        canAdd={caps['queue.add']}
                        canPlayNow={caps['media.play_now']}
                        searchEnabled={room.searchEnabled}
                        onAdd={addVideos}
                        onPlaylist={importPlaylist}
                        onPlaylistOffer={offerPlaylist}
                        onError={(message) => showNotice(message, ERROR_MS, 'error')}
                      />
                    </div>{/if}
                  <button class="ghost invite-hint" onclick={invite}
                    ><ShareNetwork size={15} weight="bold" />{$t('room.inviteWhilePicking')}</button
                  >
                {/if}
              </div>{/if}
            {#if countdownEnd}<div class="countdown" aria-live="assertive">
                {#key countdownLeft}<span class="count" in:fly={{ y: -12, duration: 220 }}>{countdownLeft}</span>{/key}
                <small>{room.playback.autoPaused ? $t('countdown.resuming') : $t('countdown.together')}</small>
              </div>{:else if room.playback.autoPaused && room.playback.status === 'paused'}<div
                class="waiting-overlay"
              >
                <span aria-hidden="true">⏳</span>
                <p>
                  {waitingFor.length
                    ? $t('countdown.waitingFor', {
                        names: waitingFor.map((member) => participantNameParts(member.displayName).label).join(', '),
                      })
                    : $t('countdown.waiting')}
                </p>
              </div>{/if}
            {#if splash && room.playback.media && splash === room.playback.media.id && !countdownEnd}<div
                class="splash"
                transition:fly={{ y: 16, duration: 380 }}
              >
                <small>{$t('room.nowPlaying')}</small><b>{room.playback.media.title}</b>
              </div>{/if}
            <div class="reaction-overlay" aria-live="polite">
              {#each reactions as reaction (reaction.id)}<span
                  style={`right:${reaction.x}px`}
                  out:fade={{ duration: 300 }}
                  >{reaction.emoji}{#if reaction.badge}<small aria-hidden="true">{reaction.badge}</small>{/if}</span
                >{/each}
            </div>
            {#if combo}{#key combo.id}<div class="combo" aria-hidden="true">
                  <span>{combo.emoji}</span><b>×{combo.count}</b>
                </div>{/key}{/if}
            {#if fullscreen}<SocialToasts toasts={socialToasts} placement="overlay" />{/if}
            <div class="bubbles" aria-live="polite">
              {#each bubbles as bubble (bubble.id)}<p
                  animate:flip={{ duration: 200 }}
                  in:fly={{ x: -20, duration: 220 }}
                  out:fade={{ duration: 250 }}
                >
                  <span aria-hidden="true">{bubble.badge}</span><b>{bubble.name}</b>
                  {bubble.text}
                </p>{/each}
            </div>
            {#if dropActive}<div class="drop-zone" transition:fade={{ duration: 120 }}>
                <span>{$t('room.dropHere')}</span>
              </div>{/if}
            {#if miniPlayer}<button
                class="mini-close"
                aria-label={$t('player.closeMini')}
                onclick={() => (miniPlayer = false)}><X size={14} weight="bold" /></button
              >{/if}
          </div>
        </div>
        <PlayerBar
          {room}
          {caps}
          {commandPending}
          position={livePosition(nowTick)}
          duration={mediaDuration}
          rate={playbackAnchor.rate}
          playing={room.playback.status === 'playing'}
          heat={heatBuckets}
          {miniPlayer}
          {fullscreen}
          {theater}
          onTogglePlay={togglePlay}
          onSeek={(position) => playbackCommand('player.seek', { position })}
          onRate={(value) => command('player.rate', { rate: value, position: livePosition() })}
          onSkip={() => command('queue.skip', {}, { silentStale: true })}
          onVoteSkip={() => command('queue.vote_skip')}
          onFullscreen={toggleFullscreen}
          onTheater={() => setTheater(!theater)}
          onMiniPlayer={() => (miniPlayer = !miniPlayer)}
        />
        <div class="now-row">
          <div class="now-text">
            {#if room.playback.media}<p class="now-title">
                <small>{$t('room.nowPlaying')}</small><b title={room.playback.media.title}
                  >{room.playback.media.title}</b
                >
              </p>{/if}
            {#if waitingFor.length}<p class="waiting">
                {$t('room.waitingFor', {
                  names: waitingFor.map((member) => participantNameParts(member.displayName).label).join(', '),
                })}
              </p>{:else if room.queue[0]}<p class="up-next">
                <strong>{$t('room.upNext')}</strong>
                {room.queue[0].media.title}
              </p>{/if}
            <p class="player-note">
              {$t('room.playerNote')} · <a href="/privacy">{$t('room.privacyDetails')}</a>
            </p>
          </div>
          <div class="reaction-bar" aria-label={$t('player.react')}>
            {#each REACTION_EMOJIS as emoji, index}<button
                class="ghost"
                title={$t('player.reactKey', { emoji, key: (index + 1) % 10 })}
                onclick={() => react(emoji)}>{emoji}</button
              >{/each}
          </div>
        </div>
      </div>
      <aside class="side-column panel">
        {#if room.scheduledAt && scheduleLeft && room.playback.media}<div class="schedule-banner">
            <span
              >🗓️ {$t('schedule.banner', {
                time: new Date(room.scheduledAt).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' }),
              })}</span
            >
            <button class="ghost small-button" onclick={downloadCalendar} aria-label={$t('schedule.calendar')}
              ><CalendarPlus size={15} weight="bold" /></button
            >
          </div>{/if}
        {#if namePrompt && !editingName}<div class="name-prompt" transition:fly={{ y: -6, duration: 180 }}>
            <span>{$t('name.prompt', { name: participantNameParts(self?.displayName ?? '').label })}</span>
            <button class="secondary small-button" onclick={startEditingName}>{$t('name.set')}</button><button
              class="ghost icon-button"
              aria-label={$t('name.keep')}
              onclick={dismissNamePrompt}><X size={14} weight="bold" /></button
            >
          </div>{/if}
        {#if editingName}<form
            class="name-form"
            onsubmit={(event) => {
              event.preventDefault();
              void saveName();
            }}
          >
            <!-- svelte-ignore a11y_autofocus -->
            <input aria-label={$t('name.label')} bind:value={nameDraft} maxlength="32" autofocus />
            <button>{$t('common.save')}</button><button
              type="button"
              class="ghost"
              onclick={() => (editingName = false)}>{$t('common.cancel')}</button
            >
          </form>{/if}
        <div class="side-add">
          <AddBar
            bind:inputEl={addInput}
            canAdd={caps['queue.add']}
            canPlayNow={caps['media.play_now']}
            searchEnabled={room.searchEnabled}
            onAdd={addVideos}
            onPlaylist={importPlaylist}
            onPlaylistOffer={offerPlaylist}
            onError={(message) => showNotice(message, ERROR_MS, 'error')}
          />
        </div>
        <div class="side-tabs" role="tablist" aria-label={$t('room.details')} tabindex="-1" onkeydown={onTabKeydown}>
          {#each sideTabs as tab}<button
              id={`room-tab-${tab}`}
              role="tab"
              aria-controls={`room-panel-${tab}`}
              aria-selected={sideTab === tab}
              tabindex={sideTab === tab ? 0 : -1}
              class:active={sideTab === tab}
              onclick={() => selectTab(tab)}
              >{$t(`tab.${tab}`)}
              {#if tab === 'queue'}<span>{room.queue.length}</span>{:else if tab === 'people'}<span
                  >{activeMembers.length}</span
                >{:else if tab === 'chat' && unread}<span class="unread">{unread > 9 ? '9+' : unread}</span
                >{/if}</button
            >{/each}
        </div>
        <div
          id="room-panel-queue"
          class="room-panel"
          role="tabpanel"
          aria-labelledby="room-tab-queue"
          hidden={sideTab !== 'queue'}
        >
          <QueuePanel
            {room}
            {caps}
            {commandPending}
            accountLinked={!!self?.accountLinked}
            command={settingsCommand}
            onRemove={removeItem}
            onPlayNow={playItemNow}
            onMoveTo={moveTo}
            onReorder={(itemIds) => command('queue.reorder', { itemIds })}
            onAdd={(videos) => addVideos(videos, 'queue')}
            onNotice={quickNotice}
          />
        </div>
        <div
          id="room-panel-chat"
          class="room-panel chat-panel"
          role="tabpanel"
          aria-labelledby="room-tab-chat"
          hidden={sideTab !== 'chat'}
        >
          <ChatPanel
            messages={chat}
            me={room.me}
            canChat={caps['chat.send']}
            {connected}
            active={sideTab === 'chat'}
            onSend={sendChat}
            onReact={react}
            canAdd={caps['queue.add']}
            canSeek={caps['playback.seek'] && !!room.playback.media}
            onAddLink={(text) => {
              if (!handleExternalText(text)) showNotice($t('add.notYouTube'), ERROR_MS, 'error');
            }}
            onSeek={(seconds) => playbackCommand('player.seek', { position: seconds })}
            bind:inputEl={chatInput}
          />
        </div>
        <div
          id="room-panel-people"
          class="room-panel"
          role="tabpanel"
          aria-labelledby="room-tab-people"
          hidden={sideTab !== 'people'}
        >
          <PeoplePanel
            members={room.members}
            me={room.me}
            {presence}
            {manager}
            {commandPending}
            onMemberAction={memberAction}
            onEditName={startEditingName}
            onInvite={invite}
          />
        </div>
        <div
          id="room-panel-activity"
          role="tabpanel"
          aria-labelledby="room-tab-activity"
          class="room-panel activity"
          hidden={sideTab !== 'activity'}
        >
          <ActivityPanel events={room.events} />
        </div>
      </aside>
    </section>
    {#if !fullscreen}<SocialToasts toasts={socialToasts} />{/if}
    {#if notice}<div
        class="status status--{noticeKind}"
        role={noticeKind === 'error' ? 'alert' : 'status'}
        aria-live={noticeKind === 'error' ? 'assertive' : 'polite'}
        transition:fly={{ y: 12, duration: 220 }}
      >
        {#if noticeKind === 'success'}<CheckCircle
            size={17}
            weight="fill"
          />{:else if noticeKind === 'error'}<WarningCircle size={17} weight="fill" />{:else}<Info
            size={17}
            weight="fill"
          />{/if}<span>{notice}</span>{#if noticeAction}<button
            class="notice-action"
            onclick={() => {
              const action = noticeAction;
              dismissNotice();
              action?.run();
            }}>{noticeAction.label}</button
          >{/if}<button class="ghost notice-close" aria-label={$t('common.dismiss')} onclick={dismissNotice}
          ><X size={13} weight="bold" /></button
        >
      </div>{/if}
    {#if inviteOpen}<InviteDialog
        {room}
        {manager}
        onSetSlug={setSlug}
        onNotice={quickNotice}
        onClose={() => (inviteOpen = false)}
      />{/if}
    {#if shortcutsOpen}<ShortcutsDialog onClose={() => (shortcutsOpen = false)} />{/if}
    {#if recapOpen}<RecapDialog stats={recapStats} onClose={() => (recapOpen = false)} />{/if}
    {#if confirmDialog}<Dialog
        label={confirmDialog.title}
        role="alertdialog"
        closeLabel={$t('common.cancel')}
        onClose={() => resolveConfirm(false)}
      >
        <p>{confirmDialog.title}</p>
        <div class="modal-actions">
          <button class="secondary" onclick={() => resolveConfirm(false)}>{$t('common.cancel')}</button><button
            class={confirmDialog.danger ? 'danger' : ''}
            onclick={() => resolveConfirm(true)}>{confirmDialog.confirmLabel}</button
          >
        </div>
      </Dialog>{/if}
  </main>{/if}

<style>
  .fatal {
    max-width: 620px;
    margin: 6rem auto;
    padding: 3rem;
    text-align: center;
  }
  .fatal > img {
    width: 4.5rem;
    height: 4.5rem;
    object-fit: contain;
  }
  .room-shell {
    max-width: 1560px;
    margin: auto;
    padding: 1rem clamp(0.7rem, 2vw, 2rem) 3rem;
    /* Opacity only: any transform on this shell, even mid-animation, becomes the
       containing block for the fixed toast, dialogs and mini-player. */
    animation: roomReveal 0.35s ease backwards;
  }
  @keyframes roomReveal {
    from {
      opacity: 0;
    }
  }
  /* Layout ---------------------------------------------------------------- */
  .room-grid {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(330px, 380px);
    gap: 1.25rem;
    align-items: start;
  }
  .room-grid.theater {
    grid-template-columns: 1fr;
  }
  .room-grid.theater .main-column {
    width: 100%;
    max-width: min(100%, 150vh);
    margin-inline: auto;
  }
  .main-column {
    display: grid;
    gap: 0.7rem;
    min-width: 0;
  }
  /* Stage: player, ambient glow and overlays -------------------------------- */
  .stage {
    position: relative;
    isolation: isolate;
  }
  .ambient {
    position: absolute;
    inset: -2% -1%;
    z-index: -1;
    background-size: cover;
    background-position: center;
    filter: blur(48px) saturate(1.9) brightness(1.1);
    opacity: 0.7;
    transform: scale(1.06);
    pointer-events: none;
    border-radius: 2rem;
  }
  .player-wrap {
    position: relative;
    border-radius: var(--radius-md);
    box-shadow: 0 22px 48px rgba(3, 12, 8, 0.28);
  }
  .player-wrap.fullscreen {
    border-radius: 0;
    background: #000;
    display: grid;
    place-items: center;
  }
  .player-wrap.fullscreen :global(.player) {
    width: 100%;
    max-height: 100vh;
    border-radius: 0;
  }
  .player-wrap.mini-player {
    position: fixed;
    right: 1rem;
    bottom: 1rem;
    width: min(420px, calc(100vw - 2rem));
    z-index: 20;
    box-shadow: 0 20px 60px rgba(0, 0, 0, 0.5);
  }
  .mini-close {
    position: absolute;
    top: 0.4rem;
    right: 0.4rem;
    z-index: 6;
    padding: 0.3rem;
    border-radius: 999px;
    background: rgba(0, 0, 0, 0.55);
    color: white;
  }
  .empty-stage {
    position: absolute;
    inset: 0;
    z-index: 2;
    display: grid;
    place-content: center;
    justify-items: center;
    gap: 0.35rem;
    padding: 1.2rem;
    text-align: center;
    color: #e7efe9;
    border-radius: var(--radius-md);
    background:
      radial-gradient(circle at 50% 30%, color-mix(in srgb, var(--accent-primary) 30%, transparent), transparent 60%),
      var(--player-background);
  }
  .empty-stage h2 {
    margin: 0;
    font-size: clamp(1.2rem, 2.6vw, 1.8rem);
  }
  .empty-stage p {
    margin: 0 0 0.6rem;
    max-width: 30rem;
    color: #b9c8bf;
    font-size: 0.9rem;
  }
  .big-countdown {
    font-size: clamp(2rem, 7vw, 4rem) !important;
    font-weight: 850;
    color: white !important;
    font-variant-numeric: tabular-nums;
    letter-spacing: -0.02em;
    margin: 0 !important;
  }
  .schedule-actions {
    display: flex;
    gap: 0.5rem;
    flex-wrap: wrap;
    justify-content: center;
  }
  .empty-stage .hint {
    margin-top: 0.6rem;
    font-size: 0.8rem;
  }
  .empty-emoji {
    font-size: clamp(2rem, 5vw, 3.2rem);
    animation: bob 2.4s ease-in-out infinite;
  }
  @keyframes bob {
    50% {
      transform: translateY(-6px);
    }
  }
  .empty-add {
    width: min(34rem, 100%);
  }
  .invite-hint {
    margin-top: 0.4rem;
    color: #cfdcd4;
    font-size: 0.82rem;
  }
  .start {
    font-size: 1.05rem;
    padding: 1rem 1.4rem;
  }
  .countdown,
  .waiting-overlay {
    position: absolute;
    inset: 0;
    z-index: 4;
    display: grid;
    place-content: center;
    justify-items: center;
    gap: 0.4rem;
    color: white;
    background: radial-gradient(circle, rgba(0, 0, 0, 0.35), rgba(0, 0, 0, 0.7));
    border-radius: var(--radius-md);
    pointer-events: none;
  }
  .count {
    font-size: clamp(4rem, 14vw, 9rem);
    font-weight: 900;
    line-height: 1;
    text-shadow: 0 10px 40px color-mix(in srgb, var(--accent-primary) 70%, transparent);
  }
  .countdown small,
  .waiting-overlay p {
    font-weight: 700;
    letter-spacing: 0.02em;
    margin: 0;
  }
  .waiting-overlay span {
    font-size: 2.4rem;
    animation: bob 1.6s ease-in-out infinite;
  }
  .splash {
    position: absolute;
    left: 1rem;
    bottom: 1rem;
    z-index: 4;
    display: grid;
    max-width: min(80%, 34rem);
    padding: 0.6rem 0.9rem;
    border-radius: var(--radius-sm);
    background: rgba(8, 14, 11, 0.72);
    backdrop-filter: blur(8px);
    color: white;
    pointer-events: none;
  }
  .splash small {
    font-size: 0.68rem;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: color-mix(in srgb, var(--accent-primary) 80%, white);
  }
  .splash b {
    font-size: 1rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .reaction-overlay {
    position: absolute;
    right: 1rem;
    bottom: 3.5rem;
    width: 5rem;
    height: 70%;
    z-index: 5;
    pointer-events: none;
  }
  .reaction-overlay span {
    position: absolute;
    bottom: 0;
    font-size: 2.3rem;
    filter: drop-shadow(0 5px 10px rgba(0, 0, 0, 0.4));
    animation: floatUp 2.6s ease-out forwards;
  }
  .reaction-overlay small {
    position: absolute;
    right: -0.45rem;
    bottom: -0.1rem;
    font-size: 0.95rem;
    filter: none;
  }
  @keyframes floatUp {
    0% {
      transform: translateY(0) scale(0.6);
      opacity: 0;
    }
    12% {
      transform: translateY(-10%) scale(1.15);
      opacity: 1;
    }
    100% {
      transform: translateY(-260%) scale(1);
      opacity: 0;
    }
  }
  .combo {
    position: absolute;
    inset: 0;
    z-index: 6;
    display: grid;
    place-content: center;
    grid-auto-flow: column;
    align-items: center;
    gap: 0.3rem;
    pointer-events: none;
    animation: comboPop 1.8s cubic-bezier(0.2, 0.9, 0.3, 1.3) forwards;
  }
  .combo span {
    font-size: clamp(3rem, 9vw, 6.5rem);
    filter: drop-shadow(0 10px 24px rgba(0, 0, 0, 0.45));
  }
  .combo b {
    font-size: clamp(1.6rem, 4vw, 3rem);
    color: white;
    -webkit-text-stroke: 1px rgba(0, 0, 0, 0.35);
    text-shadow: 0 6px 18px rgba(0, 0, 0, 0.5);
  }
  @keyframes comboPop {
    0% {
      transform: scale(0.3);
      opacity: 0;
    }
    18% {
      transform: scale(1.12);
      opacity: 1;
    }
    30% {
      transform: scale(1);
    }
    80% {
      opacity: 1;
    }
    100% {
      transform: scale(1.05) translateY(-12px);
      opacity: 0;
    }
  }
  .bubbles {
    position: absolute;
    left: 0.8rem;
    top: 0.8rem;
    z-index: 5;
    display: grid;
    gap: 0.35rem;
    max-width: min(70%, 26rem);
    pointer-events: none;
  }
  .bubbles p {
    margin: 0;
    padding: 0.4rem 0.7rem;
    border-radius: 1rem;
    background: rgba(8, 14, 11, 0.72);
    backdrop-filter: blur(8px);
    color: white;
    font-size: 0.82rem;
    line-height: 1.35;
    overflow-wrap: anywhere;
    display: -webkit-box;
    -webkit-line-clamp: 3;
    line-clamp: 3;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }
  .bubbles b {
    margin: 0 0.3rem 0 0.2rem;
  }
  .drop-zone {
    position: absolute;
    inset: 0;
    z-index: 7;
    display: grid;
    place-content: center;
    border-radius: var(--radius-md);
    border: 3px dashed var(--accent-primary);
    background: color-mix(in srgb, var(--player-background) 80%, transparent);
    color: white;
    font-weight: 800;
    font-size: 1.2rem;
    pointer-events: none;
  }
  .now-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
  }
  .now-text {
    min-width: 0;
    display: grid;
    gap: 0.1rem;
  }
  .now-title {
    margin: 0;
    display: grid;
    min-width: 0;
  }
  .now-title small {
    font-size: 0.68rem;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }
  .now-title b {
    font-size: 1.05rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .up-next,
  .waiting,
  .player-note {
    margin: 0;
    color: var(--text-muted);
    font-size: 0.78rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .waiting {
    color: var(--warning);
  }
  .player-note {
    font-size: 0.68rem;
  }
  .player-note a {
    color: inherit;
  }
  .reaction-bar {
    display: flex;
    align-items: center;
    gap: 0.05rem;
    flex: 0 0 auto;
  }
  .reaction-bar button {
    font-size: 1.1rem;
    padding: 0.3rem 0.35rem;
    border-radius: 999px;
    transition: transform 0.15s ease;
  }
  .reaction-bar button:hover {
    transform: scale(1.22);
    background: var(--surface-hover);
  }
  /* Side column -------------------------------------------------------------- */
  .side-column {
    position: sticky;
    top: calc(var(--site-header-height, 64px) + 0.75rem);
    height: calc(100vh - var(--site-header-height, 64px) - 1.75rem);
    min-height: 460px;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }
  .room-grid.theater .side-column {
    position: static;
    height: 70vh;
  }
  .schedule-banner,
  .name-prompt,
  .name-form {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    padding: 0.6rem 0.8rem;
    font-size: 0.8rem;
    background: var(--accent-muted);
    border-bottom: 1px solid var(--border-subtle);
  }
  .schedule-banner span,
  .name-prompt span {
    flex: 1;
  }
  .name-form input {
    flex: 1;
    padding: 0.45rem 0.6rem;
  }
  .name-form button {
    padding: 0.45rem 0.7rem;
  }
  .side-add {
    padding: 0.8rem;
    border-bottom: 1px solid var(--border-subtle);
  }
  .side-tabs {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 0.2rem;
    padding: 0.35rem;
    border-bottom: 1px solid var(--border-subtle);
  }
  .side-tabs button {
    background: transparent;
    color: var(--text-secondary);
    padding: 0.5rem 0.2rem;
    font-size: 0.82rem;
    box-shadow: none;
    gap: 0.3rem;
  }
  .side-tabs button:hover {
    transform: none;
    background: var(--surface-hover);
  }
  .side-tabs button.active {
    background: var(--accent-muted);
    color: var(--text-primary);
  }
  .side-tabs span {
    font-size: 0.68rem;
    color: var(--text-muted);
    font-weight: 700;
  }
  .side-tabs .unread {
    color: var(--surface-page);
    background: var(--accent-primary);
    border-radius: 999px;
    padding: 0.05rem 0.4rem;
  }
  .room-panel {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    overscroll-behavior: contain;
  }
  .room-panel[hidden] {
    display: none;
  }
  .chat-panel {
    overflow: hidden;
  }
  /* Toast ------------------------------------------------------------------ */
  .status {
    position: fixed;
    bottom: 1rem;
    left: 50%;
    transform: translateX(-50%);
    background: var(--surface-elevated);
    border: 1px solid var(--border-subtle);
    padding: 0.4rem 0.4rem 0.4rem 0.9rem;
    border-radius: 2rem;
    min-height: 2.3rem;
    font-size: 0.82rem;
    font-weight: 650;
    box-shadow: var(--shadow-panel);
    max-width: min(94vw, 34rem);
    z-index: 70;
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
  .notice-action {
    padding: 0.3rem 0.75rem;
    border-radius: 999px;
    font-size: 0.78rem;
    white-space: nowrap;
  }
  .notice-close {
    padding: 0.3rem;
    border-radius: 999px;
    color: var(--text-muted);
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
  /* Responsive ------------------------------------------------------------- */
  @media (max-width: 900px) {
    .room-grid {
      grid-template-columns: minmax(0, 1fr);
      gap: 0.7rem;
    }
    .room-grid > *,
    .main-column > * {
      min-width: 0;
    }
    /* Let the stage stick to the top of the whole room while the queue and chat
       scroll underneath it, so the video never leaves the screen. */
    .main-column {
      display: contents;
    }
    .stage {
      position: sticky;
      top: var(--site-header-height, 58px);
      z-index: 8;
      margin-inline: calc(-1 * clamp(0.7rem, 2vw, 2rem));
      background: var(--surface-page);
    }
    .ambient {
      display: none;
    }
    .player-wrap:not(.mini-player):not(.fullscreen),
    .player-wrap:not(.mini-player):not(.fullscreen) :global(.player) {
      border-radius: 0;
    }
    .side-column {
      position: static;
      height: auto;
      min-height: 0;
      overflow: visible;
    }
    .chat-panel {
      height: 62vh;
    }
    .reaction-bar {
      display: none;
    }
    .empty-stage p:not(.big-countdown),
    .invite-hint {
      display: none;
    }
  }
  @media (max-width: 700px) {
    .player-wrap.mini-player {
      bottom: calc(4.95rem + env(safe-area-inset-bottom));
    }
    .status {
      bottom: calc(4.95rem + env(safe-area-inset-bottom));
    }
  }
  @media (max-width: 580px) {
    .room-shell {
      padding: 0.5rem 0.7rem 2rem;
    }
    .stage {
      margin-inline: -0.7rem;
    }
    .empty-emoji,
    .empty-stage h2 {
      display: none;
    }
    .empty-stage {
      padding: 0.6rem;
    }
    .side-tabs button {
      font-size: 0.78rem;
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .empty-emoji,
    .reaction-overlay span,
    .combo,
    .waiting-overlay span {
      animation: none;
    }
  }
</style>
