package observability

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
)

// openAIChatStreamAggregator collects the assistant text of an OpenAI chat
// completions stream, and of the legacy completions stream that shares its
// choices shape. Neither carries a final frame worth reproducing, so both are
// reported as a single assistant message.
type openAIChatStreamAggregator struct {
	text      strings.Builder
	maxBytes  int64
	truncated bool
}

func newOpenAIChatStreamAggregator(maxBytes int64) streamAggregator {
	return &openAIChatStreamAggregator{maxBytes: maxBytes}
}

func (a *openAIChatStreamAggregator) accept(data string) bool {
	var chunk dto.ChatCompletionsStreamResponse
	if err := common.UnmarshalJsonStr(data, &chunk); err != nil || len(chunk.Choices) == 0 {
		return false
	}
	appended := false
	for i := range chunk.Choices {
		if text := chunk.Choices[i].Delta.GetContentString(); text != "" {
			a.appendText(text)
			appended = true
		}
	}
	if appended {
		return true
	}
	// Legacy completions carry their text outside the chat delta shape.
	var completions dto.CompletionsStreamResponse
	if err := common.UnmarshalJsonStr(data, &completions); err == nil {
		for _, choice := range completions.Choices {
			a.appendText(choice.Text)
		}
	}
	return true
}

func (a *openAIChatStreamAggregator) appendText(text string) {
	if a.truncated || text == "" {
		return
	}
	if a.maxBytes > 0 && int64(a.text.Len()+len(text)) > a.maxBytes {
		if remaining := a.maxBytes - int64(a.text.Len()); remaining > 0 {
			a.text.WriteString(text[:remaining])
		}
		a.truncated = true
		return
	}
	a.text.WriteString(text)
}

func (a *openAIChatStreamAggregator) output() streamOutput {
	if a.text.Len() == 0 {
		return streamOutput{}
	}
	value, err := common.Marshal([]map[string]string{{
		"role":    "assistant",
		"content": a.text.String(),
	}})
	if err != nil {
		return streamOutput{}
	}
	// The text was capped while it was collected, so the JSON envelope has to
	// survive whole: truncated text on the trace beats no output at all.
	return streamOutput{value: value, selfCapped: true, truncated: a.truncated}
}
