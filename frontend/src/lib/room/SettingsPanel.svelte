<script lang="ts">
  import { onMount } from 'svelte';
  import { fly } from 'svelte/transition';
  import { X, FilmSlate, Confetti, Crown } from 'phosphor-svelte';
  import { api } from '$lib/api';
  import { errorText, t } from '$lib/i18n';
  import { participantNameParts, type Member, type Snapshot } from '$lib/room';

  type Command = (type: string, payload?: Record<string, unknown>) => Promise<boolean>;
  let {
    room,
    self,
    manager,
    commandPending,
    command,
    onNotice,
    onLeaveOrDelete,
    onTransfer,
    onClose,
  }: {
    room: Snapshot;
    self: Member | undefined;
    manager: boolean;
    commandPending: boolean;
    command: Command;
    onNotice: (message: string, kind: 'success' | 'error') => void;
    onLeaveOrDelete: () => void;
    onTransfer: (member: Member) => void;
    onClose: () => void;
  } = $props();

  let invites = $state<{ username: string; createdAt: string }[]>([]);
  let bans = $state<{ identityId: string; displayName: string; createdAt: string }[]>([]);
  let loading = $state(false);
  let inviteUsername = $state('');
  let reportReason = $state('spam');
  let reportPending = $state(false);
  let reportSubmitted = $state(false);
  const owner = $derived(self?.role === 'owner');
  const modes = [
    { value: 'party', icon: Confetti },
    { value: 'cinema', icon: FilmSlate },
    { value: 'host', icon: Crown },
  ] as const;

  const toLocalInput = (ms: number) => {
    if (!ms) return '';
    // datetime-local wants local wall-clock time without a zone.
    return new Date(ms - new Date(ms).getTimezoneOffset() * 60_000).toISOString().slice(0, 16);
  };
  // Draft starts from the saved schedule when the panel opens.
  // svelte-ignore state_referenced_locally
  let scheduleDraft = $state(toLocalInput(room.scheduledAt ?? 0));

  async function load() {
    if (!manager) return;
    loading = true;
    try {
      [invites, bans] = await Promise.all([
        api<typeof invites>(`/api/rooms/${room.id}/invites`),
        api<typeof bans>(`/api/rooms/${room.id}/bans`),
      ]);
    } catch (e) {
      onNotice(errorText(e), 'error');
    } finally {
      loading = false;
    }
  }
  onMount(load);

  async function addInvite(event: SubmitEvent) {
    event.preventDefault();
    if (!inviteUsername.trim()) return;
    try {
      await api(`/api/rooms/${room.id}/invites`, {
        method: 'POST',
        body: JSON.stringify({ username: inviteUsername.trim() }),
      });
      inviteUsername = '';
      await load();
      onNotice($t('settings.inviteAdded'), 'success');
    } catch (e) {
      onNotice(errorText(e), 'error');
    }
  }
  async function revokeInvite(username: string) {
    try {
      await api(`/api/rooms/${room.id}/invites/${encodeURIComponent(username)}`, { method: 'DELETE' });
      invites = invites.filter((invite) => invite.username !== username);
      onNotice($t('settings.inviteRevoked'), 'success');
    } catch (e) {
      onNotice(errorText(e), 'error');
    }
  }
  async function unban(identityId: string) {
    if (await command('member.unban', { identityId })) bans = bans.filter((ban) => ban.identityId !== identityId);
  }
  async function reportRoom(event: SubmitEvent) {
    event.preventDefault();
    if (reportPending || reportSubmitted) return;
    reportPending = true;
    try {
      await api(`/api/rooms/${room.id}/reports`, { method: 'POST', body: JSON.stringify({ reason: reportReason }) });
      reportSubmitted = true;
      onNotice($t('settings.reportSent'), 'success');
    } catch (e) {
      onNotice(errorText(e), 'error');
    } finally {
      reportPending = false;
    }
  }
  function saveSchedule(event: SubmitEvent) {
    event.preventDefault();
    const at = scheduleDraft ? new Date(scheduleDraft).getTime() : 0;
    void command('room.schedule', { at: Number.isFinite(at) ? at : 0 });
  }
  const eligibleOwners = $derived(room.members.filter((m) => m.identityId !== room.me && m.accountLinked));
</script>

