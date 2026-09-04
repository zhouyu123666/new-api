package observability

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/types"
)

// buildLangfuseInput keeps only model-context fields from the final upstream
// request. The shape is intentionally protocol-specific; this is a projection
// for observability, not another cross-protocol request schema.
func buildLangfuseInput(body []byte, format types.RelayFormat) ([]byte, bool) {
	var payload map[string]any
	if err := common.Unmarshal(body, &payload); err != nil {
		return nil, false
	}

	keys := []string{"messages", "tools"}
	switch format {
	case types.RelayFormatOpenAI:
		keys = []string{"messages", "tools"}
	case types.RelayFormatClaude:
		keys = []string{"system", "messages", "tools"}
	case types.RelayFormatOpenAIResponses, types.RelayFormatOpenAIResponsesCompaction:
		keys = []string{"instructions", "input", "tools"}
	default:
		// Do not project non-target protocols into an OpenAI-shaped record.
		// Their observability format is intentionally deferred to a later change.
		return nil, false
	}

	result := make(map[string]any, len(keys))
	for _, key := range keys {
		if value, ok := payload[key]; ok {
			result[key] = value
		}
	}
	if len(result) == 0 {
		return nil, false
	}
	encoded, err := common.Marshal(result)
	if err != nil {
		return nil, false
	}
	return encoded, true
}

// normalizeLangfuseOutput converts supported upstream response envelopes into
// a compact array of semantic assistant output items. Response metadata such as
// ids, model, status, usage, and event type is deliberately discarded.
func normalizeLangfuseOutput(body []byte) ([]byte, bool) {
	var payload any
	if err := common.Unmarshal(body, &payload); err != nil {
		return nil, false
	}

	var output []any
	switch value := payload.(type) {
	case map[string]any:
		if response, ok := value["response"].(map[string]any); ok {
			if items, ok := response["output"].([]any); ok {
				output = filterResponsesOutput(items)
			}
		}
		if output == nil {
			if items, ok := value["output"].([]any); ok {
				output = filterResponsesOutput(items)
			}
		}
		if output == nil {
			if choices, ok := value["choices"].([]any); ok {
				output = filterChatChoices(choices)
			}
		}
		if output == nil {
			if content, ok := value["content"].([]any); ok {
				output = []any{claudeContentToAssistantMessage(value, content)}
			} else if _, hasRole := value["role"]; hasRole {
				output = []any{filterAssistantMessage(value)}
			}
		}
	case []any:
		output = filterResponsesOutput(value)
	}
	if output == nil {
		return nil, false
	}
	encoded, err := common.Marshal(output)
	if err != nil {
		return nil, false
	}
	return encoded, true
}

func filterChatChoices(choices []any) []any {
	result := make([]any, 0, len(choices))
	for _, raw := range choices {
		choice, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		message, ok := choice["message"].(map[string]any)
		if ok {
			result = append(result, filterAssistantMessage(message))
			continue
		}
		if text, ok := choice["text"].(string); ok {
			result = append(result, map[string]any{"role": "assistant", "content": text})
		}
	}
	if len(choices) > 0 && len(result) == 0 {
		return nil
	}
	return result
}

func filterAssistantMessage(message map[string]any) map[string]any {
	result := make(map[string]any)
	for _, key := range []string{"role", "content", "reasoning_content", "reasoning", "tool_calls", "refusal"} {
		if value, ok := message[key]; ok {
			result[key] = value
		}
	}
	if _, ok := result["role"]; !ok {
		result["role"] = "assistant"
	}
	return result
}

func filterResponsesOutput(items []any) []any {
	result := make([]any, 0, len(items))
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if role, hasRole := item["role"].(string); hasRole && role != "" {
			if _, hasType := item["type"].(string); !hasType {
				result = append(result, filterAssistantMessage(item))
				continue
			}
		}
		filtered := make(map[string]any)
		for _, key := range []string{"type", "role", "content", "call_id", "name", "arguments", "input", "result"} {
			if value, exists := item[key]; exists {
				if key == "role" {
					if role, ok := value.(string); ok && role == "" {
						continue
					}
				}
				if key == "content" {
					value = filterResponseContent(value)
				}
				filtered[key] = value
			}
		}
		if len(filtered) > 0 {
			result = append(result, filtered)
		}
	}
	if len(items) > 0 && len(result) == 0 {
		return nil
	}
	return result
}

func filterResponseContent(value any) any {
	items, ok := value.([]any)
	if !ok {
		return value
	}
	result := make([]any, 0, len(items))
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		filtered := make(map[string]any)
		for _, key := range []string{"type", "text"} {
			if value, exists := item[key]; exists {
				filtered[key] = value
			}
		}
		if len(filtered) > 0 {
			result = append(result, filtered)
		}
	}
	return result
}

func claudeContentToAssistantMessage(payload map[string]any, content []any) map[string]any {
	message := map[string]any{"role": "assistant"}
	var textBuilder strings.Builder
	var reasoningBuilder strings.Builder
	var toolCalls []any
	for _, raw := range content {
		block, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		switch block["type"] {
		case "text":
			if text, ok := block["text"].(string); ok {
				textBuilder.WriteString(text)
			}
		case "thinking", "redacted_thinking":
			if thinking, ok := block["thinking"].(string); ok {
				reasoningBuilder.WriteString(thinking)
			}
		case "tool_use":
			call := map[string]any{"type": "function"}
			if id, ok := block["id"]; ok {
				call["id"] = id
			}
			function := map[string]any{}
			if name, ok := block["name"]; ok {
				function["name"] = name
			}
			if input, ok := block["input"]; ok {
				if args, err := common.Marshal(input); err == nil {
					function["arguments"] = string(args)
				}
			}
			call["function"] = function
			toolCalls = append(toolCalls, call)
		}
	}
	message["content"] = textBuilder.String()
	if reasoningBuilder.Len() > 0 {
		message["reasoning_content"] = reasoningBuilder.String()
	}
	if len(toolCalls) > 0 {
		message["tool_calls"] = toolCalls
	}
	if _, ok := payload["role"]; ok {
		message["role"] = payload["role"]
	}
	return message
}
