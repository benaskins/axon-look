const BASE = '';

/**
 * Fetch JSON from the analytics API.
 * ClickHouse returns JSONEachRow — one JSON object per line.
 * We parse each line into an array of objects.
 */
export async function fetchApi(path) {
  const res = await fetch(`${BASE}${path}`);
  if (!res.ok) {
    throw new Error(`API error: ${res.status}`);
  }

  const text = await res.text();
  if (!text.trim()) return [];

  // JSONEachRow: one JSON object per line
  return text.trim().split('\n').map(line => JSON.parse(line));
}

/**
 * Fetch agent stats.
 */
export async function fetchAgentStats(slug) {
  const rows = await fetchApi(`/api/agents/${slug}/stats`);
  return rows[0] || {};
}

/**
 * Fetch message activity over time.
 */
export async function fetchMessages(slug, period = '7d') {
  return fetchApi(`/api/agents/${slug}/messages?period=${period}`);
}

/**
 * Fetch tool usage stats.
 */
export async function fetchTools(slug, period = '30d') {
  return fetchApi(`/api/agents/${slug}/tools?period=${period}`);
}

/**
 * Fetch relationship metrics over time.
 */
export async function fetchRelationship(slug, period = '90d') {
  return fetchApi(`/api/agents/${slug}/relationship?period=${period}`);
}

/**
 * Fetch memory stats by type.
 */
export async function fetchMemories(slug, period = '30d') {
  return fetchApi(`/api/agents/${slug}/memories?period=${period}`);
}

/**
 * Fetch conversation list.
 */
export async function fetchConversations(slug) {
  return fetchApi(`/api/agents/${slug}/conversations`);
}

/**
 * Fetch available runs.
 */
export async function fetchRuns() {
  return fetchApi('/api/runs');
}

/**
 * Fetch run summary by run_id.
 */
export async function fetchRunSummary(runId) {
  const rows = await fetchApi(`/api/runs/${runId}/summary`);
  return rows[0] || {};
}
