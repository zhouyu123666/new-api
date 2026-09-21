package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ChannelModelStatus records a temporary routing circuit for one model on one
// channel. A row exists only while the pair is disabled. Keeping this state
// separate from Channel.Models and Channel.ModelMapping avoids mutating the
// administrator's permanent routing configuration.
type ChannelModelStatus struct {
	ChannelId            int    `json:"channel_id" gorm:"primaryKey;autoIncrement:false"`
	Model                string `json:"model" gorm:"type:varchar(255);primaryKey;autoIncrement:false"`
	DisabledTime         int64  `json:"disabled_time" gorm:"bigint;index"`
	LastProbeTime        int64  `json:"last_probe_time" gorm:"bigint;index"`
	RecoverySuccessCount int    `json:"recovery_success_count"`
	Reason               string `json:"reason" gorm:"type:text"`
}

func DisableChannelModel(channelID int, modelName string, reason string) (bool, error) {
	modelName = strings.TrimSpace(modelName)
	if channelID <= 0 || modelName == "" || !operation_setting.IsChannelModelCircuitBreakerEnabled() ||
		operation_setting.IsChannelModelCircuitBreakerExcluded(channelID) {
		return false, nil
	}

	status := ChannelModelStatus{
		ChannelId:    channelID,
		Model:        modelName,
		DisabledTime: common.GetTimestamp(),
		Reason:       reason,
	}
	result := DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&status)
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected == 0 {
		return false, nil
	}
	cacheSetChannelModelDisabled(channelID, modelName, true)
	InvalidatePricingCache()
	return true, nil
}

func EnableChannelModel(channelID int, modelName string) (bool, error) {
	modelName = strings.TrimSpace(modelName)
	if channelID <= 0 || modelName == "" {
		return false, nil
	}

	result := DB.Where("channel_id = ? AND model = ?", channelID, modelName).Delete(&ChannelModelStatus{})
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected == 0 {
		return false, nil
	}
	cacheSetChannelModelDisabled(channelID, modelName, false)
	InvalidatePricingCache()
	return true, nil
}

func ClearChannelModelStatuses(channelID int) error {
	if channelID <= 0 {
		return nil
	}
	if err := DB.Where("channel_id = ?", channelID).Delete(&ChannelModelStatus{}).Error; err != nil {
		return err
	}
	cacheClearChannelModelStatuses(channelID)
	return nil
}

func ClearChannelModelStatusesByChannelIDs(channelIDs []int) error {
	if len(channelIDs) == 0 {
		return nil
	}
	if err := DB.Where("channel_id IN ?", channelIDs).Delete(&ChannelModelStatus{}).Error; err != nil {
		return err
	}
	for _, channelID := range channelIDs {
		cacheClearChannelModelStatuses(channelID)
	}
	return nil
}

func GetChannelModelStatusesByChannelIDs(channelIDs []int) ([]ChannelModelStatus, error) {
	if len(channelIDs) == 0 {
		return []ChannelModelStatus{}, nil
	}
	var statuses []ChannelModelStatus
	err := DB.Where("channel_id IN ?", channelIDs).Order("channel_id ASC").Order("model ASC").Find(&statuses).Error
	return statuses, err
}

func ClearAllChannelModelStatuses() error {
	if err := DB.Where("channel_id > ?", 0).Delete(&ChannelModelStatus{}).Error; err != nil {
		return err
	}
	if common.MemoryCacheEnabled {
		channelSyncLock.Lock()
		channel2disabledModels = make(map[int]map[string]bool)
		channelSyncLock.Unlock()
	}
	InvalidatePricingCache()
	return nil
}

func GetAllChannelModelStatuses() ([]ChannelModelStatus, error) {
	var statuses []ChannelModelStatus
	err := DB.Order("channel_id ASC").Order("model ASC").Find(&statuses).Error
	return statuses, err
}

func PruneChannelModelStatuses(channelID int, models []string) error {
	if channelID <= 0 {
		return nil
	}
	configured := make([]string, 0, len(models))
	for _, modelName := range models {
		if modelName = strings.TrimSpace(modelName); modelName != "" {
			configured = append(configured, modelName)
		}
	}
	disabledModels, err := GetDisabledChannelModels(channelID)
	if err != nil {
		return err
	}
	configuredSet := make(map[string]bool, len(configured))
	for _, modelName := range configured {
		configuredSet[modelName] = true
	}
	stale := make([]string, 0)
	for _, modelName := range disabledModels {
		if configuredSet[modelName] || configuredSet[ratio_setting.FormatMatchingModelName(modelName)] {
			continue
		}
		stale = append(stale, modelName)
	}
	if len(stale) > 0 {
		if err := DB.Where("channel_id = ? AND model IN ?", channelID, stale).Delete(&ChannelModelStatus{}).Error; err != nil {
			return err
		}
	}
	cachePruneChannelModelStatuses(channelID, disabledModels, stale)
	return nil
}

