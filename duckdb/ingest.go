package duckdb

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/benaskins/axon"
	look "github.com/benaskins/axon-look"
)

// Inserter executes insert statements against the backing DuckDB.
type Inserter interface {
	Exec(ctx context.Context, query string, params map[string]string) error
}

type ingestHandler struct {
	db Inserter
}

func (h *ingestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var events []look.Event
	if err := json.NewDecoder(r.Body).Decode(&events); err != nil {
		axon.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	for _, e := range events {
		query, params, ok := insertQuery(e)
		if !ok {
			slog.Warn("unknown event type", "type", e.Type)
			continue
		}
		if err := h.db.Exec(r.Context(), query, params); err != nil {
			slog.Error("failed to insert event", "type", e.Type, "error", err)
			// Continue -- don't fail the batch for one bad event.
		}
	}

	w.WriteHeader(http.StatusAccepted)
}

// insertQuery mirrors look.insertQuery but keeps column lists and placeholders
// that make sense for DuckDB. Placeholders still use ClickHouse `{name:Type}`
// syntax so the rewriter in client.go coerces each string param into a typed
// value before binding.
func insertQuery(e look.Event) (string, map[string]string, bool) {
	ts := e.Timestamp.UTC().Format("2006-01-02 15:04:05.000")

	switch e.Type {
	case "message":
		query := "INSERT INTO events_message (timestamp, conversation_id, agent_slug, user_id, role, prompt_tokens, completion_tokens, duration_ms, run_id) VALUES ({ts:DateTime64(3)}, {conversation_id:String}, {agent_slug:String}, {user_id:String}, {role:String}, {prompt_tokens:UInt32}, {completion_tokens:UInt32}, {duration_ms:UInt32}, {run_id:String})"
		params := map[string]string{
			"ts":                ts,
			"conversation_id":   e.ConversationID,
			"agent_slug":        e.AgentSlug,
			"user_id":           e.UserID,
			"role":              e.Role,
			"prompt_tokens":     fmt.Sprintf("%d", e.PromptTokens),
			"completion_tokens": fmt.Sprintf("%d", e.CompletionTokens),
			"duration_ms":       fmt.Sprintf("%d", e.DurationMs),
			"run_id":            e.RunID,
		}
		return query, params, true

	case "tool_invocation":
		success := "false"
		if e.Success != nil && *e.Success {
			success = "true"
		}
		query := "INSERT INTO events_tool_invocation (timestamp, conversation_id, agent_slug, user_id, tool_name, success, duration_ms, run_id) VALUES ({ts:DateTime64(3)}, {conversation_id:String}, {agent_slug:String}, {user_id:String}, {tool_name:String}, {success:Bool}, {duration_ms:UInt32}, {run_id:String})"
		params := map[string]string{
			"ts":              ts,
			"conversation_id": e.ConversationID,
			"agent_slug":      e.AgentSlug,
			"user_id":         e.UserID,
			"tool_name":       e.ToolName,
			"success":         success,
			"duration_ms":     fmt.Sprintf("%d", e.DurationMs),
			"run_id":          e.RunID,
		}
		return query, params, true

	case "conversation_started", "conversation_ended":
		eventName := e.EventName
		if eventName == "" {
			eventName = e.Type
		}
		query := "INSERT INTO events_conversation (timestamp, conversation_id, agent_slug, user_id, event, run_id) VALUES ({ts:DateTime64(3)}, {conversation_id:String}, {agent_slug:String}, {user_id:String}, {event:String}, {run_id:String})"
		params := map[string]string{
			"ts":              ts,
			"conversation_id": e.ConversationID,
			"agent_slug":      e.AgentSlug,
			"user_id":         e.UserID,
			"event":           eventName,
			"run_id":          e.RunID,
		}
		return query, params, true

	case "memory_extracted":
		query := "INSERT INTO events_memory (timestamp, agent_slug, user_id, memory_type, importance, run_id) VALUES ({ts:DateTime64(3)}, {agent_slug:String}, {user_id:String}, {memory_type:String}, {importance:Float32}, {run_id:String})"
		params := map[string]string{
			"ts":          ts,
			"agent_slug":  e.AgentSlug,
			"user_id":     e.UserID,
			"memory_type": e.MemoryType,
			"importance":  fmt.Sprintf("%f", e.Importance),
			"run_id":      e.RunID,
		}
		return query, params, true

	case "relationship_snapshot":
		query := "INSERT INTO events_relationship (timestamp, agent_slug, user_id, trust, intimacy, autonomy, reciprocity, playfulness, conflict, run_id) VALUES ({ts:DateTime64(3)}, {agent_slug:String}, {user_id:String}, {trust:Float32}, {intimacy:Float32}, {autonomy:Float32}, {reciprocity:Float32}, {playfulness:Float32}, {conflict:Float32}, {run_id:String})"
		params := map[string]string{
			"ts":          ts,
			"agent_slug":  e.AgentSlug,
			"user_id":     e.UserID,
			"trust":       fmt.Sprintf("%f", e.Trust),
			"intimacy":    fmt.Sprintf("%f", e.Intimacy),
			"autonomy":    fmt.Sprintf("%f", e.Autonomy),
			"reciprocity": fmt.Sprintf("%f", e.Reciprocity),
			"playfulness": fmt.Sprintf("%f", e.Playfulness),
			"conflict":    fmt.Sprintf("%f", e.Conflict),
			"run_id":      e.RunID,
		}
		return query, params, true

	case "consolidation_completed":
		query := "INSERT INTO events_consolidation (timestamp, agent_slug, user_id, patterns_found, memories_merged, run_id) VALUES ({ts:DateTime64(3)}, {agent_slug:String}, {user_id:String}, {patterns_found:UInt16}, {memories_merged:UInt16}, {run_id:String})"
		params := map[string]string{
			"ts":              ts,
			"agent_slug":      e.AgentSlug,
			"user_id":         e.UserID,
			"patterns_found":  fmt.Sprintf("%d", e.PatternsFound),
			"memories_merged": fmt.Sprintf("%d", e.MemoriesMerged),
			"run_id":          e.RunID,
		}
		return query, params, true

	case "eval_result":
		toolsUsed := "[]"
		if len(e.ToolsUsed) > 0 {
			toolsUsed = string(e.ToolsUsed)
		}
		criteria := "[]"
		if len(e.Criteria) > 0 {
			criteria = string(e.Criteria)
		}
		query := "INSERT INTO events_eval (timestamp, run_id, agent_slug, user_id, scenario, response, duration_ms, tools_used, passed, failed, total, criteria) VALUES ({ts:DateTime64(3)}, {run_id:String}, {agent_slug:String}, {user_id:String}, {scenario:String}, {response:String}, {duration_ms:UInt32}, {tools_used:String}, {passed:UInt16}, {failed:UInt16}, {total:UInt16}, {criteria:String})"
		params := map[string]string{
			"ts":          ts,
			"run_id":      e.RunID,
			"agent_slug":  e.AgentSlug,
			"user_id":     e.UserID,
			"scenario":    e.Scenario,
			"response":    e.Response,
			"duration_ms": fmt.Sprintf("%d", e.DurationMs),
			"tools_used":  toolsUsed,
			"passed":      fmt.Sprintf("%d", e.Passed),
			"failed":      fmt.Sprintf("%d", e.Failed),
			"total":       fmt.Sprintf("%d", e.Total),
			"criteria":    criteria,
		}
		return query, params, true

	case "run_started", "run_completed":
		event := "started"
		if e.Type == "run_completed" {
			event = "completed"
		}
		query := "INSERT INTO events_run (timestamp, run_id, agent_slug, user_id, event, description) VALUES ({ts:DateTime64(3)}, {run_id:String}, {agent_slug:String}, {user_id:String}, {event:String}, {description:String})"
		params := map[string]string{
			"ts":          ts,
			"run_id":      e.RunID,
			"agent_slug":  e.AgentSlug,
			"user_id":     e.UserID,
			"event":       event,
			"description": e.Description,
		}
		return query, params, true

	default:
		return "", nil, false
	}
}
