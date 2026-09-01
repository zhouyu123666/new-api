/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func useSQLiteLogTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Log{}))

	previousDB, previousLogDB := DB, LOG_DB
	previousLogDatabaseType := common.LogDatabaseType()
	DB, LOG_DB = db, db
	common.SetLogDatabaseType(common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		DB, LOG_DB = previousDB, previousLogDB
		common.SetLogDatabaseType(previousLogDatabaseType)
	})
	return db
}

func TestGetAllLogsStreamErrorFilter(t *testing.T) {
	db := useSQLiteLogTestDB(t)
	streamError := common.MapToJsonStr(map[string]interface{}{
		"stream_status": map[string]interface{}{"status": "error"},
	})
	streamOk := common.MapToJsonStr(map[string]interface{}{
		"stream_status": map[string]interface{}{"status": "ok"},
	})
	require.NoError(t, db.Create(&[]Log{
		{Type: LogTypeConsume, Other: streamError},
		{Type: LogTypeConsume, Other: streamOk},
		{Type: LogTypeConsume, Other: ""},
	}).Error)

	logs, total, err := GetAllLogs(
		LogTypeUnknown, 0, 0, "", "", "", 0, 20, 0, "", "", "", true, false,
	)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, logs, 1)
	assert.Equal(t, streamError, logs[0].Other)
}

func TestGetAllLogsRetryFilter(t *testing.T) {
	db := useSQLiteLogTestDB(t)
	retryOther := common.MapToJsonStr(map[string]interface{}{
		"admin_info": map[string]interface{}{
			"use_channel": []string{"1", "2"},
		},
	})
	singleChannelOther := common.MapToJsonStr(map[string]interface{}{
		"admin_info": map[string]interface{}{
			"use_channel": []string{"1"},
		},
	})
	require.NoError(t, db.Create(&[]Log{
		{Type: LogTypeConsume, Other: retryOther},
		{Type: LogTypeConsume, Other: singleChannelOther},
		{Type: LogTypeConsume, Other: ""},
	}).Error)

	logs, total, err := GetAllLogs(
		LogTypeUnknown, 0, 0, "", "", "", 0, 20, 0, "", "", "", false, true,
	)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, logs, 1)
	assert.Equal(t, retryOther, logs[0].Other)
}

func TestSpecialLogFiltersFailClosedForUnsupportedDatabase(t *testing.T) {
	db := useSQLiteLogTestDB(t)
	common.SetLogDatabaseType(common.DatabaseType("unsupported"))
	other := common.MapToJsonStr(map[string]interface{}{
		"admin_info": map[string]interface{}{
			"use_channel": []string{"1", "2"},
		},
		"stream_status": map[string]interface{}{"status": "error"},
	})
	require.NoError(t, db.Create(&Log{Type: LogTypeConsume, Other: other}).Error)

	streamLogs, streamTotal, err := GetAllLogs(
		LogTypeUnknown, 0, 0, "", "", "", 0, 20, 0, "", "", "", true, false,
	)
	require.NoError(t, err)
	assert.Zero(t, streamTotal)
	assert.Empty(t, streamLogs)

	retryLogs, retryTotal, err := GetAllLogs(
		LogTypeUnknown, 0, 0, "", "", "", 0, 20, 0, "", "", "", false, true,
	)
	require.NoError(t, err)
	assert.Zero(t, retryTotal)
	assert.Empty(t, retryLogs)
}
