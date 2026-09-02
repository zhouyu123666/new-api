package observability

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/relayconvert"
)

// maxClaudeStreamToolCalls bounds the tool-call slice so a malformed upstream
// index cannot force a huge allocation.
const maxClaudeStreamToolCalls = 1024

// assistantStreamMessage is the assistant turn a stream amounted to, in the
// OpenAI message shape the relay converters already normalize every upstream
// protocol into.
type assistantStreamMessage struct {
	Role             string                 `json:"role"`
	Content          string                 `json:"content"`
	ReasoningContent string                 `json:"reasoning_content,omitempty"`
	ToolCalls        []dto.ToolCallResponse `json:"tool_calls,omitempty"`
}

// claudeStreamAggregator rebuilds the answer of a Claude Messages stream by
// replaying every frame through the same converter the relay path uses to serve
// a Claude upstream in OpenAI format. Anthropic never resends the finished
// message, so the answer only exists as the sum of the frames — and the trace
// reads them through the implementation the client already depends on rather
// than through a second parser.
type claudeStreamAggregator struct {
	info      relayconvert.ClaudeResponseInfo
	text      strings.Builder
	reasoning strings.Builder
	toolCalls []claudeStreamToolCall
}

// claudeStreamToolCall keeps a tool call under the content block index that
// carries its argument fragments; the indexes are sparse because text and
// thinking blocks occupy some of them.
type claudeStreamToolCall struct {
	index int
	call  dto.ToolCallResponse
}

func newClaudeStreamAggregator(int64) streamAggregator { return &claudeStreamAggregator{} }

func (a *claudeStreamAggregator) accept(data string) bool {
	var event dto.ClaudeResponse
	if err := common.UnmarshalJsonStr(data, &event); err != nil {
		return false
	}
	// A tool-argument frame without its fragment would make the shared
	// converter dereference a nil pointer, and it carries nothing to collect.
	if event.Delta != nil && event.Delta.Type == "input_json_delta" && event.Delta.PartialJson == nil {
		return event.Type == "content_block_delta"
	}
	chunk := relayconvert.StreamResponseClaude2OpenAI(&event)
	if !relayconvert.FormatClaudeResponseInfo(&event, chunk, &a.info) {
		// The converter has nothing to do for the frames that only close what
		// earlier frames delivered, but they still belong to this stream.
		return event.Type == "content_block_stop" || event.Type == "message_stop"
	}
	if chunk == nil {
		return true
	}
	for i := range chunk.Choices {
		delta := &chunk.Choices[i].Delta
		if delta.Content != nil {
			a.text.WriteString(*delta.Content)
		}
		if delta.ReasoningContent != nil {
			a.reasoning.WriteString(*delta.ReasoningContent)
		}
		for _, call := range delta.ToolCalls {
			a.mergeToolCall(call)
		}
	}
	return true
}

// mergeToolCall folds one converted tool-call fragment into the call its
// content block index owns: the name and id arrive with the block, the
// arguments arrive as JSON fragments spread over later frames.
func (a *claudeStreamAggregator) mergeToolCall(call dto.ToolCallResponse) {
	index := 0
	if call.Index != nil {
		index = *call.Index
	}
	for i := range a.toolCalls {
		if a.toolCalls[i].index != index {
			continue
		}
		target := &a.toolCalls[i].call
		if call.ID != "" {
			target.ID = call.ID
		}
		if call.Function.Name != "" {
			target.Function.Name = call.Function.Name
		}
		target.Function.Arguments += call.Function.Arguments
		return
	}
	if len(a.toolCalls) >= maxClaudeStreamToolCalls {
		return
	}
	// The stream index is a position inside this stream and means nothing on a
	// finished message.
	call.Index = nil
	call.Type = "function"
	a.toolCalls = append(a.toolCalls, claudeStreamToolCall{index: index, call: call})
}

func (a *claudeStreamAggregator) output() streamOutput {
	if a.text.Len() == 0 && a.reasoning.Len() == 0 && len(a.toolCalls) == 0 {
		return streamOutput{}
	}
	message := assistantStreamMessage{
		Role:             "assistant",
		Content:          a.text.String(),
		ReasoningContent: a.reasoning.String(),
	}
	for _, tool := range a.toolCalls {
		message.ToolCalls = append(message.ToolCalls, tool.call)
	}
	value, err := common.Marshal([]assistantStreamMessage{message})
	if err != nil {
		return streamOutput{}
	}
	return streamOutput{value: value}
}
