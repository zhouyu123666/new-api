package observability

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/relayconvert"
)

// responsesStreamAggregator rebuilds the answer of an OpenAI Responses stream.
// The aggregation itself is the shared relayconvert accumulator that the
// buffered Responses-to-Chat converter uses, so text, reasoning and tool calls
// survive exactly as they do on the relay path. The terminal frame is what
// reaches the trace; the aggregated output is only spliced in when that frame
// arrives with an empty output array.
type responsesStreamAggregator struct {
	acc      *relayconvert.ResponsesBufferedAccumulator
	final    *dto.ResponsesStreamResponse
	finalRaw string
}

func newResponsesStreamAggregator(int64) streamAggregator {
	return &responsesStreamAggregator{acc: relayconvert.NewResponsesBufferedAccumulator()}
}

func (a *responsesStreamAggregator) accept(data string) bool {
	var event dto.ResponsesStreamResponse
	if err := common.UnmarshalJsonStr(data, &event); err != nil || !strings.HasPrefix(event.Type, "response.") {
		return false
	}
	if a.final != nil {
		return true
	}
	a.acc.ProcessEvent(&event)
	switch event.Type {
	case "response.completed", "response.done", "response.failed", "response.incomplete", "response.cancelled", "response.canceled":
		a.final = &event
		a.finalRaw = data
	}
	return true
}

func (a *responsesStreamAggregator) output() streamOutput {
	// A stream cut short before its terminal event still delivered whatever
	// the aggregator collected, and a partial answer on the trace is worth
	// more than none.
	if a.final == nil {
		output := a.acc.BuildOutput()
		if len(output) == 0 {
			return streamOutput{}
		}
		value, err := common.Marshal(output)
		if err != nil {
			return streamOutput{}
		}
		return streamOutput{value: value}
	}
	value := common.StringToByteSlice(a.finalRaw)
	if a.final.Response == nil || len(a.final.Response.Output) == 0 {
		if spliced, ok := spliceResponsesOutput(a.finalRaw, a.acc.BuildOutput()); ok {
			value = spliced
		}
	}
	return streamOutput{value: value}
}

// spliceResponsesOutput replaces an empty output array in a terminal Responses
// frame with the aggregated stream output, keeping every other field of the
// original frame exactly as the upstream sent it.
func spliceResponsesOutput(raw string, output []dto.ResponsesOutput) ([]byte, bool) {
	if len(output) == 0 {
		return nil, false
	}
	var envelope map[string]any
	if err := common.UnmarshalJsonStr(raw, &envelope); err != nil {
		return nil, false
	}
	response, ok := envelope["response"].(map[string]any)
	if !ok {
		return nil, false
	}
	response["output"] = output
	spliced, err := common.Marshal(envelope)
	return spliced, err == nil
}
