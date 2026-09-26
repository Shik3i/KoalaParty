<script lang="ts">
  import {
    ArrowsClockwise,
    ClipboardText,
    DotsThree,
    DownloadSimple,
    Gear,
    Keyboard,
    PencilSimple,
    ShareNetwork,
    ShieldCheck,
    Sparkle,
  } from 'phosphor-svelte';
  import { t } from '$lib/i18n';
  import { participantNameParts, type Member, type Snapshot } from '$lib/room';
  import { anchoredMenu } from './actions';

  let {
    room,
    connected,
    syncLabel,
    syncNote,
    activeMembers,
    manager,
    settingsOpen,
    onInvite,
    onToggleSettings,
    onRename,
    onShowPeople,
    onSyncNow,
    onCopyDiagnostics,
    onDownloadDiagnostics,
    onShortcuts,
    onRecap,
  }: {
    room: Snapshot;
    connected: boolean;
    syncLabel: string;
    syncNote: string;
    activeMembers: Member[];
    manager: boolean;
    settingsOpen: boolean;
    onInvite: () => void;
    onToggleSettings: () => void;
    onRename: (name: string) => void;
    onShowPeople: () => void;
    onSyncNow: () => void;
    onCopyDiagnostics: () => void;
    onDownloadDiagnostics: () => void;
    onShortcuts: () => void;
    onRecap: () => void;
  } = $props();

  let renaming = $state(false);
  let draft = $state('');
  let moreMenu: HTMLDetailsElement | undefined = $state();

  function save() {
    if (!renaming) return;
    renaming = false;
    if (draft.trim() !== room.label) onRename(draft);
  }
  function fromMenu(action: () => void) {
    if (moreMenu) moreMenu.open = false;
    action();
  }
  const visibilityLabel = $derived(
    room.visibility === 'public'
      ? $t('visibility.public')
      : room.visibility === 'private'
        ? $t('visibility.private')
        : room.visibility === 'friends_only'
          ? $t('visibility.friends_only')
          : $t('visibility.unlisted'),
  );
</script>

<header class="room-header">
  <div class="room-title">
    <div class="title-line">
      {#if renaming}<form
          class="rename-form"
          onsubmit={(event) => {
            event.preventDefault();
            save();
          }}
        >
          <!-- svelte-ignore a11y_autofocus -->
          <input
            aria-label={$t('room.nameLabel')}
            bind:value={draft}
            maxlength="60"
            autofocus
            placeholder={$t('room.namePlaceholder')}
            onblur={save}
            onkeydown={(event) => event.key === 'Escape' && (renaming = false)}
          />
        </form>{:else}<h1>{room.label}</h1>
        {#if manager}<button
            class="ghost icon-button"
            aria-label={$t('room.rename')}
            title={$t('room.rename')}
            onclick={() => {
              draft = room.label;
              renaming = true;
            }}><PencilSimple size={16} weight="bold" /></button
          >{/if}{/if}
    </div>
    <div class="room-meta">
      <span class:offline={!connected} class="connection" role="status"
        >{connected ? $t('room.live') : $t('room.reconnecting')}</span
      ><span class="sync-pill" title={$t('room.syncHint')}>{syncLabel}</span><span class="visibility"
        >{visibilityLabel}</span
      >
      <button
        class="ghost avatars"
        aria-label={$t('room.watchingShow', { count: activeMembers.length })}
        onclick={onShowPeople}
        >{#each activeMembers.slice(0, 5) as member (member.identityId)}<span
            class="avatar-chip"
            title={member.displayName}>{participantNameParts(member.displayName).badge}</span
          >{/each}<span class="avatar-count">{$t('room.watching', { count: activeMembers.length })}</span></button
      >
    </div>
  </div>
  <div class="room-actions">
    <button class="invite-button" onclick={onInvite}><ShareNetwork size={17} weight="bold" />{$t('room.invite')}</button
    >
    <button
      class="secondary icon-button"
      aria-label={$t('room.settings')}
      title={$t('room.settings')}
      aria-controls="room-settings"
      aria-expanded={settingsOpen}
      class:active={settingsOpen}
      onclick={onToggleSettings}><Gear size={18} weight="bold" /></button
    >
    <details class="more" bind:this={moreMenu} use:anchoredMenu>
      <summary class="secondary icon-button" aria-label={$t('room.more')} title={$t('room.more')}
        ><DotsThree size={20} weight="bold" /></summary
      >
      <div class="menu wide">
        <button class="ghost" onclick={() => fromMenu(onRecap)}
          ><Sparkle size={16} weight="bold" />{$t('recap.open')}</button
        >
        <button class="ghost" onclick={() => fromMenu(onSyncNow)}
          ><ArrowsClockwise size={16} weight="bold" />{$t('room.syncNow')}</button
        >
        <button class="ghost" onclick={() => fromMenu(onCopyDiagnostics)}
          ><ClipboardText size={16} weight="bold" />{$t('room.copyDiagnostics')}</button
        >
        <button class="ghost" onclick={() => fromMenu(onDownloadDiagnostics)}
          ><DownloadSimple size={16} weight="bold" />{$t('room.downloadDiagnostics')}</button
        >
        <button class="ghost" onclick={() => fromMenu(onShortcuts)}
          ><Keyboard size={16} weight="bold" />{$t('room.shortcuts')}</button
        >
        <a class="menu-link" href="/privacy"><ShieldCheck size={16} weight="bold" />{$t('room.privacyDetails')}</a>
        <p class="menu-note">{syncNote}</p>
      </div>
    </details>
  </div>
</header>

<style>
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
  h1 {
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
    display: inline-flex;
    border: 1px solid var(--border-subtle);
    background: var(--surface-elevated);
    border-radius: var(--radius-sm);
  }
  .menu.wide {
    width: 240px;
  }
  .menu-link {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    padding: 0.6rem 0.7rem;
    color: var(--text-secondary);
    font-weight: 720;
    font-size: 0.85rem;
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
  @media (max-width: 580px) {
    .room-header {
      margin-bottom: 0.5rem;
      align-items: flex-start;
    }
    h1 {
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
  }
</style>
