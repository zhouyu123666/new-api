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
	acc   *relayconvert.ResponsesBufferedAccumulator
	final *dto.ResponsesStreamResponse
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
	var output []dto.ResponsesOutput
	if a.final.Response != nil {
		output = a.final.Response.Output
	}
	if len(output) == 0 {
		output = a.acc.BuildOutput()
	}
	if len(output) == 0 {
		return streamOutput{}
	}
	value, err := common.Marshal(output)
	if err != nil {
		return streamOutput{}
	}
	return streamOutput{value: value}
}
