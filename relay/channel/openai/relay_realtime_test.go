package openai

import (
	"testing"

	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/require"
)

func TestAccumulateUsageFallsBackToInputAndOutputWhenTotalMissing(t *testing.T) {
	total := &dto.RealtimeUsage{}
	segment := &dto.RealtimeUsage{
		InputTokens:  7,
		OutputTokens: 3,
	}

	require.NoError(t, accumulateUsage(segment, total))
	require.Equal(t, 10, total.TotalTokens)
	require.Equal(t, 7, total.InputTokens)
	require.Equal(t, 3, total.OutputTokens)
}
