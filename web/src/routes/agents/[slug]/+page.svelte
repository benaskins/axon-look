<script>
  import { page } from '$app/stores';
  import { fetchAgentStats, fetchMessages, fetchTools, fetchRelationship, fetchMemories, fetchConversations, fetchRuns, fetchRunSummary } from '$lib/api.js';

  let slug = $derived($page.params.slug);

  let stats = $state(null);
  let messages = $state([]);
  let tools = $state([]);
  let relationship = $state([]);
  let memories = $state([]);
  let conversations = $state([]);
  let runs = $state([]);
  let selectedRunId = $state('');
  let runSummary = $state(null);
  let loading = $state(true);
  let error = $state(null);

  $effect(() => {
    loadData(slug);
  });

  $effect(() => {
    if (selectedRunId) {
      loadRunSummary(selectedRunId);
    } else {
      runSummary = null;
    }
  });

  async function loadData(s) {
    loading = true;
    error = null;
    try {
      [stats, messages, tools, relationship, memories, conversations, runs] = await Promise.all([
        fetchAgentStats(s),
        fetchMessages(s),
        fetchTools(s),
        fetchRelationship(s),
        fetchMemories(s),
        fetchConversations(s),
        fetchRuns(),
      ]);
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  async function loadRunSummary(runId) {
    try {
      runSummary = await fetchRunSummary(runId);
    } catch (e) {
      runSummary = null;
    }
  }

  function fmt(n) {
    if (n == null) return '—';
    if (typeof n === 'number') return n.toLocaleString();
    return n;
  }

  function fmtDuration(ms) {
    if (ms == null) return '—';
    if (ms < 1000) return `${Math.round(ms)}ms`;
    return `${(ms / 1000).toFixed(1)}s`;
  }
</script>

<div class="dashboard">
  <header>
    <a href="/" class="back">&larr; agents</a>
    <h1>{slug}</h1>
    {#if runs.length > 0}
      <select class="run-selector" bind:value={selectedRunId}>
        <option value="">All data</option>
        {#each runs as run}
          <option value={run.run_id}>{run.run_id} — {run.description || 'untitled'}</option>
        {/each}
      </select>
    {/if}
  </header>

  {#if runSummary}
    <section class="panel run-summary">
      <h2>Run: {selectedRunId}</h2>
      <div class="stats-grid">
        <div class="stat-card">
          <span class="stat-label">Messages</span>
          <span class="stat-value">{fmt(runSummary.messages)}</span>
        </div>
        <div class="stat-card">
          <span class="stat-label">Tool Calls</span>
          <span class="stat-value">{fmt(runSummary.tool_invocations)}</span>
        </div>
        <div class="stat-card">
          <span class="stat-label">Conversations</span>
          <span class="stat-value">{fmt(runSummary.conversations)}</span>
        </div>
        <div class="stat-card">
          <span class="stat-label">Memories</span>
          <span class="stat-value">{fmt(runSummary.memories)}</span>
        </div>
        <div class="stat-card">
          <span class="stat-label">Relationships</span>
          <span class="stat-value">{fmt(runSummary.relationship_snapshots)}</span>
        </div>
        <div class="stat-card">
          <span class="stat-label">Consolidations</span>
          <span class="stat-value">{fmt(runSummary.consolidations)}</span>
        </div>
      </div>
    </section>
  {/if}

  {#if loading}
    <p class="status">Loading...</p>
  {:else if error}
    <p class="status error">{error}</p>
  {:else}
    <!-- Stats header -->
    <section class="stats-grid">
      <div class="stat-card">
        <span class="stat-label">Conversations</span>
        <span class="stat-value">{fmt(stats.total_conversations)}</span>
      </div>
      <div class="stat-card">
        <span class="stat-label">Messages</span>
        <span class="stat-value">{fmt(stats.total_messages)}</span>
      </div>
      <div class="stat-card">
        <span class="stat-label">Prompt Tokens</span>
        <span class="stat-value">{fmt(stats.total_prompt_tokens)}</span>
      </div>
      <div class="stat-card">
        <span class="stat-label">Completion Tokens</span>
        <span class="stat-value">{fmt(stats.total_completion_tokens)}</span>
      </div>
      <div class="stat-card">
        <span class="stat-label">Avg Duration</span>
        <span class="stat-value">{fmtDuration(stats.avg_duration_ms)}</span>
      </div>
    </section>

    <!-- Sections for Steps 8-10 -->
    <section class="panel">
      <h2>Message Activity</h2>
      {#if messages.length === 0}
        <p class="empty">No message data yet</p>
      {:else}
        <table>
          <thead>
            <tr>
              <th>Day</th>
              <th>Total</th>
              <th>User</th>
              <th>Assistant</th>
            </tr>
          </thead>
          <tbody>
            {#each messages as row}
              <tr>
                <td>{row.day}</td>
                <td>{fmt(row.total)}</td>
                <td>{fmt(row.user_messages)}</td>
                <td>{fmt(row.assistant_messages)}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      {/if}
    </section>

    <section class="panel">
      <h2>Tool Usage</h2>
      {#if tools.length === 0}
        <p class="empty">No tool data yet</p>
      {:else}
        <table>
          <thead>
            <tr>
              <th>Tool</th>
              <th>Invocations</th>
              <th>Successes</th>
              <th>Avg Duration</th>
            </tr>
          </thead>
          <tbody>
            {#each tools as row}
              <tr>
                <td><code>{row.tool_name}</code></td>
                <td>{fmt(row.invocations)}</td>
                <td>{fmt(row.successes)}</td>
                <td>{fmtDuration(row.avg_duration_ms)}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      {/if}
    </section>

    <section class="panel">
      <h2>Relationship</h2>
      {#if relationship.length === 0}
        <p class="empty">No relationship data yet</p>
      {:else}
        <table>
          <thead>
            <tr>
              <th>Time</th>
              <th>Trust</th>
              <th>Intimacy</th>
              <th>Autonomy</th>
              <th>Reciprocity</th>
              <th>Playfulness</th>
              <th>Conflict</th>
            </tr>
          </thead>
          <tbody>
            {#each relationship as row}
              <tr>
                <td>{new Date(row.timestamp).toLocaleDateString()}</td>
                <td>{(row.trust * 100).toFixed(0)}%</td>
                <td>{(row.intimacy * 100).toFixed(0)}%</td>
                <td>{(row.autonomy * 100).toFixed(0)}%</td>
                <td>{(row.reciprocity * 100).toFixed(0)}%</td>
                <td>{(row.playfulness * 100).toFixed(0)}%</td>
                <td>{(row.conflict * 100).toFixed(0)}%</td>
              </tr>
            {/each}
          </tbody>
        </table>
      {/if}
    </section>

    <section class="panel">
      <h2>Memories</h2>
      {#if memories.length === 0}
        <p class="empty">No memory data yet</p>
      {:else}
        <table>
          <thead>
            <tr>
              <th>Type</th>
              <th>Count</th>
              <th>Avg Importance</th>
            </tr>
          </thead>
          <tbody>
            {#each memories as row}
              <tr>
                <td>{row.memory_type}</td>
                <td>{fmt(row.count)}</td>
                <td>{(row.avg_importance * 100).toFixed(0)}%</td>
              </tr>
            {/each}
          </tbody>
        </table>
      {/if}
    </section>

    <section class="panel">
      <h2>Conversations</h2>
      {#if conversations.length === 0}
        <p class="empty">No conversation data yet</p>
      {:else}
        <table>
          <thead>
            <tr>
              <th>ID</th>
              <th>Messages</th>
              <th>Tools Used</th>
              <th>Avg Duration</th>
            </tr>
          </thead>
          <tbody>
            {#each conversations as row}
              <tr>
                <td><code>{row.conversation_id?.slice(0, 8)}</code></td>
                <td>{fmt(row.messages)}</td>
                <td>{fmt(row.tools_used)}</td>
                <td>{fmtDuration(row.avg_duration_ms)}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      {/if}
    </section>
  {/if}
</div>

<style>
  .dashboard {
    display: flex;
    flex-direction: column;
    gap: 1.5rem;
  }

  header {
    display: flex;
    align-items: baseline;
    gap: 1rem;
  }

  .back {
    font-size: 0.75rem;
    color: var(--text-muted);
  }

  h1 {
    font-size: 1.5rem;
    font-weight: 600;
    color: var(--accent);
  }

  h2 {
    font-size: 0.875rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-muted);
    margin-bottom: 0.75rem;
  }

  .status {
    color: var(--text-muted);
    text-align: center;
    padding: 2rem;
  }

  .status.error {
    color: var(--chart-5);
  }

  .stats-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
    gap: 0.75rem;
  }

  .stat-card {
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 1rem;
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }

  .stat-label {
    font-size: 0.7rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-muted);
  }

  .stat-value {
    font-size: 1.5rem;
    font-weight: 600;
    font-family: var(--font-mono);
    color: var(--text-primary);
  }

  .panel {
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 1rem;
  }

  .empty {
    color: var(--text-muted);
    font-size: 0.875rem;
    text-align: center;
    padding: 1rem;
  }

  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.8125rem;
  }

  th {
    text-align: left;
    color: var(--text-muted);
    font-weight: 500;
    font-size: 0.7rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    padding: 0.5rem;
    border-bottom: 1px solid var(--border);
  }

  td {
    padding: 0.5rem;
    border-bottom: 1px solid var(--border);
    color: var(--text-secondary);
  }

  tr:last-child td {
    border-bottom: none;
  }

  code {
    font-family: var(--font-mono);
    font-size: 0.75rem;
    color: var(--accent);
  }

  .run-selector {
    margin-left: auto;
    background: var(--bg-secondary);
    color: var(--text-primary);
    border: 1px solid var(--border);
    border-radius: 4px;
    padding: 0.375rem 0.5rem;
    font-size: 0.75rem;
    font-family: var(--font-mono);
  }

  .run-summary {
    border-color: var(--accent);
  }
</style>