<section
  id="room-settings"
  class="settings panel"
  aria-label={$t('room.settings')}
  transition:fly={{ y: -8, duration: 180 }}
>
  <header class="settings-head">
    <h2>{$t('room.settings')}</h2>
    <button class="ghost icon-button" aria-label={$t('settings.close')} onclick={onClose}
      ><X size={16} weight="bold" /></button
    >
  </header>
  <div class="settings-grid">
    {#if manager}<div class="wide">
        <h3>{$t('settings.mode')}</h3>
        <div class="modes" role="radiogroup" aria-label={$t('settings.mode')}>
          {#each modes as mode (mode.value)}{@const Icon = mode.icon}<button
              type="button"
              role="radio"
              aria-checked={room.mode === mode.value}
              class="mode"
              class:active={room.mode === mode.value}
              onclick={() => room.mode !== mode.value && command('room.mode', { mode: mode.value })}
            >
              <Icon size={22} weight="duotone" />
              <b>{$t(`settings.mode.${mode.value}`)}</b>
              <small>{$t(`settings.mode.${mode.value}.hint`)}</small>
            </button>{/each}
        </div>
      </div>{/if}
    <div>
      <h3>{$t('settings.access')}</h3>
      <p class="muted">{$t('settings.accessHint')}</p>
      {#if manager}<label
          >{$t('settings.visibility')}<select
            value={room.visibility}
            disabled={commandPending}
            onchange={(e) => command('room.visibility', { visibility: e.currentTarget.value })}
          >
            <option value="unlisted">{$t('visibility.unlisted')}</option>{#if room.publicRoomsEnabled}<option
                value="public">{$t('visibility.public')}</option
              >{/if}<option value="private">{$t('visibility.private')}</option><option value="friends_only"
              >{$t('visibility.friends_only')}</option
            >
          </select></label
        >{/if}
      <p class="muted small">
        {$t('settings.privacyNote')} <a href="/privacy">{$t('room.privacyDetails')}</a>
      </p>
    </div>
    {#if manager}<div>
        <h3>{$t('settings.together')}</h3>
        <label class="toggle">
          <input
            type="checkbox"
            checked={room.waitForAll}
            onchange={(e) => command('room.wait', { enabled: e.currentTarget.checked })}
          /><span>{$t('settings.waitForAll')}</span>
        </label>
        <p class="muted small">{$t('settings.waitForAllHint')}</p>
        <label
          >{$t('settings.countdown')}<select
            value={room.countdownSeconds}
            onchange={(e) => command('room.countdown', { seconds: Number(e.currentTarget.value) })}
          >
            {#each [0, 2, 3, 5] as seconds}<option value={seconds}
                >{seconds ? $t('settings.countdownSeconds', { seconds }) : $t('settings.countdownOff')}</option
              >{/each}
          </select></label
        >
      </div>
      <div>
        <h3>SponsorBlock</h3>
        <p class="muted">
          {$t('settings.sponsorHint')}
          <a href="https://sponsor.ajay.app" target="_blank" rel="noopener noreferrer">SponsorBlock</a> (CC BY-NC-SA 4.0).
        </p>
        <label class="toggle">
          <input
            type="checkbox"
            checked={room.sponsorBlock}
            onchange={(e) => command('room.sponsorblock', { enabled: e.currentTarget.checked })}
          /><span>{$t('settings.sponsorToggle')}</span>
        </label>
      </div>
      <div>
        <h3>{$t('schedule.title')}</h3>
        <p class="muted">{$t('schedule.hint')}</p>
        <form class="inline-form" onsubmit={saveSchedule}>
          <label>{$t('schedule.when')}<input type="datetime-local" bind:value={scheduleDraft} /></label><button
            class="secondary">{$t('common.save')}</button
          >
        </form>
        {#if room.scheduledAt}<button
            class="ghost small-button"
            onclick={() => {
              scheduleDraft = '';
              void command('room.schedule', { at: 0 });
            }}>{$t('schedule.clear')}</button
          >{/if}
      </div>
      <div>
        <h3>{$t('settings.invitations')}</h3>
        <form class="inline-form" onsubmit={addInvite}>
          <label
            >{$t('settings.accountUsername')}<input
              bind:value={inviteUsername}
              pattern="[A-Za-z0-9_]+"
              minlength="3"
              maxlength="24"
            /></label
          ><button disabled={loading}>{$t('settings.invite')}</button>
        </form>
        {#if loading}<p class="muted">{$t('common.loading')}</p>{:else if !invites.length}<p class="muted">
            {$t('settings.noInvitations')}
          </p>{:else}<ul class="list">
            {#each invites as invite (invite.username)}<li>
                <span>{invite.username}</span><button class="ghost" onclick={() => revokeInvite(invite.username)}
                  >{$t('settings.revoke')}</button
                >
              </li>{/each}
          </ul>{/if}
      </div>
      <div>
        <h3>{$t('settings.bans')}</h3>
        {#if !bans.length}<p class="muted">{$t('settings.noBans')}</p>{:else}<ul class="list">
            {#each bans as ban (ban.identityId)}<li>
                <span>{participantNameParts(ban.displayName || '?').label}</span><button
                  class="ghost"
                  disabled={commandPending}
                  onclick={() => unban(ban.identityId)}>{$t('settings.unban')}</button
                >
              </li>{/each}
          </ul>{/if}
      </div>{/if}
    {#if room.visibility === 'public'}<div>
        <h3>{$t('settings.report')}</h3>
        <p class="muted">{$t('settings.reportHint')}</p>
        {#if reportSubmitted}<p role="status">{$t('settings.reportSent')}</p>{:else}<form
            class="inline-form"
            onsubmit={reportRoom}
          >
            <label
              >{$t('settings.reason')}<select bind:value={reportReason}>
                {#each ['spam', 'illegal_content', 'sexual_content', 'violent_content', 'harassment', 'other'] as const as reason}<option
                    value={reason}>{$t(`report.${reason}`)}</option
                  >{/each}
              </select></label
            ><button disabled={reportPending}
              >{reportPending ? $t('settings.submitting') : $t('settings.submitReport')}</button
            >
          </form>{/if}
      </div>{/if}
    {#if owner}<div>
        <h3>{$t('settings.transfer')}</h3>
        <p class="muted">{$t('settings.transferHint')}</p>
        <ul class="list">
          {#each eligibleOwners as member (member.identityId)}<li>
              <span>{member.displayName}</span><button class="secondary" onclick={() => onTransfer(member)}
                >{$t('settings.transferButton')}</button
              >
            </li>{/each}
        </ul>
        {#if !eligibleOwners.length}<p class="muted">{$t('settings.noEligible')}</p>{/if}
      </div>{/if}
    <div class="danger-settings">
      <h3>{owner ? $t('settings.delete') : $t('settings.leave')}</h3>
      <p class="muted">{owner ? $t('settings.deleteHint') : $t('settings.leaveHint')}</p>
      <button class="danger" onclick={onLeaveOrDelete}>{owner ? $t('settings.delete') : $t('settings.leave')}</button>
    </div>
  </div>
</section>

<style>
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
  h2 {
    margin: 0;
    font-size: 1.05rem;
  }
  h3 {
    margin: 0 0 0.5rem;
    font-size: 0.98rem;
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
    display: grid;
    gap: 0.5rem;
    align-content: start;
  }
  .settings-grid > .wide {
    grid-column: 1 / -1;
  }
  .modes {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 0.6rem;
  }
  .mode {
    display: grid;
    justify-items: start;
    gap: 0.25rem;
    text-align: left;
    background: var(--surface-elevated);
    color: var(--text-primary);
    border: 1px solid var(--border-subtle);
    box-shadow: none;
    padding: 0.8rem;
  }
  .mode small {
    color: var(--text-muted);
    font-weight: 550;
    line-height: 1.35;
  }
  .mode.active {
    border-color: var(--accent-primary);
    background: var(--accent-muted);
  }
  .mode :global(svg) {
    color: var(--accent-primary);
  }
  .small {
    font-size: 0.78rem;
    margin: 0;
  }
  p {
    margin: 0;
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
  }
  .inline-form {
    display: grid;
    grid-template-columns: 1fr auto;
    align-items: end;
    gap: 0.6rem;
  }
  .list {
    list-style: none;
    padding: 0;
    margin: 0;
  }
  .list li {
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
  @media (max-width: 900px) {
    .settings-grid {
      grid-template-columns: 1fr;
    }
    .modes {
      grid-template-columns: 1fr;
    }
  }
</style>
