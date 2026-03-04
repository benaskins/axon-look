package anal

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/benaskins/axon"
)

func escapeSingleQuotes(s string) string {
	return strings.ReplaceAll(s, "'", "\\'")
}

// Inserter executes insert statements against ClickHouse.
type Inserter interface {
	Exec(ctx context.Context, query string) error
}

type ingestHandler struct {
	db Inserter
}

func (h *ingestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var events []Event
	if err := json.NewDecoder(r.Body).Decode(&events); err != nil {
		axon.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	for _, e := range events {
		query, ok := insertQuery(e)
		if !ok {
			slog.Warn("unknown event type", "type", e.Type)
			continue
		}
		if err := h.db.Exec(r.Context(), query); err != nil {
			slog.Error("failed to insert event", "type", e.Type, "error", err)
			// Continue — don't fail the batch for one bad event
		}
	}

	w.WriteHeader(http.StatusAccepted)
}

func insertQuery(e Event) (string, bool) {
	ts := e.Timestamp.Format("2006-01-02 15:04:05.000")

	switch e.Type {
	case "message":
		return fmt.Sprintf(
			"INSERT INTO events_message (timestamp, conversation_id, agent_slug, user_id, role, prompt_tokens, completion_tokens, duration_ms, run_id) VALUES ('%s', '%s', '%s', '%s', '%s', %d, %d, %d, '%s')",
			ts, e.ConversationID, e.AgentSlug, e.UserID, e.Role, e.PromptTokens, e.CompletionTokens, e.DurationMs, e.RunID,
		), true

	case "tool_invocation":
		success := false
		if e.Success != nil {
			success = *e.Success
		}
		return fmt.Sprintf(
			"INSERT INTO events_tool_invocation (timestamp, conversation_id, agent_slug, user_id, tool_name, success, duration_ms, run_id) VALUES ('%s', '%s', '%s', '%s', '%s', %t, %d, '%s')",
			ts, e.ConversationID, e.AgentSlug, e.UserID, e.ToolName, success, e.DurationMs, e.RunID,
		), true

	case "conversation_started", "conversation_ended":
		eventName := e.EventName
		if eventName == "" {
			eventName = e.Type
		}
		return fmt.Sprintf(
			"INSERT INTO events_conversation (timestamp, conversation_id, agent_slug, user_id, event, run_id) VALUES ('%s', '%s', '%s', '%s', '%s', '%s')",
			ts, e.ConversationID, e.AgentSlug, e.UserID, eventName, e.RunID,
		), true

	case "memory_extracted":
		return fmt.Sprintf(
			"INSERT INTO events_memory (timestamp, agent_slug, user_id, memory_type, importance, run_id) VALUES ('%s', '%s', '%s', '%s', %f, '%s')",
			ts, e.AgentSlug, e.UserID, e.MemoryType, e.Importance, e.RunID,
		), true

	case "relationship_snapshot":
		return fmt.Sprintf(
			"INSERT INTO events_relationship (timestamp, agent_slug, user_id, trust, intimacy, autonomy, reciprocity, playfulness, conflict, run_id) VALUES ('%s', '%s', '%s', %f, %f, %f, %f, %f, %f, '%s')",
			ts, e.AgentSlug, e.UserID, e.Trust, e.Intimacy, e.Autonomy, e.Reciprocity, e.Playfulness, e.Conflict, e.RunID,
		), true

	case "consolidation_completed":
		return fmt.Sprintf(
			"INSERT INTO events_consolidation (timestamp, agent_slug, user_id, patterns_found, memories_merged, run_id) VALUES ('%s', '%s', '%s', %d, %d, '%s')",
			ts, e.AgentSlug, e.UserID, e.PatternsFound, e.MemoriesMerged, e.RunID,
		), true

	case "eval_result":
		toolsUsed := "[]"
		if len(e.ToolsUsed) > 0 {
			toolsUsed = string(e.ToolsUsed)
		}
		criteria := "[]"
		if len(e.Criteria) > 0 {
			criteria = string(e.Criteria)
		}
		return fmt.Sprintf(
			"INSERT INTO events_eval (timestamp, run_id, agent_slug, user_id, scenario, response, duration_ms, tools_used, passed, failed, total, criteria) VALUES ('%s', '%s', '%s', '%s', '%s', '%s', %d, '%s', %d, %d, %d, '%s')",
			ts, e.RunID, e.AgentSlug, e.UserID, escapeSingleQuotes(e.Scenario), escapeSingleQuotes(e.Response), e.DurationMs, toolsUsed, e.Passed, e.Failed, e.Total, escapeSingleQuotes(criteria),
		), true

	case "run_started", "run_completed":
		event := "started"
		if e.Type == "run_completed" {
			event = "completed"
		}
		return fmt.Sprintf(
			"INSERT INTO events_run (timestamp, run_id, agent_slug, user_id, event, description) VALUES ('%s', '%s', '%s', '%s', '%s', '%s')",
			ts, e.RunID, e.AgentSlug, e.UserID, event, e.Description,
		), true

	default:
		return "", false
	}
}
