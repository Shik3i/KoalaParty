<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';

  interface ActiveRoom {
    roomId: string;
    label: string;
    viewerCount: number;
    currentVideo?: string;
  }

  interface Stats {
    totalAccounts: number;
    totalRooms: number;
    onlineUsers: number;
    activeRooms: ActiveRoom[];
  }

  interface Settings {
    sessionTTL: string;
    activityMaxAge: string;
    activityMaxEvents: number;
    roomMaxIdle: string;
    publicRooms: boolean;
  }

  interface Report {
    id: string;
    roomId: string;
    roomLabel: string;
    reason: string;
    metadata: any;
    createdAt: string;
  }

  let activeTab = $state<'stats' | 'settings' | 'reports'>('stats');
  let stats: Stats | null = $state(null);
  let settings: Settings | null = $state(null);
  let reports: Report[] = $state([]);
  let loading = $state(true);
  let error = $state('');
  let successMsg = $state('');

  async function loadData() {
    loading = true;
    error = '';
    try {
      const [s, set, rep] = await Promise.all([
        api<Stats>('/api/admin/stats'),
        api<Settings>('/api/admin/settings'),
        api<Report[]>('/api/admin/reports'),
      ]);
      stats = s;
      settings = set;
      reports = rep;
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load administrator data';
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    loadData();
  });

  async function saveSettings(e: SubmitEvent) {
    e.preventDefault();
    if (!settings) return;
    error = '';
    successMsg = '';
    try {
      await api('/api/admin/settings', {
        method: 'POST',
        body: JSON.stringify(settings),
      });
      successMsg = 'Configuration settings updated successfully.';
      setTimeout(() => (successMsg = ''), 4000);
      loadData();
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to save settings';
    }
  }

  async function handleReport(reportId: string, action: 'resolve' | 'delist') {
    error = '';
    try {
      await api(`/api/admin/reports/${reportId}/${action}`, { method: 'POST' });
      reports = reports.filter((r) => r.id !== reportId);
      successMsg = `Report successfully ${action === 'delist' ? 'delisted & resolved' : 'resolved'}.`;
      setTimeout(() => (successMsg = ''), 4000);
      loadData();
    } catch (e) {
      error = e instanceof Error ? e.message : `Failed to ${action} report`;
    }
  }
</script>

<svelte:head>
  <title>Admin Console — KoalaParty</title>
</svelte:head>

