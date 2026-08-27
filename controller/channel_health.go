package controller

import (
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	channelmetrics "github.com/QuantumNous/new-api/pkg/channel_metrics"

	"github.com/gin-gonic/gin"
)

// channelHealthMaxIds mirrors the model-side bound so an oversized request is
// rejected before it reaches the log database.
const channelHealthMaxIds = 200

type channelHealthItem struct {
	ChannelId int                         `json:"channel_id"`
	Buckets   []model.ChannelHealthBucket `json:"buckets"`
	InFlight  int64                       `json:"in_flight"`
}

// GetChannelHealth returns the success/failure history of the requested
// channels split into fixed-width blocks, plus the number of relay attempts
// currently in flight on this node.
//
// The response is scoped to the channel ids of one table page: a pool-wide
// aggregation over the log table would be far too expensive to run on every
// refresh of the channel list.
func GetChannelHealth(c *gin.Context) {
	rawIds := strings.TrimSpace(c.Query("ids"))
	if rawIds == "" {
		common.ApiSuccess(c, gin.H{
			"items":         []channelHealthItem{},
			"block_count":   model.ChannelHealthBlockCount,
			"block_seconds": model.ChannelHealthBlockSeconds,
			"start_ts":      time.Now().Unix(),
		})
		return
	}

	parts := strings.Split(rawIds, ",")
	if len(parts) > channelHealthMaxIds {
		common.ApiErrorMsg(c, "请求的渠道数量过多")
		return
	}
	channelIds := make([]int, 0, len(parts))
	seen := make(map[int]bool, len(parts))
	for _, part := range parts {
		channelId, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || channelId <= 0 || seen[channelId] {
			continue
		}
		seen[channelId] = true
		channelIds = append(channelIds, channelId)
	}

	// The window is not aligned to wall-clock boundaries: the last block always
	// ends at the current moment so the bar reflects the newest traffic.
	windowSeconds := int64(model.ChannelHealthBlockCount) * int64(model.ChannelHealthBlockSeconds)
	startTs := time.Now().Unix() - windowSeconds

	buckets, err := model.GetChannelHealthBuckets(channelIds, startTs)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	inFlight := channelmetrics.GetInFlight(channelIds)

	items := make([]channelHealthItem, 0, len(channelIds))
	for _, channelId := range channelIds {
		item := channelHealthItem{
			ChannelId: channelId,
			InFlight:  inFlight[channelId],
		}
		if channelBuckets, ok := buckets[channelId]; ok {
			item.Buckets = channelBuckets
		}
		items = append(items, item)
	}

	common.ApiSuccess(c, gin.H{
		"items":         items,
		"block_count":   model.ChannelHealthBlockCount,
		"block_seconds": model.ChannelHealthBlockSeconds,
		"start_ts":      startTs,
	})
}
