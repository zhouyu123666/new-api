package channel_metrics

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetInFlight(t *testing.T) {
	t.Helper()
	inFlightMu.Lock()
	inFlight = make(map[int]int64)
	inFlightMu.Unlock()
}

func TestInFlightTracksPerChannelCounts(t *testing.T) {
	resetInFlight(t)

	IncInFlight(7)
	IncInFlight(7)
	IncInFlight(9)

	counts := GetInFlight([]int{7, 9, 11})
	assert.EqualValues(t, 2, counts[7])
	assert.EqualValues(t, 1, counts[9])
	// Idle channels are omitted rather than reported as zero.
	assert.NotContains(t, counts, 11)

	DecInFlight(7)
	DecInFlight(9)

	counts = GetInFlight([]int{7, 9})
	assert.EqualValues(t, 1, counts[7])
	assert.NotContains(t, counts, 9)
}

func TestInFlightReturnsToEmptyAfterBalancedPairs(t *testing.T) {
	resetInFlight(t)

	// An unpaired decrement must not push a channel negative, otherwise a
	// single stray call would permanently hide real traffic.
	DecInFlight(3)
	assert.Empty(t, GetInFlight([]int{3}))

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			IncInFlight(3)
			DecInFlight(3)
		}()
	}
	wg.Wait()

	assert.Empty(t, GetInFlight([]int{3}))
	inFlightMu.RLock()
	remaining := len(inFlight)
	inFlightMu.RUnlock()
	// Balanced pairs must not leak map entries for idle channels.
	require.Zero(t, remaining)
}

func TestInFlightIgnoresInvalidChannelIds(t *testing.T) {
	resetInFlight(t)

	IncInFlight(0)
	IncInFlight(-1)
	DecInFlight(0)

	assert.Empty(t, GetInFlight([]int{0, -1}))
	assert.Empty(t, GetInFlight(nil))
}