<main class="admin-container">
  <header class="admin-header">
    <h1>Admin Console</h1>
    <p class="muted">Monitor statistics, manage runtime settings, and moderate reported rooms.</p>
  </header>

  {#if error}
    <div class="alert error-alert" role="alert">
      <span>⚠️ Error:</span>
      {error}
    </div>
  {/if}

  {#if successMsg}
    <div class="alert success-alert" role="alert">
      <span>✅ Success:</span>
      {successMsg}
    </div>
  {/if}

  <div class="tabs" role="tablist">
    <button
      role="tab"
      aria-selected={activeTab === 'stats'}
      class:active={activeTab === 'stats'}
      onclick={() => (activeTab = 'stats')}
    >
      📈 Dashboard
    </button>
    <button
      role="tab"
      aria-selected={activeTab === 'settings'}
      class:active={activeTab === 'settings'}
      onclick={() => (activeTab = 'settings')}
    >
      ⚙️ Settings
    </button>
    <button
      role="tab"
      aria-selected={activeTab === 'reports'}
      class:active={activeTab === 'reports'}
      onclick={() => (activeTab = 'reports')}
    >
      🚨 Moderation Reports ({reports.length})
    </button>
  </div>

  {#if loading}
    <div class="loading-state">
      <span class="spinner"></span> Loading administrative dashboard...
    </div>
  {:else}
    <div class="tab-content panel">
      {#if activeTab === 'stats' && stats}
        <section class="dashboard-grid">
          <div class="stat-card">
            <h3>Connected Online Users</h3>
            <span class="stat-value">{stats.onlineUsers}</span>
            <p class="muted">Unique WS connections</p>
          </div>
          <div class="stat-card">
            <h3>Registered Accounts</h3>
            <span class="stat-value">{stats.totalAccounts}</span>
            <p class="muted">Total users stored in DB</p>
          </div>
          <div class="stat-card">
            <h3>Total Rooms</h3>
            <span class="stat-value">{stats.totalRooms}</span>
            <p class="muted">Active created rooms</p>
          </div>
        </section>

        <section class="active-rooms-section">
          <h2>Currently Active WebSocket Rooms ({stats.activeRooms.length})</h2>
          {#if stats.activeRooms.length === 0}
            <p class="empty-state">No rooms are currently online right now.</p>
          {:else}
            <div class="table-wrapper">
              <table>
                <thead>
                  <tr>
                    <th>Room Code</th>
                    <th>Label</th>
                    <th>Viewers</th>
                    <th>Playing Video</th>
                  </tr>
                </thead>
                <tbody>
                  {#each stats.activeRooms as room}
                    <tr>
                      <td><a href="/room/{room.roomId}" class="room-link">{room.roomId}</a></td>
                      <td>{room.label}</td>
                      <td><span class="badge">{room.viewerCount} online</span></td>
                      <td><span class="video-title">{room.currentVideo || 'None (idle)'}</span></td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
          {/if}
        </section>
      {/if}

      {#if activeTab === 'settings' && settings}
        <form onsubmit={saveSettings} class="settings-form">
          <h2>Dynamic Configuration Settings</h2>
          <p class="muted">
            Modify database settings which update running server variables in memory instantly without restarting.
          </p>

          <div class="form-group">
            <label for="session-ttl">
              <strong>Session Token TTL</strong>
              <span class="hint">Expiry time of authenticated cookies (e.g., 168h, 24h).</span>
            </label>
            <input id="session-ttl" type="text" bind:value={settings.sessionTTL} required />
          </div>

          <div class="form-group">
            <label for="room-max-idle">
              <strong>Room Maximum Idle TTL</strong>
              <span class="hint"
                >How long empty/idle rooms remain active before being cleaned up (e.g., 8760h, 24h).</span
              >
            </label>
            <input id="room-max-idle" type="text" bind:value={settings.roomMaxIdle} required />
          </div>

          <div class="form-group">
            <label for="activity-max-age">
              <strong>Room Activity Log History Age</strong>
              <span class="hint">Events older than this duration are automatically pruned (e.g., 720h, 48h).</span>
            </label>
            <input id="activity-max-age" type="text" bind:value={settings.activityMaxAge} required />
          </div>

          <div class="form-group">
            <label for="activity-max-events">
              <strong>Room Activity Log Max Size</strong>
              <span class="hint">Prunes oldest events when exceeding this number of events per room (minimum 10).</span>
            </label>
            <input id="activity-max-events" type="number" min="10" bind:value={settings.activityMaxEvents} required />
          </div>

          <div class="form-group checkbox-group">
            <input id="public-rooms" type="checkbox" bind:checked={settings.publicRooms} />
            <label for="public-rooms">
              <strong>Enable Public Room Discovery</strong>
              <span class="hint">Allows room owners to list their room in the public /discover section.</span>
            </label>
          </div>

          <button type="submit" class="primary">Save Configuration</button>
        </form>
      {/if}

      {#if activeTab === 'reports'}
        <section class="reports-section">
          <h2>Room Reports Pending Moderation ({reports.length})</h2>
          {#if reports.length === 0}
            <p class="empty-state">All reports resolved! No pending complaints.</p>
          {:else}
            <div class="reports-list">
              {#each reports as report}
                <div class="report-card">
                  <div class="report-meta">
                    <span class="report-badge danger">{report.reason.replace('_', ' ')}</span>
                    <span class="report-date">Reported {new Date(report.createdAt).toLocaleString()}</span>
                  </div>
                  <h3>
                    Room: <a href="/room/{report.roomId}" class="room-link">{report.roomLabel} ({report.roomId})</a>
                  </h3>
                  {#if report.metadata && report.metadata.playback}
                    <div class="report-details">
                      <p><strong>Current Playback:</strong> {report.metadata.playback.media?.title || 'None'}</p>
                    </div>
                  {/if}
                  <div class="report-actions">
                    <button class="secondary" onclick={() => handleReport(report.id, 'resolve')}>Resolve Report</button>
                    <button class="danger" onclick={() => handleReport(report.id, 'delist')}>Delist Room</button>
                  </div>
                </div>
              {/each}
            </div>
          {/if}
        </section>
      {/if}
    </div>
  {/if}
</main>

<style>
  .admin-container {
    max-width: 1000px;
    margin: 2rem auto;
    padding: 0 1.5rem;
  }
  .admin-header {
    margin-bottom: 2rem;
  }
  .admin-header h1 {
    font-size: 2.5rem;
    margin-bottom: 0.5rem;
  }
  .tabs {
    display: flex;
    gap: 0.5rem;
    border-bottom: 1px solid var(--border-subtle);
    margin-bottom: 1.5rem;
    overflow-x: auto;
  }
  .tabs button {
    background: none;
    border: none;
    border-bottom: 3px solid transparent;
    padding: 0.75rem 1.25rem;
    font-size: 1rem;
    font-weight: 600;
    color: var(--text-secondary);
    cursor: pointer;
    white-space: nowrap;
    transition: all 0.2s ease;
  }
  .tabs button:hover {
    color: var(--text-primary);
  }
  .tabs button.active {
    color: var(--accent-primary);
    border-bottom-color: var(--accent-primary);
  }
  .tab-content {
    padding: 2rem;
  }
  .dashboard-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
    gap: 1.5rem;
    margin-bottom: 2.5rem;
  }
  .stat-card {
    background: var(--surface-panel);
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-md);
    padding: 1.5rem;
    display: flex;
    flex-direction: column;
  }
  .stat-card h3 {
    font-size: 0.9rem;
    color: var(--text-secondary);
    margin-bottom: 0.5rem;
  }
  .stat-value {
    font-size: 2.5rem;
    font-weight: 800;
    color: var(--text-primary);
    margin-bottom: 0.25rem;
  }
  .active-rooms-section h2 {
    font-size: 1.3rem;
    margin-bottom: 1rem;
  }
  .table-wrapper {
    overflow-x: auto;
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-md);
  }
  table {
    width: 100%;
    border-collapse: collapse;
    text-align: left;
    font-size: 0.95rem;
  }
  th,
  td {
    padding: 0.75rem 1rem;
    border-bottom: 1px solid var(--border-subtle);
  }
  th {
    background: var(--surface-panel);
    font-weight: 700;
    color: var(--text-secondary);
  }
  tr:last-child td {
    border-bottom: none;
  }
  .room-link {
    font-family: monospace;
    font-weight: 700;
    color: var(--accent-primary);
    text-decoration: none;
  }
  .room-link:hover {
    text-decoration: underline;
  }
  .badge {
    background: var(--surface-elevated);
    border: 1px solid var(--border-subtle);
    border-radius: 4px;
    padding: 0.2rem 0.5rem;
    font-size: 0.8rem;
    font-weight: 600;
  }
  .video-title {
    font-weight: 600;
    max-width: 300px;
    display: inline-block;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .settings-form {
    display: flex;
    flex-direction: column;
    gap: 1.5rem;
    max-width: 600px;
  }
  .form-group {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }
  .form-group label {
    display: flex;
    flex-direction: column;
  }
  .hint {
    font-size: 0.8rem;
    color: var(--text-muted);
  }
  .checkbox-group {
    flex-direction: row;
    align-items: flex-start;
    gap: 0.75rem;
  }
  .checkbox-group input {
    margin-top: 0.25rem;
    width: 1.2rem;
    height: 1.2rem;
  }
  .alert {
    padding: 1rem;
    border-radius: var(--radius-md);
    margin-bottom: 1.5rem;
    font-weight: 600;
  }
  .error-alert {
    background: var(--surface-panel);
    border: 1px solid var(--warning);
    color: var(--warning);
  }
  .success-alert {
    background: var(--surface-panel);
    border: 1px solid var(--accent-primary);
    color: var(--accent-primary);
  }
  .empty-state {
    text-align: center;
    padding: 3rem;
    color: var(--text-muted);
    border: 1px dashed var(--border-subtle);
    border-radius: var(--radius-md);
  }
  .reports-list {
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }
  .report-card {
    background: var(--surface-panel);
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-md);
    padding: 1.5rem;
  }
  .report-meta {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 0.75rem;
  }
  .report-badge {
    text-transform: uppercase;
    font-size: 0.75rem;
    font-weight: 700;
    border-radius: 4px;
    padding: 0.2rem 0.5rem;
  }
  .report-badge.danger {
    background: color-mix(in srgb, var(--warning) 15%, transparent);
    border: 1px solid var(--warning);
    color: var(--warning);
  }
  .report-date {
    font-size: 0.8rem;
    color: var(--text-muted);
  }
  .report-details {
    background: var(--surface-elevated);
    border-radius: var(--radius-sm);
    padding: 0.75rem;
    margin: 1rem 0;
  }
  .report-actions {
    display: flex;
    gap: 0.5rem;
    margin-top: 1rem;
  }
  .loading-state {
    text-align: center;
    padding: 4rem;
    color: var(--text-muted);
  }
  .spinner {
    display: inline-block;
    width: 1.5rem;
    height: 1.5rem;
    border: 3px solid var(--border-subtle);
    border-top-color: var(--accent-primary);
    border-radius: 50%;
    animation: spin 1s linear infinite;
    vertical-align: middle;
    margin-right: 0.5rem;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
</style>
