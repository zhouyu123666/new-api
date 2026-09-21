package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const (
	ChannelModelEventDisabled     = "disabled"
	ChannelModelEventProbeSuccess = "probe_success"
	ChannelModelEventProbeFailed  = "probe_failed"
	ChannelModelEventRecovered    = "recovered"
)

type ChannelModelEvent struct {
	Id           int64  `json:"id" gorm:"primaryKey"`
	ChannelId    int    `json:"channel_id" gorm:"index"`
	ChannelName  string `json:"channel_name" gorm:"type:varchar(255)"`
	Model        string `json:"model" gorm:"type:varchar(255);index"`
	Event        string `json:"event" gorm:"type:varchar(32);index"`
	StatusCode   int    `json:"status_code"`
	SuccessCount int    `json:"success_count"`
	Reason       string `json:"reason" gorm:"type:text"`
	CreatedAt    int64  `json:"created_at" gorm:"bigint;index"`
}

func RecordChannelModelEvent(event *ChannelModelEvent) error {
	if event == nil || event.ChannelId <= 0 || strings.TrimSpace(event.Model) == "" || strings.TrimSpace(event.Event) == "" {
		return nil
	}
	if event.CreatedAt == 0 {
		event.CreatedAt = common.GetTimestamp()
	}
	return DB.Create(event).Error
}

type ChannelModelEventQuery struct {
	ChannelId int
	Model     string
	Event     string
	StartTime int64
	EndTime   int64
	Offset    int
	Limit     int
}

func GetChannelModelEvents(query ChannelModelEventQuery) ([]ChannelModelEvent, int64, error) {
	db := DB.Model(&ChannelModelEvent{})
	if query.ChannelId > 0 {
		db = db.Where("channel_id = ?", query.ChannelId)
	}
	if modelName := strings.TrimSpace(query.Model); modelName != "" {
		db = db.Where("model = ?", modelName)
	}
	if event := strings.TrimSpace(query.Event); event != "" {
		db = db.Where("event = ?", event)
	}
	if query.StartTime > 0 {
		db = db.Where("created_at >= ?", query.StartTime)
	}
	if query.EndTime > 0 {
		db = db.Where("created_at <= ?", query.EndTime)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	limit := query.Limit
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	var events []ChannelModelEvent
	err := db.Order("id DESC").Offset(max(query.Offset, 0)).Limit(limit).Find(&events).Error
	return events, total, err
}

func CleanupChannelModelEvents(before int64) (int64, error) {
	if before <= 0 {
		return 0, nil
	}
	result := DB.Where("created_at < ?", before).Delete(&ChannelModelEvent{})
	return result.RowsAffected, result.Error
}

func HasChannelModelEventsBefore(before int64) bool {
	if before <= 0 {
		return false
	}
	var count int64
	return DB.Model(&ChannelModelEvent{}).Where("created_at < ?", before).Count(&count).Error == nil && count > 0
}
