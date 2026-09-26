<script lang="ts">
  import { DotsThreeVertical, Hourglass, PencilSimple, ShareNetwork, SpeakerSlash } from 'phosphor-svelte';
  import { t } from '$lib/i18n';
  import { participantNameParts, type Member, type PresenceState } from '$lib/room';
  import { anchoredMenu, closeMenu } from './actions';

  let {
    members,
    me,
    presence,
    manager,
    commandPending,
    onMemberAction,
    onEditName,
    onInvite,
  }: {
    members: Member[];
    me: string;
    presence: Record<string, PresenceState>;
    manager: boolean;
    commandPending: boolean;
    onMemberAction: (member: Member, action: 'kick' | 'ban' | 'role' | 'mute') => void;
    onEditName: () => void;
    onInvite: () => void;
  } = $props();
  const online = $derived(members.filter((member) => member.active).length);
  const roleLabel = (role: string) =>
    role === 'owner' ? $t('role.owner') : role === 'admin' ? $t('role.admin') : $t('role.member');
</script>

<header>
  <h2>{$t('people.title')}</h2>
  <span class="online-count"><span class="online-dot"></span>{$t('people.online', { count: online })}</span>
</header>
<ul class="members">
  {#each members as member (member.identityId)}{@const parts = participantNameParts(member.displayName)}
    {@const state = member.active ? presence[member.identityId] : undefined}
    <li>
      <div class="avatar" class:offline={!member.active}>
        <span aria-hidden="true">{parts.badge}</span><span
          class="presence"
          title={member.active ? $t('people.onlineOne') : $t('people.offline')}
        ></span>
      </div>
      <div>
        <b>{parts.label}{member.identityId === me ? ` ${$t('people.you')}` : ''}</b><small
          ><span class="role">{roleLabel(member.role)}</span>{#if state === 'buffering'}<span class="state"
              ><Hourglass size={11} weight="bold" /> {$t('people.buffering')}</span
            >{:else if state === 'blocked'}<span class="state"
              ><SpeakerSlash size={11} weight="bold" /> {$t('people.blocked')}</span
            >{/if}{#if member.permissions['chat.send'] === false}<span class="state">
              · {$t('people.muted')}</span
            >{/if}</small
        >
      </div>
      {#if member.identityId === me}<button
          class="ghost small-button"
          onclick={onEditName}
          aria-label={$t('people.changeName')}><PencilSimple size={14} weight="bold" /></button
        >{/if}
      {#if manager && member.role !== 'owner' && member.identityId !== me}<details use:anchoredMenu>
          <summary aria-label={$t('people.manage', { name: member.displayName })}
            ><DotsThreeVertical size={18} weight="bold" /></summary
          >
          <div class="menu">
            <button
              class="ghost"
              disabled={commandPending}
              onclick={(event) => {
                closeMenu(event);
                onMemberAction(member, 'role');
              }}>{member.role === 'admin' ? $t('people.makeMember') : $t('people.makeAdmin')}</button
            >{#if member.role === 'member'}<button
                class="ghost"
                disabled={commandPending}
                onclick={(event) => {
                  closeMenu(event);
                  onMemberAction(member, 'mute');
                }}>{member.permissions['chat.send'] === false ? $t('people.unmute') : $t('people.mute')}</button
              >{/if}<button
              class="ghost"
              disabled={commandPending}
              onclick={(event) => {
                closeMenu(event);
                onMemberAction(member, 'kick');
              }}>{$t('people.kick')}</button
            ><button
              class="danger"
              disabled={commandPending}
              onclick={(event) => {
                closeMenu(event);
                onMemberAction(member, 'ban');
              }}>{$t('people.ban')}</button
            >
          </div>
        </details>{/if}
    </li>{/each}
</ul>
<button class="secondary invite-wide" onclick={onInvite}
  ><ShareNetwork size={16} weight="bold" />{$t('people.inviteMore')}</button
>

<style>
  .members {
    list-style: none;
    padding: 0;
    margin: 0;
  }
  .members li {
    display: flex;
    align-items: center;
    gap: 0.55rem;
    padding: 0.6rem 0.8rem;
    border-top: 1px solid var(--border-subtle);
  }
  .members li:hover {
    background: var(--surface-hover);
  }
  .members li > div:not(.avatar) {
    min-width: 0;
    flex: 1;
  }
  b,
  small {
    display: block;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  small {
    font-size: 0.7rem;
    color: var(--text-muted);
  }
  .state {
    color: var(--warning);
    margin-left: 0.3rem;
  }
  .avatar {
    position: relative;
    width: 2rem;
    height: 2rem;
    flex: 0 0 auto;
    border-radius: 50%;
    display: grid;
    place-content: center;
    background: var(--accent-muted);
    font-weight: 900;
    font-size: 1.05rem;
    line-height: 1;
  }
  .presence {
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
</style>
