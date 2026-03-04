package anal

import (
	"encoding/json"
	"time"
)

// Event is a typed analytics event from a producer service.
type Event struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`

	// Common fields
	AgentSlug      string `json:"agent_slug,omitempty"`
	UserID         string `json:"user_id,omitempty"`
	ConversationID string `json:"conversation_id,omitempty"`
	RunID          string `json:"run_id,omitempty"`

	// run fields
	Description string `json:"description,omitempty"`

	// message fields
	Role             string `json:"role,omitempty"`
	PromptTokens     uint32 `json:"prompt_tokens,omitempty"`
	CompletionTokens uint32 `json:"completion_tokens,omitempty"`
	DurationMs       uint32 `json:"duration_ms,omitempty"`

	// tool_invocation fields
	ToolName string `json:"tool_name,omitempty"`
	Success  *bool  `json:"success,omitempty"`

	// conversation fields
	EventName string `json:"event_name,omitempty"`

	// memory fields
	MemoryType string  `json:"memory_type,omitempty"`
	Importance float32 `json:"importance,omitempty"`

	// relationship fields
	Trust       float32 `json:"trust,omitempty"`
	Intimacy    float32 `json:"intimacy,omitempty"`
	Autonomy    float32 `json:"autonomy,omitempty"`
	Reciprocity float32 `json:"reciprocity,omitempty"`
	Playfulness float32 `json:"playfulness,omitempty"`
	Conflict    float32 `json:"conflict,omitempty"`

	// consolidation fields
	PatternsFound  uint16 `json:"patterns_found,omitempty"`
	MemoriesMerged uint16 `json:"memories_merged,omitempty"`

	// eval_result fields
	Scenario  string          `json:"scenario,omitempty"`
	Response  string          `json:"response,omitempty"`
	ToolsUsed json.RawMessage `json:"tools_used,omitempty"`
	Passed    uint16          `json:"passed,omitempty"`
	Failed    uint16          `json:"failed,omitempty"`
	Total     uint16          `json:"total,omitempty"`
	Criteria  json.RawMessage `json:"criteria,omitempty"`
}