func GetDisabledChannelModels(channelID int) ([]string, error) {
	models := make([]string, 0)
	if !operation_setting.IsChannelModelCircuitBreakerEnabled() {
		return models, nil
	}
	err := DB.Model(&ChannelModelStatus{}).
		Where("channel_id = ?", channelID).
		Order("model").
		Pluck("model", &models).Error
	return models, err
}

func GetChannelIDsWithDisabledModels() (map[int]bool, error) {
	if !operation_setting.IsChannelModelCircuitBreakerEnabled() {
		return map[int]bool{}, nil
	}
	channelIDs := make([]int, 0)
	err := DB.Model(&ChannelModelStatus{}).Distinct("channel_id").Pluck("channel_id", &channelIDs).Error
	if err != nil {
		return nil, err
	}
	result := make(map[int]bool, len(channelIDs))
	for _, channelID := range channelIDs {
		result[channelID] = true
	}
	return result, nil
}

func HasDueChannelModelRecovery(now int64, delaySeconds int64, intervalSeconds int64) bool {
	var count int64
	return dueChannelModelRecoveryQuery(now, delaySeconds, intervalSeconds).Count(&count).Error == nil && count > 0
}

func GetDueChannelModelRecoveries(now int64, delaySeconds int64, intervalSeconds int64, limit int) ([]ChannelModelStatus, error) {
	if limit < 1 {
		limit = 1000
	}
	var statuses []ChannelModelStatus
	err := dueChannelModelRecoveryQuery(now, delaySeconds, intervalSeconds).
		Order("channel_model_statuses.disabled_time ASC").
		Order("channel_model_statuses.channel_id ASC").
		Order("channel_model_statuses.model ASC").
		Limit(limit).
		Find(&statuses).Error
	return statuses, err
}

func dueChannelModelRecoveryQuery(now int64, delaySeconds int64, intervalSeconds int64) *gorm.DB {
	if delaySeconds < 0 {
		delaySeconds = 0
	}
	if intervalSeconds < 1 {
		intervalSeconds = 1
	}
	query := DB.Model(&ChannelModelStatus{}).
		Joins("JOIN channels ON channels.id = channel_model_statuses.channel_id").
		Where("channels.status = ?", common.ChannelStatusEnabled).
		Where("channel_model_statuses.disabled_time <= ?", now-delaySeconds).
		Where("channel_model_statuses.last_probe_time = ? OR channel_model_statuses.last_probe_time <= ?", 0, now-intervalSeconds)
	excluded, _, err := operation_setting.ParseChannelModelExcludedChannelIDs(operation_setting.GetMonitorSetting().ChannelModelExcludedChannelIDs)
	if err == nil && len(excluded) > 0 {
		query = query.Where("channel_model_statuses.channel_id NOT IN ?", excluded)
	}
	return query
}

func RecordChannelModelRecoveryProbe(channelID int, modelName string, success bool, probeTime int64) (int, bool, error) {
	updates := map[string]any{"last_probe_time": probeTime}
	if success {
		updates["recovery_success_count"] = gorm.Expr("recovery_success_count + ?", 1)
	} else {
		updates["recovery_success_count"] = 0
	}
	result := DB.Model(&ChannelModelStatus{}).
		Where("channel_id = ? AND model = ?", channelID, modelName).
		Updates(updates)
	if result.Error != nil {
		return 0, false, result.Error
	}
	if result.RowsAffected == 0 {
		return 0, false, nil
	}
	var status ChannelModelStatus
	if err := DB.Where("channel_id = ? AND model = ?", channelID, modelName).First(&status).Error; err != nil {
		return 0, false, err
	}
	return status.RecoverySuccessCount, true, nil
}

func IsChannelModelDisabled(channelID int, modelName string) bool {
	modelName = strings.TrimSpace(modelName)
	if channelID <= 0 || modelName == "" || !operation_setting.IsChannelModelCircuitBreakerEnabled() ||
		operation_setting.IsChannelModelCircuitBreakerExcluded(channelID) {
		return false
	}
	if common.MemoryCacheEnabled {
		channelSyncLock.RLock()
		defer channelSyncLock.RUnlock()
		return isChannelModelDisabledLocked(channelID, modelName)
	}
	var count int64
	if err := DB.Model(&ChannelModelStatus{}).
		Where("channel_id = ? AND model = ?", channelID, modelName).
		Count(&count).Error; err != nil {
		common.SysLog("failed to query channel model status: " + err.Error())
		return false
	}
	return count > 0
}
