<script lang="ts">
  import { onMount } from 'svelte';
  import { fly, scale, fade } from 'svelte/transition';
  import { flip } from 'svelte/animate';
  import { page } from '$app/state';
  import { api, establish, websocketURL, ApiError } from '$lib/api';
  import { randomUUID, updateDisplayName } from '$lib/identity';
  import { rememberRoom } from '$lib/recentRooms';
  import YouTubePlayer from '$lib/YouTubePlayer.svelte';
  import AddBar from '$lib/room/AddBar.svelte';
  import ChatPanel from '$lib/room/ChatPanel.svelte';
  import {
    automaticEndDelay,
    filterQueue,
    formatActivity,
    formatDuration,
    parseYouTubeInput,
    participantNameParts,
    remainingEndReportLease,
    reconnectDelay,
    SKIPPED_SPONSOR_CATEGORIES,
    SPONSOR_CATEGORY_LABELS,
    type AddMode,
    type ChatMessage,
    type Member,
    type PresenceState,
    type QueueItem,
    type Snapshot,
    type SponsorSegment,
    type VideoRequest,
  } from '$lib/room';
  import { shouldReanchorPlayback } from '$lib/playerSync';
  import { formatDiagnosticEvents, type DiagnosticEvent } from '$lib/diagnostics';
  import {
    Gear,
    X,
    Play,
    Pause,
    SkipForward,
    DotsSixVertical,
    DotsThreeVertical,
    DotsThree,
    CheckCircle,
    WarningCircle,
    Info,
    ArrowsOut,
    ArrowsIn,
    Shuffle,
    Repeat,
    ThumbsUp,
    PictureInPicture,
    ArrowsClockwise,
    DownloadSimple,
    ClipboardText,
    ShareNetwork,
    PencilSimple,
    Keyboard,
    CornersOut,
    CornersIn,
    Hourglass,
    SpeakerSlash,
    ArrowBendDownRight,
    ArrowCounterClockwise,
    CaretUp,
    CaretDown,
    ShieldCheck,
  } from 'phosphor-svelte';
  const roomId = (page.params.roomId ?? '').toUpperCase();
  const ERROR_MS = 6000;
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
  // The embedded YouTube player loads as soon as you enter a room. The privacy
  // details live in the room menu and settings instead of a click-to-consent gate.
  const watching = true;
  let theater = false;
  let miniPlayer = false;
  let fullscreen = false;
  let diagnostics = { drift: 0, state: 'loading', correctedAt: null as number | null };
  let diagnosticEvents: DiagnosticEvent[] = [];
  let online = true;
  let visible = true;
  let reactions: Array<{ id: string; emoji: string; x: number }> = [];
  let bubbles: Array<{ id: string; badge: string; name: string; text: string }> = [];
  let queueQuery = '';
  type SideTab = 'queue' | 'chat' | 'people' | 'activity';
  const sideTabs: SideTab[] = ['queue', 'chat', 'people', 'activity'];
  let sideTab: SideTab = 'queue';
  let dragging: string | null = null;
  let settingsOpen = false;
  let settingsLoading = false;
  let shortcutsOpen = false;
  let invites: { username: string; createdAt: string }[] = [];
  let inviteUsername = '';
  let reportReason = 'spam';
  let reportPending = false;
  let reportSubmitted = false;
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
  let renamingRoom = false;
  let roomNameDraft = '';
  let dropActive = false;
  let dragDepth = 0;
  let addInput: HTMLInputElement | null = null;
  let chatInput: HTMLTextAreaElement | null = null;
  let playerWrap: HTMLDivElement;
  let moreMenu: HTMLDetailsElement;
  let pendingAdd = page.url.searchParams.get('add');
  let confirmDialog: { title: string; confirmLabel: string; danger: boolean; resolve: (ok: boolean) => void } | null =
    null;
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
  let manager = false;
  let caps: Record<string, boolean> = {};
  $: self = room?.members.find((m) => m.identityId === room?.me);
  $: manager = self?.role === 'owner' || self?.role === 'admin';
  $: caps = Object.fromEntries(
    CAPABILITIES.map((cap) => [cap, !!self && (manager || self.permissions[cap] !== false)]),
  );
  // Script code may run right after `room` changes, before reactive values
  // update, so these read the snapshot directly.
  const me = () => room?.members.find((m) => m.identityId === room?.me);
  const manages = () => me()?.role === 'owner' || me()?.role === 'admin';
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
  function announceChanges(previous: Snapshot, next: Snapshot) {
    const nextMedia = next.playback.media?.id ?? '';
    if (nextMedia && nextMedia !== (previous.playback.media?.id ?? '')) {
      if (splashTimer) clearTimeout(splashTimer);
      splash = nextMedia;
      splashTimer = setTimeout(() => (splash = null), 3200);
    }
    const wasActive = new Set(previous.members.filter((m) => m.active).map((m) => m.identityId));
    for (const member of next.members) {
      if (member.active && !wasActive.has(member.identityId) && member.identityId !== next.me) {
        const parts = participantNameParts(member.displayName);
        pushBubble(parts.badge, parts.label, 'joined the party');
      }
    }
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
      playbackAnchor = {
        position: pb.position,
        status: pb.status,
        mediaId,
        rate,
        revision: pb.revision,
        at: Date.now(),
      };
    }
    if (room) announceChanges(room, next);
    room = next;
    updateMediaSession(next);
    updateMediaPosition();
    updateProgressTimer();
  }
  const livePosition = (now = Date.now()) =>
    playbackAnchor.status === 'playing'
      ? playbackAnchor.position + ((now - playbackAnchor.at) / 1000) * playbackAnchor.rate
      : playbackAnchor.position;
  let mediaDuration = 0;
  let nowTick = Date.now();
  function updateProgressTimer() {
    if (progressTimer) clearInterval(progressTimer);
    progressTimer = null;
    nowTick = Date.now();
    if (!visible || playbackAnchor.status !== 'playing' || !playbackAnchor.mediaId) return;
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
  function closeMoreMenu() {
    if (moreMenu) moreMenu.open = false;
  }
  async function copyDiagnostics() {
    closeMoreMenu();
    try {
      await navigator.clipboard.writeText(diagnosticReport());
      showNotice('Diagnostics copied. They stay local until you share them.', 2600, 'success');
    } catch {
      showNotice('Diagnostics could not be copied. Check clipboard permissions.', ERROR_MS, 'error');
    }
  }
  function downloadDiagnostics() {
    closeMoreMenu();
    const url = URL.createObjectURL(new Blob([diagnosticReport()], { type: 'text/plain;charset=utf-8' }));
    const link = document.createElement('a');
    link.href = url;
    link.download = `koalaparty-${roomId.toLowerCase()}-diagnostics.txt`;
    link.click();
    URL.revokeObjectURL(url);
    showNotice('Diagnostics downloaded. They contain local technical state only.', 2600, 'success');
  }
  function syncNow() {
    closeMoreMenu();
    syncRequest += 1;
    showNotice('Realigning your player with the room…', 1600, 'info');
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
  function focusTrap(node: HTMLElement, onEscape: () => void = () => resolveConfirm(false)) {
    const previous = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const controls = () =>
      Array.from(node.querySelectorAll<HTMLElement>('button:not([disabled]), a[href], input, select'));
    const keydown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault();
        onEscape();
        return;
      }
      if (event.key !== 'Tab') return;
      const items = controls();
      if (!items.length) return;
      const first = items[0];
      const last = items[items.length - 1];
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    };
    node.addEventListener('keydown', keydown);
    requestAnimationFrame(() => controls().at(-1)?.focus());
    return {
      destroy() {
        node.removeEventListener('keydown', keydown);
        previous?.focus();
      },
    };
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
  // Menus live inside scrollable, clipped panels. Position them as fixed popovers
  // anchored to their trigger so they are never clipped by neighbouring content.
  function anchoredMenu(details: HTMLDetailsElement) {
    const menu = details.querySelector<HTMLElement>('.menu');
    const reposition = () => {
      if (!menu || !details.open) return;
      const rect = details.getBoundingClientRect();
      menu.style.top = `${rect.bottom + 4}px`;
      menu.style.right = `${window.innerWidth - rect.right}px`;
    };
    const closeOutside = (event: MouseEvent) => {
      if (details.open && !details.contains(event.target as Node)) details.open = false;
    };
    details.addEventListener('toggle', reposition);
    window.addEventListener('scroll', reposition, true);
    window.addEventListener('resize', reposition);
    document.addEventListener('click', closeOutside);
    return {
      destroy() {
        details.removeEventListener('toggle', reposition);
        window.removeEventListener('scroll', reposition, true);
        window.removeEventListener('resize', reposition);
        document.removeEventListener('click', closeOutside);
      },
    };
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
    showNotice(`${SPONSOR_CATEGORY_LABELS[segment.category] ?? 'Segment'} skipped · SponsorBlock`, 2200, 'info');
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
          ? 'Room created — invite link copied. Paste a YouTube link to start!'
          : 'Room created — paste a YouTube link to start, then invite friends.',
        4500,
        'success',
      );
    } catch {
      /* sessionStorage unavailable */
    }
  }
  function prepareNamePrompt() {
    const self = me();
    if (!self || self.accountLinked) return;
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
            error = e instanceof Error ? e.message : 'Could not join room.';
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
    if (!handleExternalText(text)) showNotice('The shared link is not a YouTube video.', ERROR_MS, 'error');
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
      showNotice('Fullscreen is not available here.', 2500, 'error');
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
  let reactionTimers: ReturnType<typeof setTimeout>[] = [];
  onMount(() => {
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
      showNotice('You are offline. Changes will resume after reconnecting.', 0, 'error');
    };
    const onVisibility = () => {
      visible = document.visibilityState === 'visible';
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
      if (isTyping(event.target) || confirmDialog || shortcutsOpen) return;
      const onControl = event.target instanceof HTMLElement && !!event.target.closest('button, a, summary');
      const key = event.key.toLowerCase();
      const media = room.playback.media;
      if ((key === 'k' || (key === ' ' && !onControl)) && media && can('playback.play_pause')) {
        event.preventDefault();
        void command(room.playback.status === 'playing' ? 'player.pause' : 'player.play', {
          position: livePosition(),
        });
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
      !dragging &&
      !!event.dataTransfer &&
      (event.dataTransfer.types.includes('text/uri-list') || event.dataTransfer.types.includes('text/plain'));
    const onDragEnter = (event: DragEvent) => {
      if (!carriesText(event) || !can('queue.add')) return;
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
      if (!handleExternalText(text)) showNotice('Drop a YouTube link to add it to the queue.', 3000, 'error');
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
      if (room?.playback.media && can('playback.play_pause')) {
        void command('player.play', { position: livePosition() });
      }
    });
    setMediaAction('pause', () => {
      if (room?.playback.media && can('playback.play_pause')) {
        void command('player.pause', { position: livePosition() });
      }
    });
    setMediaAction('seekbackward', (details) => {
      if (room?.playback.media && can('playback.seek')) {
        void command('player.seek', { position: Math.max(0, livePosition() - (details.seekOffset ?? 10)) });
      }
    });
    setMediaAction('seekforward', (details) => {
      if (room?.playback.media && can('playback.seek')) {
        void command('player.seek', { position: Math.max(0, livePosition() + (details.seekOffset ?? 10)) });
      }
    });
    setMediaAction('seekto', (details) => {
      if (room?.playback.media && can('playback.seek') && details.seekTime !== undefined) {
        void command('player.seek', { position: Math.max(0, details.seekTime) });
      }
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
      if (everConnected) showNotice('Reconnected', 1800, 'success');
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
      showNotice('Connection lost. Reconnecting…', 0, 'error');
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
          const reaction = { id: randomUUID(), emoji: String(data.emoji), x: Math.round(Math.random() * 60) };
          reactions = [...reactions, reaction].slice(-24);
          const timer = setTimeout(() => {
            reactions = reactions.filter((item) => item.id !== reaction.id);
            reactionTimers = reactionTimers.filter((t) => t !== timer);
          }, 2600);
          reactionTimers.push(timer);
        } else if (data.type === 'chat.history') {
          chat = Array.isArray(data.messages) ? data.messages : [];
        } else if (data.type === 'chat') {
          receiveChat(data.message as ChatMessage);
        } else if (data.type === 'presence.all') {
          presence = data.states && typeof data.states === 'object' ? data.states : {};
        } else if (data.type === 'presence') {
          presence = { ...presence, [String(data.identityId)]: data.state as PresenceState };
        } else if (data.type === 'error') {
          recordDiagnostic({
            at: new Date().toISOString(),
            source: 'websocket',
            event: 'command_error',
            details: { code: typeof data.code === 'string' ? data.code : null },
          });
          showNotice(data.message || 'The server denied that action.', ERROR_MS, 'error');
        }
      } catch {
        recordDiagnostic({ at: new Date().toISOString(), source: 'websocket', event: 'invalid_message', details: {} });
        showNotice('Received an invalid room update. Reconnecting…', 0, 'error');
        ws.close();
      }
    };
  }
  function receiveChat(message: ChatMessage) {
    chat = [...chat, message].slice(-200);
    const chatVisible = sideTab === 'chat' && !theater && !fullscreen && !miniPlayer;
    if (sideTab !== 'chat') unread += 1;
    if (!chatVisible && message.identityId !== room?.me) {
      const parts = participantNameParts(message.name);
      pushBubble(parts.badge, parts.label, message.text);
    }
  }
  function sendChat(text: string): boolean {
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      showNotice('Not connected — your message was not sent.', 3000, 'error');
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
      showNotice(e instanceof Error ? e.message : 'Action failed.', ERROR_MS, 'error');
      return false;
    } finally {
      if (managePending) commandPending = false;
    }
  }

  async function playbackCommand(type: string, payload: Record<string, unknown>) {
    if (!(await command(type, payload, { silentStale: true, bypassPending: true }))) syncRequest += 1;
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
          ? 'Now playing for everyone'
          : videos.length > 1
            ? `Added ${videos.length} videos to the queue`
            : mode === 'next'
              ? 'Queued to play next'
              : 'Added to the queue',
        2200,
        'success',
      );
    }
    return ok;
  }
  function offerPlaylist(listId: string) {
    showNotice('This video is part of a playlist.', 8000, 'info', {
      label: 'Add whole playlist',
      run: () => void importPlaylist(listId),
    });
  }
  async function importPlaylist(listId: string) {
    if (!room?.searchEnabled) {
      showNotice('Playlist import is not enabled on this server — paste single videos instead.', ERROR_MS, 'error');
      return;
    }
    showNotice('Loading playlist…', 0, 'info');
    try {
      const playlist = await api<{ title: string; items: { videoId: string }[] }>(
        `/api/youtube/playlist?list=${encodeURIComponent(listId)}`,
      );
      const items = playlist.items.map((item) => ({ videoId: item.videoId, start: 0 }));
      if (!items.length) {
        showNotice('That playlist has no playable videos.', ERROR_MS, 'error');
        return;
      }
      if (await command('queue.add', { items }))
        showNotice(`Added “${playlist.title}” (${items.length} videos)`, 3500, 'success');
    } catch (e) {
      showNotice(e instanceof Error ? e.message : 'Could not load the playlist.', ERROR_MS, 'error');
    }
  }
  async function invite() {
    const url = `${location.origin}/room/${roomId}`;
    const coarse = typeof matchMedia === 'function' && matchMedia('(pointer: coarse)').matches;
    if (coarse && typeof navigator.share === 'function') {
      try {
        await navigator.share({ title: `Join ${room?.label ?? 'my KoalaParty'}`, text: 'Watch YouTube with me', url });
        return;
      } catch (e) {
        if (e instanceof DOMException && e.name === 'AbortError') return;
      }
    }
    try {
      await navigator.clipboard.writeText(url);
      showNotice('Invite link copied — paste it to your friends.', 2600, 'success');
    } catch {
      showNotice('Could not copy the invite link. Copy it from the address bar.', ERROR_MS, 'error');
    }
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
      showNotice(`You're now ${name}.`, 2200, 'success');
    } catch (e) {
      showNotice(e instanceof Error ? e.message : 'Could not change your name.', ERROR_MS, 'error');
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
  async function saveRoomName() {
    renamingRoom = false;
    if (!room || roomNameDraft.trim() === room.label) return;
    await command('room.rename', { name: roomNameDraft });
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
  function moveTo(itemId: string, to: number) {
    if (!room) return;
    const ids = room.queue.map((q) => q.id);
    const from = ids.indexOf(itemId);
    if (from < 0 || to < 0 || to >= ids.length || from === to) return;
    ids.splice(to, 0, ids.splice(from, 1)[0]);
    command('queue.reorder', { itemIds: ids });
  }
  function closeMenu(event: Event) {
    (event.currentTarget as HTMLElement).closest('details')?.removeAttribute('open');
  }
  async function removeItem(item: QueueItem) {
    if (!room) return;
    const index = queueIndex(room.queue, item.id);
    if (await command('queue.remove', { itemId: item.id })) {
      showNotice(`Removed “${item.media.title}”`, 5000, 'info', {
        label: 'Undo',
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
      showNotice(e instanceof Error ? e.message : 'Could not load invitations.', ERROR_MS, 'error');
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
      showNotice(e instanceof Error ? e.message : 'Could not add invitation.', ERROR_MS, 'error');
    }
  }
  async function revokeInvite(username: string) {
    try {
      await api(`/api/rooms/${roomId}/invites/${encodeURIComponent(username)}`, { method: 'DELETE' });
      invites = invites.filter((invite) => invite.username !== username);
      showNotice('Invitation revoked.', 2200, 'success');
    } catch (e) {
      showNotice(e instanceof Error ? e.message : 'Could not revoke invitation.', ERROR_MS, 'error');
    }
  }
  async function reportRoom() {
    if (reportPending || reportSubmitted) return;
    reportPending = true;
    try {
      await api(`/api/rooms/${roomId}/reports`, {
        method: 'POST',
        body: JSON.stringify({ reason: reportReason }),
      });
      reportSubmitted = true;
      showNotice('Report submitted. Thank you.', 3000, 'success');
    } catch (e) {
      showNotice(e instanceof Error ? e.message : 'Could not submit the report.', ERROR_MS, 'error');
    } finally {
      reportPending = false;
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
      showNotice(e instanceof Error ? e.message : 'Room action failed.', ERROR_MS, 'error');
    }
  }
  async function transfer(member: Member) {
    if (!(await ask(`Transfer ownership to ${member.displayName}? You will become an admin.`, 'Transfer'))) return;
    await command('room.transfer', { identityId: member.identityId });
  }
  function seekFromScrubber(event: Event) {
    const value = Number((event.currentTarget as HTMLInputElement).value);
    if (Number.isFinite(value)) void playbackCommand('player.seek', { position: value });
  }
  function toggleSettings() {
    settingsOpen = !settingsOpen;
    if (settingsOpen) loadInvites();
  }
  $: activeMembers = room?.members.filter((member) => member.active) ?? [];
  $: waitingFor = room
    ? activeMembers.filter(
        (member) =>
          member.identityId !== room!.me &&
          (presence[member.identityId] === 'buffering' || presence[member.identityId] === 'blocked'),
      )
    : [];
  $: syncLabel = !connected
    ? online
      ? 'Reconnecting…'
      : 'Offline'
    : !room?.playback.media
      ? 'Ready'
      : diagnostics.state === 'buffering'
        ? 'Buffering'
        : diagnostics.state === 'live'
          ? 'Live stream'
          : Math.abs(diagnostics.drift) < 0.6
            ? 'In sync'
            : `${Math.abs(diagnostics.drift).toFixed(1)}s ${diagnostics.drift < 0 ? 'behind' : 'ahead'}`;
</script>

<svelte:head><title>{room?.label || roomId} · KoalaParty</title></svelte:head>
<svelte:window onkeydown={(e) => confirmDialog && e.key === 'Escape' && resolveConfirm(false)} />
{#if error}<main class="fatal panel">
    <img src="/icons/koalaparty-192.png" alt="" />
    <h1>Couldn’t enter this room</h1>
    <p class="error">{error}</p>
    <a class="button" href="/">Back home</a>
  </main>{:else if !room}<main class="fatal loading" aria-busy="true">
    <div class="spinner" aria-hidden="true"></div>
    <p>{joinAttempt > 0 ? 'Reconnecting to the room…' : 'Joining room…'}</p>
    {#if joinAttempt > 1}<small class="muted">The server may be restarting — this will retry automatically.</small>{/if}
  </main>{:else}
  <main class="room-shell" class:theater>
    <header class="room-header">
      <div class="room-title">
        <div class="title-line">
          {#if renamingRoom}<form
              class="rename-form"
              onsubmit={(event) => {
                event.preventDefault();
                void saveRoomName();
              }}
            >
              <!-- svelte-ignore a11y_autofocus -->
              <input
                aria-label="Room name"
                bind:value={roomNameDraft}
                maxlength="60"
                autofocus
                placeholder="Name this room"
                onblur={saveRoomName}
                onkeydown={(event) => event.key === 'Escape' && (renamingRoom = false)}
              />
            </form>{:else}<h1>{room.label}</h1>
            {#if manager}<button
                class="ghost icon-button"
                aria-label="Rename room"
                title="Rename room"
                onclick={() => {
                  roomNameDraft = room!.label;
                  renamingRoom = true;
                }}><PencilSimple size={16} weight="bold" /></button
              >{/if}{/if}
        </div>
        <div class="room-meta">
          <span class:offline={!connected} class="connection" role="status">{connected ? 'Live' : 'Reconnecting'}</span
          ><span class="sync-pill" title="Difference between your player and the shared room clock">{syncLabel}</span
          ><span class="visibility">{room.visibility.replace('_', '-')}</span>
          <button
            class="ghost avatars"
            aria-label={`${activeMembers.length} watching — show people`}
            onclick={() => selectTab('people')}
            >{#each activeMembers.slice(0, 5) as member (member.identityId)}<span
                class="avatar-chip"
                title={member.displayName}>{participantNameParts(member.displayName).badge}</span
              >{/each}<span class="avatar-count">{activeMembers.length} watching</span></button
          >
        </div>
      </div>
      <div class="room-actions">
        <button class="invite-button" onclick={invite}><ShareNetwork size={17} weight="bold" />Invite</button>
        <button
          class="secondary icon-button"
          aria-label="Room settings"
          title="Room settings"
          aria-controls="room-settings"
          aria-expanded={settingsOpen}
          class:active={settingsOpen}
          onclick={toggleSettings}><Gear size={18} weight="bold" /></button
        >
        <details class="more" bind:this={moreMenu} use:anchoredMenu>
          <summary class="secondary icon-button" aria-label="More room options" title="More"
            ><DotsThree size={20} weight="bold" /></summary
          >
          <div class="menu wide">
            <button class="ghost" onclick={syncNow}><ArrowsClockwise size={16} weight="bold" />Sync now</button>
            <button class="ghost" onclick={copyDiagnostics}
              ><ClipboardText size={16} weight="bold" />Copy diagnostics</button
            >
            <button class="ghost" onclick={downloadDiagnostics}
              ><DownloadSimple size={16} weight="bold" />Download diagnostics</button
            >
            <button
              class="ghost"
              onclick={() => {
                closeMoreMenu();
                shortcutsOpen = true;
              }}><Keyboard size={16} weight="bold" />Keyboard shortcuts</button
            >
            <a class="menu-link" href="/privacy"><ShieldCheck size={16} weight="bold" />Privacy details</a>
            <p class="menu-note">
              {syncLabel}{diagnostics.correctedAt
                ? ` · corrected ${Math.max(0, Math.round((nowTick - diagnostics.correctedAt) / 1000))}s ago`
                : ''}
            </p>
          </div>
        </details>
      </div>
    </header>
    {#if settingsOpen}<section
        id="room-settings"
        class="settings panel"
        aria-label="Room settings"
        transition:fly={{ y: -8, duration: 180 }}
      >
        <header class="settings-head">
          <h2>Room settings</h2>
          <button class="ghost icon-button" aria-label="Close settings" onclick={toggleSettings}
            ><X size={16} weight="bold" /></button
          >
        </header>
        <div class="settings-grid">
          <div>
            <h2>Access</h2>
            <p class="muted">Choose who can enter this room. Invite lists apply to private rooms.</p>
            {#if manager}<label
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
            <p class="muted small">
              Opening a room loads YouTube's privacy-enhanced player from youtube-nocookie.com. Chat is never stored. <a
                href="/privacy">Privacy details</a
              >
            </p>
          </div>
          {#if manager}<div>
              <h2>SponsorBlock</h2>
              <p class="muted">
                Automatically skip sponsor, intro and outro segments for everyone, in sync. Segment data from
                <a href="https://sponsor.ajay.app" target="_blank" rel="noopener noreferrer">SponsorBlock</a> (CC BY-NC-SA
                4.0).
              </p>
              <label class="toggle">
                <input
                  type="checkbox"
                  checked={room.sponsorBlock}
                  onchange={(e) => command('room.sponsorblock', { enabled: e.currentTarget.checked })}
                /><span>Skip sponsor segments automatically</span>
              </label>
            </div>{/if}
          {#if manager}<div>
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
          {#if room.visibility === 'public'}<div>
              <h2>Report room</h2>
              <p class="muted">Send this public room to the instance administrators for review.</p>
              {#if reportSubmitted}<p role="status">Report submitted. Thank you.</p>{:else}<form
                  class="report-form"
                  onsubmit={(event) => {
                    event.preventDefault();
                    reportRoom();
                  }}
                >
                  <label
                    >Reason<select bind:value={reportReason}>
                      <option value="spam">Spam or misleading</option>
                      <option value="illegal_content">Illegal content</option>
                      <option value="sexual_content">Sexual content</option>
                      <option value="violent_content">Violent content</option>
                      <option value="harassment">Harassment</option>
                      <option value="other">Other</option>
                    </select></label
                  ><button disabled={reportPending}>{reportPending ? 'Submitting…' : 'Submit report'}</button>
                </form>{/if}
            </div>{/if}
          {#if self?.role === 'owner'}<div>
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
            <h2>{self?.role === 'owner' ? 'Delete room' : 'Leave room'}</h2>
            <p class="muted">
              {self?.role === 'owner'
                ? 'Permanently closes the room for every participant.'
                : 'Removes this room from your account.'}
            </p>
            <button class="danger" onclick={leaveOrDelete}
              >{self?.role === 'owner' ? 'Delete room' : 'Leave room'}</button
            >
          </div>
        </div>
      </section>{/if}
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
              enabled={watching}
              videoId={room.playback.media?.providerId}
              mediaId={room.playback.media?.id}
              playbackRevision={room.playback.revision}
              {syncRequest}
              status={room.playback.status}
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
                {#if room.queue.length && caps['queue.skip']}<button
                    class="start"
                    onclick={() => command('queue.skip')}
                    disabled={commandPending}><Play size={18} weight="fill" />Play from queue</button
                  >{:else}
                  <span class="empty-emoji" aria-hidden="true">🍿</span>
                  <h2>Start the party</h2>
                  <p>
                    Paste a YouTube link anywhere on this page, drop one here, or use the box{room.searchEnabled
                      ? ' — or type to search YouTube'
                      : ''}.
                  </p>
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
                    ><ShareNetwork size={15} weight="bold" />Invite friends while you pick</button
                  >
                {/if}
              </div>{/if}
            {#if splash && room.playback.media && splash === room.playback.media.id}<div
                class="splash"
                transition:fly={{ y: 16, duration: 380 }}
              >
                <small>Now playing</small><b>{room.playback.media.title}</b>
              </div>{/if}
            <div class="reaction-overlay" aria-live="polite">
              {#each reactions as reaction (reaction.id)}<span
                  style={`right:${reaction.x}px`}
                  out:fade={{ duration: 300 }}>{reaction.emoji}</span
                >{/each}
            </div>
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
                <span>Drop to add to the queue</span>
              </div>{/if}
            {#if miniPlayer}<button
                class="mini-close"
                aria-label="Close mini-player"
                onclick={() => (miniPlayer = false)}><X size={14} weight="bold" /></button
              >{/if}
          </div>
        </div>
        <div class="player-bar">
          <button
            class="play-toggle"
            aria-label={room.playback.status === 'playing' ? 'Pause' : 'Play'}
            onclick={() =>
              command(room!.playback.status === 'playing' ? 'player.pause' : 'player.play', {
                position: livePosition(),
              })}
            disabled={commandPending || !caps['playback.play_pause'] || !room.playback.media}
            >{#if room.playback.status === 'playing'}<Pause size={18} weight="fill" />{:else}<Play
                size={18}
                weight="fill"
              />{/if}</button
          >
          <div class="scrubber-area">
            {#if room.playback.media}{@const pos = livePosition(nowTick)}{@const pct =
                mediaDuration > 0 ? Math.min(100, (pos / mediaDuration) * 100) : 0}
              <div
                class="scrubber"
                role="progressbar"
                aria-label="Playback progress"
                aria-valuemin="0"
                aria-valuemax={Math.max(1, Math.round(mediaDuration || pos || 1))}
                aria-valuenow={Math.min(
                  Math.max(0, Math.round(pos)),
                  Math.max(1, Math.round(mediaDuration || pos || 1)),
                )}
                aria-valuetext={mediaDuration > 0
                  ? `${formatDuration(pos)} of ${formatDuration(mediaDuration)}`
                  : formatDuration(pos)}
              >
                <div class="scrubber-track">
                  <div class="scrubber-fill" style="width:{pct}%"></div>
                  {#if caps['playback.seek'] && mediaDuration > 0}<input
                      class="scrubber-input"
                      type="range"
                      min="0"
                      max={Math.floor(mediaDuration)}
                      step="1"
                      value={Math.floor(Math.min(pos, mediaDuration))}
                      aria-label="Seek for everyone"
                      onchange={seekFromScrubber}
                    />{/if}
                </div>
                <div class="scrubber-time">
                  <span>{formatDuration(pos)}</span><span
                    >{mediaDuration > 0 ? formatDuration(mediaDuration) : '–:--'}</span
                  >
                </div>
              </div>{:else}<p class="idle-note">Nothing playing yet</p>{/if}
          </div>
          {#if room.playback.media && !miniPlayer}<select
              class="speed-control"
              aria-label="Playback speed"
              title="Playback speed — synced for everyone"
              value={playbackAnchor.rate}
              disabled={commandPending || !caps['playback.play_pause']}
              onchange={(e) =>
                command('player.rate', { rate: Number(e.currentTarget.value), position: livePosition() })}
              >{#each [0.25, 0.5, 0.75, 1, 1.25, 1.5, 1.75, 2] as r}<option value={r}>{r === 1 ? '1×' : `${r}×`}</option
                >{/each}</select
            >{/if}
          {#if room.playback.media}{#if caps['queue.skip']}<button
                class="secondary bar-button"
                aria-label="Skip to next video"
                title="Skip to the next video"
                onclick={() => command('queue.skip', {}, { silentStale: true })}
                disabled={commandPending}
                ><SkipForward size={17} weight="fill" /><span class="bar-label">Skip</span></button
              >{:else if caps['queue.vote']}<button
                class="secondary bar-button"
                class:active={room.playback.skipVoted}
                aria-pressed={room.playback.skipVoted}
                title="Skip when most people vote"
                onclick={() => command('queue.vote_skip')}
                ><SkipForward size={17} weight="fill" /><span
                  >Vote skip {room.playback.skipVotes}/{room.playback.skipNeeded}</span
                ></button
              >{/if}{/if}
          <button
            class="secondary bar-button"
            aria-label={fullscreen ? 'Exit fullscreen' : 'Fullscreen'}
            title="Fullscreen with reactions (F)"
            onclick={toggleFullscreen}
            >{#if fullscreen}<CornersIn size={17} weight="bold" />{:else}<CornersOut
                size={17}
                weight="bold"
              />{/if}</button
          ><button
            class="secondary bar-button theater-toggle"
            aria-pressed={theater}
            aria-label={theater ? 'Exit theater mode' : 'Theater mode'}
            title={theater ? 'Exit theater mode (T)' : 'Theater mode (T)'}
            onclick={() => setTheater(!theater)}
            >{#if theater}<ArrowsIn size={17} weight="bold" />{:else}<ArrowsOut size={17} weight="bold" />{/if}</button
          ><button
            class="secondary bar-button"
            aria-pressed={miniPlayer}
            aria-label={miniPlayer ? 'Dock player' : 'Float mini-player'}
            title={miniPlayer ? 'Dock player (M)' : 'Float mini-player (M)'}
            onclick={() => (miniPlayer = !miniPlayer)}><PictureInPicture size={17} weight="bold" /></button
          >
        </div>
        <div class="now-row">
          <div class="now-text">
            {#if room.playback.media}<p class="now-title">
                <small>Now playing</small><b title={room.playback.media.title}>{room.playback.media.title}</b>
              </p>{/if}
            {#if waitingFor.length}<p class="waiting">
                <Hourglass size={13} weight="bold" />Waiting for {waitingFor
                  .map((member) => participantNameParts(member.displayName).label)
                  .join(', ')}
              </p>{:else if room.queue[0]}<p class="up-next">
                <strong>Up next:</strong>
                {room.queue[0].media.title}
              </p>{/if}
            <p class="player-note">
              Player by YouTube (privacy-enhanced, youtube-nocookie.com) · <a href="/privacy">Privacy details</a>
            </p>
          </div>
          <div class="reaction-bar" aria-label="Send a reaction">
            {#each ['❤️', '😂', '🔥', '👀', '😴', '👏'] as emoji}<button class="ghost" onclick={() => react(emoji)}
                >{emoji}</button
              >{/each}
          </div>
        </div>
      </div>
      <aside class="side-column panel">
        {#if namePrompt && !editingName}<div class="name-prompt" transition:fly={{ y: -6, duration: 180 }}>
            <span
              >You're <b>{participantNameParts(self?.displayName ?? '').label}</b>. Let friends know who you are.</span
            >
            <button class="secondary small-button" onclick={startEditingName}>Set name</button><button
              class="ghost icon-button"
              aria-label="Keep this name"
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
            <input aria-label="Your name" bind:value={nameDraft} maxlength="32" autofocus />
            <button>Save</button><button type="button" class="ghost" onclick={() => (editingName = false)}
              >Cancel</button
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
        <div class="side-tabs" role="tablist" aria-label="Room details" tabindex="-1" onkeydown={onTabKeydown}>
          {#each sideTabs as tab}<button
              id={`room-tab-${tab}`}
              role="tab"
              aria-controls={`room-panel-${tab}`}
              aria-selected={sideTab === tab}
              tabindex={sideTab === tab ? 0 : -1}
              class:active={sideTab === tab}
              onclick={() => selectTab(tab)}
              >{tab === 'queue' ? 'Queue' : tab === 'chat' ? 'Chat' : tab === 'people' ? 'People' : 'Activity'}
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
          <header>
            <h2>Queue</h2>
            <div class="queue-tools">
              <button
                class="ghost"
                title="Shuffle queue"
                aria-label="Shuffle queue"
                onclick={() => command('queue.shuffle')}
                disabled={commandPending || room.queue.length < 2 || !caps['queue.reorder']}
                ><Shuffle size={15} weight="bold" /></button
              ><button
                class="ghost"
                class:active={room.queueLoop}
                title="Loop played videos"
                aria-label="Loop queue"
                aria-pressed={room.queueLoop}
                onclick={() => command('queue.loop', { enabled: !room!.queueLoop })}
                disabled={!caps['queue.reorder']}><Repeat size={15} weight="bold" /></button
              >
            </div>
          </header>
          {#if room.queue.length > 4}<label class="queue-search">
              <span>Search queue</span>
              <input bind:value={queueQuery} type="search" placeholder="Title or video ID" />
            </label>{/if}
          {#if !room.queue.length}<div class="empty">
              <span>🎋</span>
              <p>The queue is empty.<br />Paste YouTube links above — or anywhere with Ctrl+V.</p>
            </div>{:else if !filterQueue(room.queue, queueQuery).length}<div class="empty">
              <span>🔎</span>
              <p>No queued video matches “{queueQuery}”.</p>
            </div>{:else}<ol class="queue">
              {#each filterQueue(room.queue, queueQuery) as item (item.id)}{@const i = queueIndex(room.queue, item.id)}
                <li
                  animate:flip={{ duration: 260 }}
                  draggable={!queueQuery && !commandPending && caps['queue.reorder']}
                  ondragstart={() => (dragging = item.id)}
                  ondragend={() => (dragging = null)}
                  ondragover={(e) => e.preventDefault()}
                  ondrop={() => drop(item.id)}
                >
                  {#if caps['queue.reorder']}<span class="handle" aria-hidden="true"
                      ><DotsSixVertical size={16} weight="bold" /></span
                    >{/if}{#if watching}<img src={item.media.thumbnail} alt="" loading="lazy" />{:else}<span
                      class="thumbnail-placeholder"
                      aria-hidden="true"><Play size={16} weight="fill" /></span
                    >{/if}
                  <div>
                    <b title={item.media.title}>{item.media.title}</b><small
                      >{i + 1}{item.addedBy ? ` · ${participantNameParts(item.addedBy).label}` : ''}{item.start
                        ? ` · from ${formatDuration(item.start)}`
                        : ''}</small
                    >
                  </div>
                  <button
                    class="ghost vote"
                    class:active={item.voted}
                    aria-label={`Vote for ${item.media.title}`}
                    title="Vote — most-voted plays first"
                    onclick={() => command('queue.vote', { itemId: item.id })}
                    disabled={!caps['queue.vote']}
                    ><ThumbsUp size={14} weight={item.voted ? 'fill' : 'bold'} />{item.votes}</button
                  >
                  <details class="item-menu" use:anchoredMenu>
                    <summary aria-label={`More actions for ${item.media.title}`}
                      ><DotsThreeVertical size={16} weight="bold" /></summary
                    >
                    <div class="menu">
                      <button
                        class="ghost"
                        disabled={!caps['media.play_now'] || !caps['queue.remove']}
                        onclick={(event) => {
                          closeMenu(event);
                          void playItemNow(item);
                        }}><Play size={14} weight="fill" />Play now</button
                      ><button
                        class="ghost"
                        disabled={!caps['queue.reorder'] || i === 0}
                        onclick={(event) => {
                          closeMenu(event);
                          moveTo(item.id, 0);
                        }}><ArrowBendDownRight size={14} weight="bold" />Play next</button
                      ><button
                        class="ghost"
                        aria-label={`Move ${item.media.title} up`}
                        disabled={!caps['queue.reorder'] || i === 0}
                        onclick={(event) => {
                          closeMenu(event);
                          moveTo(item.id, i - 1);
                        }}><CaretUp size={14} weight="bold" />Move up</button
                      ><button
                        class="ghost"
                        aria-label={`Move ${item.media.title} down`}
                        disabled={!caps['queue.reorder'] || i === room.queue.length - 1}
                        onclick={(event) => {
                          closeMenu(event);
                          moveTo(item.id, i + 1);
                        }}><CaretDown size={14} weight="bold" />Move down</button
                      >
                    </div>
                  </details>
                  <button
                    class="ghost icon"
                    aria-label={`Remove ${item.media.title}`}
                    onclick={() => removeItem(item)}
                    disabled={!caps['queue.remove']}><X size={16} weight="bold" /></button
                  >
                </li>{/each}
            </ol>{/if}
          {#if room.history.length}<details class="history">
              <summary>Recently played ({room.history.length})</summary>
              <ul>
                {#each room.history as item, index (`${item.id}-${index}`)}<li>
                    <span title={item.title}>{item.title}</span>{#if caps['queue.add']}<button
                        class="ghost"
                        aria-label={`Add ${item.title} again`}
                        title="Add again"
                        onclick={() => addVideos([{ videoId: item.providerId, start: 0 }], 'queue')}
                        ><ArrowCounterClockwise size={14} weight="bold" /></button
                      >{/if}
                  </li>{/each}
              </ul>
            </details>{/if}
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
          <header>
            <h2>Participants</h2>
            <span class="online-count"><span class="online-dot"></span>{activeMembers.length} online</span>
          </header>
          <ul class="members">
            {#each room.members as member (member.identityId)}{@const parts = participantNameParts(member.displayName)}
              {@const state = member.active ? presence[member.identityId] : undefined}
              <li>
                <div class="avatar" class:offline={!member.active}>
                  <span aria-hidden="true">{parts.badge}</span><span
                    class="presence"
                    title={member.active ? 'Online' : 'Offline'}
                  ></span>
                </div>
                <div>
                  <b>{parts.label}{member.identityId === room.me ? ' (you)' : ''}</b><small
                    >{member.role}{#if state === 'buffering'}<span class="state"
                        ><Hourglass size={11} weight="bold" /> buffering</span
                      >{:else if state === 'blocked'}<span class="state"
                        ><SpeakerSlash size={11} weight="bold" /> needs to tap for sound</span
                      >{/if}{#if member.permissions['chat.send'] === false}<span class="state">
                        · muted</span
                      >{/if}</small
                  >
                </div>
                {#if member.identityId === room.me}<button
                    class="ghost small-button"
                    onclick={startEditingName}
                    aria-label="Change your name"><PencilSimple size={14} weight="bold" /></button
                  >{/if}
                {#if manager && member.role !== 'owner' && member.identityId !== room.me}<details use:anchoredMenu>
                    <summary aria-label={`Manage ${member.displayName}`}
                      ><DotsThreeVertical size={18} weight="bold" /></summary
                    >
                    <div class="menu">
                      <button class="ghost" disabled={commandPending} onclick={() => memberAction(member, 'role')}
                        >{member.role === 'admin' ? 'Make member' : 'Make admin'}</button
                      >{#if member.role === 'member'}<button
                          class="ghost"
                          disabled={commandPending}
                          onclick={() => memberAction(member, 'mute')}
                          >{member.permissions['chat.send'] === false ? 'Unmute chat' : 'Mute in chat'}</button
                        >{/if}<button
                        class="ghost"
                        disabled={commandPending}
                        onclick={() => memberAction(member, 'kick')}>Kick</button
                      ><button class="danger" disabled={commandPending} onclick={() => memberAction(member, 'ban')}
                        >Ban</button
                      >
                    </div>
                  </details>{/if}
              </li>{/each}
          </ul>
          <button class="secondary invite-wide" onclick={invite}
            ><ShareNetwork size={16} weight="bold" />Invite more people</button
          >
        </div>
        <div
          id="room-panel-activity"
          role="tabpanel"
          aria-labelledby="room-tab-activity"
          class="room-panel activity"
          hidden={sideTab !== 'activity'}
        >
          {@render Activity(room.events)}
        </div>
      </aside>
    </section>
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
          >{/if}<button class="ghost notice-close" aria-label="Dismiss" onclick={dismissNotice}
          ><X size={13} weight="bold" /></button
        >
      </div>{/if}
    {#if shortcutsOpen}<div class="modal-backdrop">
        <button
          class="modal-scrim"
          aria-label="Close shortcuts"
          onclick={() => (shortcutsOpen = false)}
          transition:fade={{ duration: 160 }}
        ></button>
        <div
          class="modal panel shortcuts"
          role="dialog"
          aria-modal="true"
          aria-label="Keyboard shortcuts"
          use:focusTrap={() => (shortcutsOpen = false)}
          transition:scale={{ start: 0.94, duration: 180 }}
        >
          <h2>Keyboard shortcuts</h2>
          <dl>
            <dt><kbd>Ctrl</kbd>+<kbd>V</kbd></dt>
            <dd>Paste a YouTube link anywhere to queue it</dd>
            <dt><kbd>K</kbd> / <kbd>Space</kbd></dt>
            <dd>Play or pause for everyone</dd>
            <dt><kbd>←</kbd> <kbd>→</kbd></dt>
            <dd>Jump 5 seconds</dd>
            <dt><kbd>/</kbd> or <kbd>A</kbd></dt>
            <dd>Focus the add box</dd>
            <dt><kbd>C</kbd></dt>
            <dd>Open chat</dd>
            <dt><kbd>F</kbd> · <kbd>T</kbd> · <kbd>M</kbd></dt>
            <dd>Fullscreen · theater · mini-player</dd>
            <dt><kbd>Shift</kbd>+<kbd>Enter</kbd></dt>
            <dd>In the add box: play now for everyone</dd>
          </dl>
          <div class="modal-actions"><button onclick={() => (shortcutsOpen = false)}>Got it</button></div>
        </div>
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
          use:focusTrap
          transition:scale={{ start: 0.94, duration: 180 }}
        >
          <p>{confirmDialog.title}</p>
          <div class="modal-actions">
            <button class="secondary" onclick={() => resolveConfirm(false)}>Cancel</button><button
              class={confirmDialog.danger ? 'danger' : ''}
              onclick={() => resolveConfirm(true)}>{confirmDialog.confirmLabel}</button
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
  /* Header ---------------------------------------------------------------- */
  .room-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    margin-bottom: 0.9rem;
  }
  .room-title {
    display: grid;
    gap: 0.35rem;
    min-width: 0;
  }
  .title-line {
    display: flex;
    align-items: center;
    gap: 0.3rem;
    min-width: 0;
  }
  .room-header h1 {
    font-size: 1.35rem;
    margin: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    min-width: 0;
  }
  .rename-form input {
    font-size: 1.1rem;
    font-weight: 750;
    padding: 0.35rem 0.6rem;
    min-width: 16rem;
  }
  .room-meta {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 0.4rem;
  }
  .icon-button {
    padding: 0.5rem;
    border-radius: var(--radius-sm);
  }
  .connection,
  .visibility,
  .sync-pill {
    font-size: 0.68rem;
    font-weight: 800;
    padding: 0.25rem 0.55rem;
    border-radius: 2rem;
    background: var(--accent-muted);
    text-transform: uppercase;
    letter-spacing: 0.05em;
    white-space: nowrap;
  }
  .sync-pill {
    text-transform: none;
    letter-spacing: 0;
    font-weight: 650;
    color: var(--text-secondary);
    background: var(--surface-hover);
  }
  .connection::before {
    content: '●';
    color: var(--success);
    margin-right: 0.3rem;
  }
  .connection.offline::before {
    color: var(--warning);
  }
  .avatars {
    display: inline-flex;
    align-items: center;
    padding: 0.15rem 0.5rem 0.15rem 0.2rem;
    border-radius: 999px;
    font-size: 0.75rem;
    gap: 0;
  }
  .avatar-chip {
    width: 1.55rem;
    height: 1.55rem;
    display: grid;
    place-content: center;
    border-radius: 50%;
    background: var(--surface-elevated);
    border: 2px solid var(--surface-page);
    margin-left: -0.35rem;
    font-size: 0.85rem;
  }
  .avatar-chip:first-child {
    margin-left: 0;
  }
  .avatar-count {
    margin-left: 0.4rem;
    color: var(--text-secondary);
    font-weight: 650;
  }
  .room-actions {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    flex: 0 0 auto;
  }
  .room-actions .active {
    border-color: var(--accent-primary);
  }
  .more summary {
    list-style: none;
    display: inline-flex;
    border: 1px solid var(--border-subtle);
    background: var(--surface-elevated);
    border-radius: var(--radius-sm);
  }
  .more summary::-webkit-details-marker {
    display: none;
  }
  .menu.wide {
    width: 230px;
  }
  .menu :global(svg) {
    flex: 0 0 auto;
  }
  .menu-link {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    padding: 0.7rem 1rem;
    color: var(--text-secondary);
    font-weight: 720;
    text-decoration: none;
    border-radius: var(--radius-sm);
  }
  .menu-link:hover {
    background: var(--surface-hover);
  }
  .menu-note {
    margin: 0.3rem 0 0;
    padding: 0.4rem 0.6rem 0.2rem;
    border-top: 1px solid var(--border-subtle);
    font-size: 0.72rem;
    color: var(--text-muted);
  }
  /* Settings -------------------------------------------------------------- */
  .settings {
    padding: 1rem;
    margin-bottom: 1rem;
  }
  .settings-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 0.6rem;
  }
  .settings-head h2 {
    margin: 0;
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
  .small {
    font-size: 0.78rem;
  }
  .toggle {
    display: flex;
    align-items: center;
    gap: 0.55rem;
    cursor: pointer;
  }
  .toggle input {
    width: auto;
    margin: 0;
    flex: 0 0 auto;
    cursor: pointer;
  }
  .invite-form,
  .report-form {
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
  /* Player bar -------------------------------------------------------------- */
  .player-bar {
    display: flex;
    align-items: center;
    gap: 0.55rem;
  }
  .play-toggle {
    flex: 0 0 auto;
    width: 2.8rem;
    height: 2.8rem;
    padding: 0;
    border-radius: 50%;
  }
  .scrubber-area {
    flex: 1;
    min-width: 0;
    padding: 0 0.3rem;
  }
  .scrubber-track {
    position: relative;
    height: 6px;
    border-radius: 999px;
    background: var(--surface-hover);
  }
  .scrubber-fill {
    height: 100%;
    border-radius: 999px;
    background: linear-gradient(90deg, var(--accent-primary), var(--accent-hover));
    transition: width 0.5s linear;
  }
  .scrubber-input {
    position: absolute;
    inset: -8px 0;
    width: 100%;
    height: calc(100% + 16px);
    opacity: 0;
    cursor: pointer;
    margin: 0;
    padding: 0;
  }
  .scrubber-area:hover .scrubber-track {
    height: 8px;
  }
  .scrubber-time {
    display: flex;
    justify-content: space-between;
    margin-top: 0.35rem;
    font-size: 0.72rem;
    color: var(--text-muted);
    font-variant-numeric: tabular-nums;
  }
  .idle-note {
    margin: 0;
    font-size: 0.8rem;
    color: var(--text-muted);
  }
  .bar-button {
    flex: 0 0 auto;
    padding: 0.55rem 0.7rem;
  }
  .bar-button.active {
    color: var(--accent-primary);
    border-color: var(--accent-primary);
  }
  .speed-control {
    flex: 0 0 5.2rem;
    width: 5.2rem;
    padding: 0.5rem 0.5rem;
    border-radius: var(--radius-md);
    border: 1px solid var(--border-subtle);
    background: var(--surface-elevated);
    color: inherit;
    font: inherit;
    font-size: 0.85rem;
    cursor: pointer;
  }
  .speed-control:disabled {
    cursor: default;
    opacity: 0.6;
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
  .waiting {
    margin: 0;
    color: var(--text-muted);
    font-size: 0.78rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .waiting {
    display: inline-flex;
    align-items: center;
    gap: 0.3rem;
    color: var(--warning);
  }
  .player-note {
    margin: 0;
    font-size: 0.68rem;
    color: var(--text-muted);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    min-width: 0;
  }
  .player-note a {
    color: inherit;
  }
  .reaction-bar {
    display: flex;
    align-items: center;
    gap: 0.15rem;
    flex: 0 0 auto;
  }
  .reaction-bar button {
    font-size: 1.15rem;
    padding: 0.35rem 0.45rem;
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
  .small-button {
    padding: 0.35rem 0.6rem;
    font-size: 0.78rem;
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
  .room-panel > header {
    padding: 0.7rem 1rem;
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .side-column h2 {
    font-size: 0.95rem;
    margin: 0;
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
    padding: 0.35rem 0.5rem;
  }
  .queue-search {
    display: grid;
    gap: 0.35rem;
    padding: 0 1rem 0.8rem;
    color: var(--text-muted);
    font-size: 0.75rem;
  }
  .queue-search input {
    width: 100%;
  }
  .empty {
    text-align: center;
    padding: 2.5rem 1rem;
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
  }
  .queue li,
  .members li {
    display: flex;
    align-items: center;
    gap: 0.55rem;
    padding: 0.6rem 0.8rem;
    border-top: 1px solid var(--border-subtle);
    transition: background 0.18s ease;
  }
  .queue li:hover,
  .members li:hover {
    background: var(--surface-hover);
  }
  .queue img,
  .queue .thumbnail-placeholder {
    width: 72px;
    aspect-ratio: 16/9;
    object-fit: cover;
    border-radius: 6px;
    flex: 0 0 auto;
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
  .queue b {
    font-size: 0.85rem;
    white-space: normal;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    line-clamp: 2;
    -webkit-box-orient: vertical;
    line-height: 1.3;
  }
  .queue small,
  .members small {
    font-size: 0.7rem;
    color: var(--text-muted);
  }
  .members small {
    text-transform: capitalize;
  }
  .members .state {
    text-transform: none;
    color: var(--warning);
    margin-left: 0.3rem;
  }
  .handle {
    color: var(--text-muted);
    cursor: grab;
  }
  @media (pointer: coarse) {
    .handle {
      display: none;
    }
  }
  .icon {
    font-size: 1.3rem;
    padding: 0.3rem;
  }
  .history {
    margin: 0.6rem 0.8rem 1rem;
    color: var(--text-muted);
    font-size: 0.82rem;
  }
  .history ul {
    list-style: none;
    padding: 0;
    margin: 0.4rem 0 0;
  }
  .history li {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.4rem;
    padding: 0.15rem 0;
  }
  .history li span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .history li button {
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
  .invite-wide {
    margin: 0.8rem;
    width: calc(100% - 1.6rem);
  }
  details {
    position: relative;
  }
  summary {
    cursor: pointer;
    list-style: none;
    padding: 0.4rem;
  }
  summary::-webkit-details-marker {
    display: none;
  }
  .menu {
    position: fixed;
    right: 0;
    top: 2rem;
    width: 170px;
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
    padding: 0.55rem 0.7rem;
    font-size: 0.85rem;
  }
  .events {
    padding: 0.8rem 1rem;
  }
  .events article {
    display: flex;
    gap: 0.8rem;
    padding: 0.45rem;
  }
  .events p {
    margin: 0;
    font-size: 0.85rem;
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
  /* Dialogs ---------------------------------------------------------------- */
  .modal-backdrop {
    position: fixed;
    inset: 0;
    display: grid;
    place-items: center;
    padding: 1rem;
    z-index: 80;
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
  .shortcuts {
    max-width: 30rem;
  }
  .shortcuts h2 {
    margin: 0;
    font-size: 1.1rem;
  }
  .shortcuts dl {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: 0.55rem 1rem;
    margin: 0;
    font-size: 0.88rem;
  }
  .shortcuts dt {
    white-space: nowrap;
  }
  .shortcuts dd {
    margin: 0;
    color: var(--text-secondary);
  }
  kbd {
    display: inline-block;
    padding: 0.1rem 0.4rem;
    border: 1px solid var(--border-strong);
    border-bottom-width: 2px;
    border-radius: 5px;
    font: inherit;
    font-size: 0.78rem;
    font-weight: 700;
    background: var(--surface-elevated);
  }
  /* Responsive ------------------------------------------------------------- */
  @media (max-width: 1100px) {
    .bar-label {
      display: none;
    }
  }
  @media (max-width: 900px) {
    .settings-grid {
      grid-template-columns: 1fr;
    }
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
    .empty-stage p,
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
    .room-header {
      margin-bottom: 0.5rem;
      align-items: flex-start;
    }
    .room-header h1 {
      font-size: 1.1rem;
    }
    .room-actions {
      gap: 0.35rem;
    }
    .rename-form input {
      min-width: 0;
      width: 100%;
    }
    .visibility,
    .avatar-count {
      display: none;
    }
    .invite-button {
      padding: 0.55rem 0.8rem;
    }
    .player-bar {
      gap: 0.35rem;
    }
    .play-toggle {
      width: 2.5rem;
      height: 2.5rem;
    }
    .bar-button {
      padding: 0.5rem 0.55rem;
    }
    .theater-toggle {
      display: none;
    }
    .speed-control {
      flex-basis: 4.2rem;
      width: 4.2rem;
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
    .queue li {
      padding: 0.55rem 0.6rem;
      gap: 0.45rem;
    }
    .queue img,
    .queue .thumbnail-placeholder {
      width: 58px;
    }
    .vote {
      padding: 0.3rem 0.4rem;
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .empty-emoji,
    .reaction-overlay span {
      animation: none;
    }
  }
</style>
