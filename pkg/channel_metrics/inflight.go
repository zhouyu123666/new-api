// Package channel_metrics tracks live per-channel relay state that cannot be
// derived from the log tables.
//
// The channel status health bar reads its success/failure history straight from
// the logs, but the number of requests currently being relayed is real-time
// state that no table holds. Counts are node-local on purpose: an in-flight
// counter kept in Redis would drift upward forever whenever a node dies between
// the increment and the decrement.
package channel_metrics

import "sync"

var (
	inFlightMu sync.RWMutex
	inFlight   = make(map[int]int64)
)

// IncInFlight marks one relay attempt against the channel as started.
func IncInFlight(channelId int) {
	if channelId <= 0 {
		return
	}
	inFlightMu.Lock()
	defer inFlightMu.Unlock()
	inFlight[channelId]++
}

// DecInFlight marks one relay attempt against the channel as finished. It must
// be paired with IncInFlight via defer so a panic unwinding through the relay
// still releases the counter.
func DecInFlight(channelId int) {
	if channelId <= 0 {
		return
	}
	inFlightMu.Lock()
	defer inFlightMu.Unlock()
	remaining := inFlight[channelId] - 1
	if remaining <= 0 {
		delete(inFlight, channelId)
		return
	}
	inFlight[channelId] = remaining
}

// GetInFlight returns the current in-flight counts of the given channels on
// this node, omitting channels that are idle.
func GetInFlight(channelIds []int) map[int]int64 {
	result := make(map[int]int64, len(channelIds))
	if len(channelIds) == 0 {
		return result
	}
	inFlightMu.RLock()
	defer inFlightMu.RUnlock()
	for _, channelId := range channelIds {
		if count := inFlight[channelId]; count > 0 {
			result[channelId] = count
		}
	}
	return result
}
