package controller

import (
	"context"
	"fmt"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func setupLogExportTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	originalMainDatabaseType := common.MainDatabaseType()
	originalLogDatabaseType := common.LogDatabaseType()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	common.MemoryCacheEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	model.DB = db
	model.LOG_DB = db

	if err = db.AutoMigrate(&model.Token{}, &model.Log{}); err != nil {
		t.Fatalf("failed to migrate tables: %v", err)
	}

	t.Cleanup(func() {
		common.SetDatabaseTypes(originalMainDatabaseType, originalLogDatabaseType)
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func seedExportToken(t *testing.T, db *gorm.DB, userID int, name string, rawKey string) *model.Token {
	t.Helper()

	token := &model.Token{
		UserId:         userID,
		Name:           name,
		Key:            rawKey,
		Status:         common.TokenStatusEnabled,
		CreatedTime:    1,
		AccessedTime:   1,
		ExpiredTime:    -1,
		RemainQuota:    100,
		UnlimitedQuota: true,
		Group:          "default",
	}
	if err := db.Create(token).Error; err != nil {
		t.Fatalf("failed to create token: %v", err)
	}
	return token
}

func seedExportLog(t *testing.T, db *gorm.DB, log *model.Log) {
	t.Helper()
	if err := db.Create(log).Error; err != nil {
		t.Fatalf("failed to create log: %v", err)
	}
}

func TestExportLogSummaryIncludesZeroUsageTokens(t *testing.T) {
	db := setupLogExportTestDB(t)
	tokenA := seedExportToken(t, db, 1, "alpha-key", "abcd1234efgh5678")
	seedExportToken(t, db, 2, "beta-key", "ijkl1234mnop5678")

	seedExportLog(t, db, &model.Log{
		UserId:           1,
		CreatedAt:        100,
		Type:             model.LogTypeConsume,
		Username:         "alice",
		TokenName:        tokenA.Name,
		TokenId:          tokenA.Id,
		ModelName:        "gpt-4o",
		Quota:            2000,
		PromptTokens:     100,
		CompletionTokens: 50,
	})
	seedExportLog(t, db, &model.Log{
		UserId:    1,
		CreatedAt: 120,
		Type:      model.LogTypeError,
		Username:  "alice",
		TokenName: tokenA.Name,
		TokenId:   tokenA.Id,
		ModelName: "gpt-4o",
	})

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/log/export/summary?start_timestamp=50&end_timestamp=200", nil, 1)
	ctx.Set("role", common.RoleAdminUser)
	ExportLogSummary(ctx)

	if contentType := recorder.Header().Get("Content-Type"); !strings.Contains(contentType, "application/vnd.ms-excel") {
		t.Fatalf("expected excel content type, got %q", contentType)
	}
	if disposition := recorder.Header().Get("Content-Disposition"); !strings.Contains(disposition, "usage-log-summary") || !strings.Contains(disposition, ".xls") {
		t.Fatalf("expected summary filename in disposition, got %q", disposition)
	}

	body := recorder.Body.String()
	if !strings.Contains(body, "<th>总调用次数</th>") {
		t.Fatalf("expected summary header in export body, got %q", body)
	}
	if !strings.Contains(body, "<th>令牌总额度(USD)</th>") || !strings.Contains(body, "<th>令牌剩余额度(USD)</th>") {
		t.Fatalf("expected token quota header in export body, got %q", body)
	}
	if strings.Contains(body, "<th>总花费金额(CNY)</th>") {
		t.Fatalf("summary export should not contain CNY column, got %q", body)
	}
	if !strings.Contains(body, "<td>alpha-key</td>") || !strings.Contains(body, "<td>无限</td><td>无限</td><td>无限</td><td>1</td><td>150</td><td>2000</td><td>0.004000</td>") {
		t.Fatalf("expected alpha token summary row, got %q", body)
	}
	if !strings.Contains(body, "<td>beta-key</td>") || !strings.Contains(body, "<td>无限</td><td>无限</td><td>无限</td><td>0</td><td>0</td><td>0</td><td>0.000000</td>") {
		t.Fatalf("expected zero-usage token row, got %q", body)
	}
}

func TestExportLogSummaryRestrictsRegularUserToOwnTokens(t *testing.T) {
	db := setupLogExportTestDB(t)
	tokenA := seedExportToken(t, db, 1, "alpha-key", "abcd1234efgh5678")
	seedExportToken(t, db, 1, "gamma-key", "qrst1234uvwx5678")
	tokenB := seedExportToken(t, db, 2, "beta-key", "ijkl1234mnop5678")

	seedExportLog(t, db, &model.Log{
		UserId:           1,
		CreatedAt:        100,
		Type:             model.LogTypeConsume,
		Username:         "alice",
		TokenName:        tokenA.Name,
		TokenId:          tokenA.Id,
		ModelName:        "gpt-4o",
		Quota:            2000,
		PromptTokens:     100,
		CompletionTokens: 50,
	})
	seedExportLog(t, db, &model.Log{
		UserId:           2,
		CreatedAt:        120,
		Type:             model.LogTypeConsume,
		Username:         "bob",
		TokenName:        tokenB.Name,
		TokenId:          tokenB.Id,
		ModelName:        "claude-3-7-sonnet",
		Quota:            5000,
		PromptTokens:     200,
		CompletionTokens: 80,
	})

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/log/export/summary?start_timestamp=50&end_timestamp=200", nil, 1)
	ctx.Set("role", common.RoleCommonUser)
	ExportLogSummary(ctx)

	body := recorder.Body.String()
	if !strings.Contains(body, "alpha-key") || !strings.Contains(body, "gamma-key") {
		t.Fatalf("expected regular user summary export to include own tokens, got %q", body)
	}
	if strings.Contains(body, "beta-key") || strings.Contains(body, "claude-3-7-sonnet") {
		t.Fatalf("regular user summary export should exclude other users' tokens, got %q", body)
	}
}

func TestExportLogDetailOnlySuccessRecordsForRegularUser(t *testing.T) {
	db := setupLogExportTestDB(t)
	tokenA := seedExportToken(t, db, 1, "alpha-key", "abcd1234efgh5678")
	tokenB := seedExportToken(t, db, 2, "beta-key", "ijkl1234mnop5678")

	seedExportLog(t, db, &model.Log{
		UserId:           1,
		CreatedAt:        100,
		Type:             model.LogTypeConsume,
		Username:         "alice",
		TokenName:        tokenA.Name,
		TokenId:          tokenA.Id,
		ModelName:        "gpt-4o",
		Quota:            3000,
		PromptTokens:     120,
		CompletionTokens: 30,
		Group:            "default",
		RequestId:        "req-alpha",
		Content:          "detail row",
	})
	seedExportLog(t, db, &model.Log{
		UserId:    1,
		CreatedAt: 105,
		Type:      model.LogTypeError,
		Username:  "alice",
		TokenName: tokenA.Name,
		TokenId:   tokenA.Id,
		ModelName: "gpt-4o",
		Group:     "default",
		RequestId: "req-alpha-error",
		Content:   "upstream error",
	})
	seedExportLog(t, db, &model.Log{
		UserId:    1,
		CreatedAt: 110,
		Type:      model.LogTypeTopup,
		Username:  "alice",
		TokenName: tokenA.Name,
		TokenId:   tokenA.Id,
	})
	seedExportLog(t, db, &model.Log{
		UserId:           2,
		CreatedAt:        120,
		Type:             model.LogTypeConsume,
		Username:         "bob",
		TokenName:        tokenB.Name,
		TokenId:          tokenB.Id,
		ModelName:        "claude-3-7-sonnet",
		Quota:            4000,
		PromptTokens:     200,
		CompletionTokens: 60,
	})

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/log/export/detail?start_timestamp=50&end_timestamp=200&token_name=alpha-key", nil, 1)
	ctx.Set("role", common.RoleCommonUser)
	ExportLogDetail(ctx)

	if contentType := recorder.Header().Get("Content-Type"); !strings.Contains(contentType, "application/vnd.ms-excel") {
		t.Fatalf("expected excel content type, got %q", contentType)
	}

	body := recorder.Body.String()
	if strings.Contains(body, "<th>状态</th>") {
		t.Fatalf("detail export should not contain status column, got %q", body)
	}
	if strings.Contains(body, "<th>花费金额(CNY)</th>") {
		t.Fatalf("detail export should not contain CNY column, got %q", body)
	}
	if !strings.Contains(body, "<th>Request ID</th>") || !strings.Contains(body, "<th>缓存创建Tokens</th>") || !strings.Contains(body, "<th>缓存读取Tokens</th>") {
		t.Fatalf("expected new detail headers, got %q", body)
	}
	if !strings.Contains(body, "<td>alpha-key</td>") || !strings.Contains(body, "<td>"+fmt.Sprintf("%d", tokenA.Id)+"</td>") {
		t.Fatalf("expected alpha success detail row, got %q", body)
	}
	if !strings.Contains(body, "<td>req-alpha</td>") {
		t.Fatalf("expected request id in detail export, got %q", body)
	}
	if strings.Contains(body, "req-alpha-error") || strings.Contains(body, "upstream error") {
		t.Fatalf("detail export should exclude error rows, got %q", body)
	}
	if strings.Contains(body, "beta-key") || strings.Contains(body, "claude-3-7-sonnet") {
		t.Fatalf("detail export should exclude other users' rows, got %q", body)
	}
}

// callLogHandler runs a log handler as the given user and returns the JSON
// lines of the response.
func callLogHandler(t *testing.T, handler gin.HandlerFunc, userId int, role int, query string) []map[string]any {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/log?"+query, nil)
	ctx.Set("id", userId)
	ctx.Set("role", role)
	handler(ctx)
	var lines []map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(recorder.Body.String()), "\n") {
		var value map[string]any
		require.NoError(t, common.UnmarshalJsonStr(line, &value))
		lines = append(lines, value)
	}
	return lines
}

func exportedRequestIds(lines []map[string]any) []any {
	ids := make([]any, 0, len(lines))
	for _, line := range lines[:len(lines)-1] {
		ids = append(ids, line["request_id"])
	}
	return ids
}

// seedExportLogs stores logs of two users: three in the last hour and one
// from 400 days ago.
func seedExportLogs(t *testing.T, mainDB *gorm.DB, logDB *gorm.DB, now int64) {
	t.Helper()
	require.NoError(t, mainDB.Create(&model.Channel{Id: 9, Name: "primary"}).Error)
	logs := []model.Log{
		{UserId: 1, Username: "alice", CreatedAt: now - 3600, Type: model.LogTypeConsume, ModelName: "gpt-a", ChannelId: 9, Quota: 10, RequestId: "a-1", Other: `{"admin_info":{"use_channel":["9"]}}`},
		{UserId: 2, Username: "bob", CreatedAt: now - 1800, Type: model.LogTypeConsume, ModelName: "gpt-b", ChannelId: 9, Quota: 20, RequestId: "b-1"},
		{UserId: 1, Username: "alice", CreatedAt: now - 60, Type: model.LogTypeError, ModelName: "gpt-a", RequestId: "a-2"},
		{UserId: 1, Username: "alice", CreatedAt: now - 400*24*3600, Type: model.LogTypeConsume, ModelName: "gpt-a", RequestId: "a-old"},
	}
	for i := range logs {
		require.NoError(t, logDB.Create(&logs[i]).Error)
	}
}

func useLogExportDatabases(t *testing.T, mainDB *gorm.DB, logDB *gorm.DB, mainType common.DatabaseType, logType common.DatabaseType) {
	t.Helper()
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousMain, previousLog := common.MainDatabaseType(), common.LogDatabaseType()
	previousRedis, previousMemory := common.RedisEnabled, common.MemoryCacheEnabled
	model.DB, model.LOG_DB = mainDB, logDB
	common.SetDatabaseTypes(mainType, logType)
	common.RedisEnabled, common.MemoryCacheEnabled = false, false
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMain, previousLog)
		common.RedisEnabled, common.MemoryCacheEnabled = previousRedis, previousMemory
	})
}

func TestLogExportDatabaseMatrix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousMaster := common.IsMasterNode
	common.IsMasterNode = true
	t.Cleanup(func() { common.IsMasterNode = previousMaster })
	for _, tc := range []struct {
		kind, env string
		typ       common.DatabaseType
	}{
		{"sqlite", "", common.DatabaseTypeSQLite},
		{"mysql", "AUDIT_MYSQL_DSN", common.DatabaseTypeMySQL},
		{"postgres", "AUDIT_POSTGRES_DSN", common.DatabaseTypePostgreSQL},
		{"clickhouse", "AUDIT_CLICKHOUSE_DSN", common.DatabaseTypeClickHouse},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			dsn := os.Getenv(tc.env)
			if tc.env != "" && dsn == "" {
				t.Skip(tc.env + " is not configured")
			}
			db, isolatedDSN := newAuditTestDatabase(t, tc.kind, dsn)
			mainDB, mainType := db, tc.typ
			if tc.kind == "clickhouse" {
				mainDB, _ = newAuditTestDatabase(t, "sqlite", "")
				mainType = common.DatabaseTypeSQLite
			}
			useLogExportDatabases(t, mainDB, db, mainType, tc.typ)
			require.NoError(t, mainDB.AutoMigrate(&model.Channel{}))
			if tc.kind == "clickhouse" {
				t.Setenv("LOG_SQL_DSN", isolatedDSN)
				require.NoError(t, model.InitLogDB())
			} else {
				require.NoError(t, db.AutoMigrate(&model.Log{}))
			}
			versionSQL := "SELECT version()"
			if tc.kind == "sqlite" {
				versionSQL = "SELECT sqlite_version()"
			}
			var version string
			require.NoError(t, model.LOG_DB.Raw(versionSQL).Scan(&version).Error)
			t.Logf("log database: %s %s", tc.kind, version)
			now := time.Now().Unix()
			seedExportLogs(t, mainDB, model.LOG_DB, now)
			query := fmt.Sprintf("start_timestamp=%d&end_timestamp=%d", now-2*24*3600, now+3600)

			listed := callLogHandler(t, GetUserLogs, 1, common.RoleCommonUser, query)[0]
			require.Equal(t, true, listed["success"])
			assert.EqualValues(t, 2, listed["data"].(map[string]any)["total"])
			listed = callLogHandler(t, GetAllLogs, 3, common.RoleAdminUser, query+"&username=alice")[0]
			require.Equal(t, true, listed["success"])
			assert.EqualValues(t, 2, listed["data"].(map[string]any)["total"])

			own := callLogHandler(t, ExportUserLogs, 1, common.RoleCommonUser, query)
			require.Len(t, own, 3)
			assert.Equal(t, []any{"a-2", "a-1"}, exportedRequestIds(own))
			assert.Equal(t, map[string]any{"success": true, "message": "", "data": map[string]any{"count": float64(2)}}, own[2])
			assert.Equal(t, float64(1), own[0]["id"], "user exports carry list positions, not row ids")
			assert.Empty(t, own[1]["channel_name"])
			assert.NotContains(t, own[1]["other"], "admin_info")

			all := callLogHandler(t, ExportAllLogs, 3, common.RoleAdminUser, query+"&model_name=gpt-%25")
			require.Len(t, all, 4)
			assert.Equal(t, []any{"a-2", "b-1", "a-1"}, exportedRequestIds(all))
			assert.Equal(t, "primary", all[2]["channel_name"])
			assert.Contains(t, all[2]["other"], "admin_info")
			assert.Equal(t, true, all[3]["success"])
		})
	}
}

func TestLogExportRules(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, _ := newAuditTestDatabase(t, "sqlite", "")
	useLogExportDatabases(t, db, db, common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Log{}, &model.QuotaData{}))
	now := time.Now().Unix()
	seedExportLogs(t, db, db, now)
	logSetting := operation_setting.GetLogSetting()
	previousSetting := *logSetting
	t.Cleanup(func() { *logSetting = previousSetting })
	recent := fmt.Sprintf("start_timestamp=%d&end_timestamp=%d", now-2*24*3600, now)
	tooLong := fmt.Sprintf("start_timestamp=%d&end_timestamp=%d", now-366*24*3600, now)

	t.Run("time range", func(t *testing.T) {
		rejected := callLogHandler(t, ExportAllLogs, 3, common.RoleAdminUser, tooLong)
		require.Len(t, rejected, 1)
		assert.Equal(t, false, rejected[0]["success"])
		assert.Equal(t, "The time range cannot exceed {{days}} days", rejected[0]["message_key"])
		assert.Equal(t, map[string]any{"days": float64(365)}, rejected[0]["message_params"])

		assert.Equal(t, false, callLogHandler(t, GetUserLogs, 1, common.RoleCommonUser, tooLong)[0]["success"])
		assert.Equal(t, false, callLogHandler(t, GetLogsSelfStat, 1, common.RoleCommonUser, tooLong)[0]["success"])
		assert.Equal(t, true, callLogHandler(t, GetUserLogs, 3, common.RoleAdminUser, tooLong)[0]["success"], "administrators search their own logs without a limit")

		// The dashboard follows the same range instead of its old one-month limit.
		fortyDays := fmt.Sprintf("start_timestamp=%d&end_timestamp=%d", now-40*24*3600, now)
		for _, handler := range []gin.HandlerFunc{GetUserQuotaDates, GetUserFlowQuotaDates} {
			assert.Equal(t, "The time range cannot exceed {{days}} days", callLogHandler(t, handler, 1, common.RoleCommonUser, tooLong)[0]["message_key"])
			assert.Equal(t, true, callLogHandler(t, handler, 1, common.RoleCommonUser, fortyDays)[0]["success"])
		}

		// Without a start, a user's search reaches back the whole range.
		listed := callLogHandler(t, GetUserLogs, 1, common.RoleCommonUser, "")[0]
		assert.Equal(t, true, listed["success"])
		assert.EqualValues(t, 2, listed["data"].(map[string]any)["total"])

		logSetting.MaxRangeDays = 400
		assert.Equal(t, true, callLogHandler(t, GetUserLogs, 1, common.RoleCommonUser, tooLong)[0]["success"])
		logSetting.MaxRangeDays = previousSetting.MaxRangeDays
	})

	t.Run("user export switch", func(t *testing.T) {
		logSetting.UserExportEnabled = false
		t.Cleanup(func() { logSetting.UserExportEnabled = previousSetting.UserExportEnabled })
		disabled := callLogHandler(t, ExportUserLogs, 1, common.RoleCommonUser, recent)
		require.Len(t, disabled, 1)
		assert.Equal(t, "Log export is disabled", disabled[0]["message_key"])
		assert.Equal(t, true, callLogHandler(t, ExportUserLogs, 3, common.RoleAdminUser, recent)[0]["success"], "administrators export their own logs even when users cannot")
	})

	t.Run("one export per user", func(t *testing.T) {
		release, ok, err := service.AcquireLogExportLock(1)
		require.NoError(t, err)
		require.True(t, ok)
		busy := callLogHandler(t, ExportUserLogs, 1, common.RoleCommonUser, recent)
		require.Len(t, busy, 1)
		assert.Equal(t, "Another log export is running. Try again after it finishes.", busy[0]["message_key"])
		other := callLogHandler(t, ExportUserLogs, 2, common.RoleCommonUser, recent)
		assert.Equal(t, true, other[len(other)-1]["success"], "other users are not blocked")
		release()
		assert.Len(t, callLogHandler(t, ExportUserLogs, 1, common.RoleCommonUser, recent), 3)
	})
}

func TestAcquireLogExportLock(t *testing.T) {
	for _, useRedis := range []bool{false, true} {
		t.Run(fmt.Sprintf("redis=%v", useRedis), func(t *testing.T) {
			previousRedis, previousClient := common.RedisEnabled, common.RDB
			common.RedisEnabled = useRedis
			if useRedis {
				server := miniredis.RunT(t)
				common.RDB = redis.NewClient(&redis.Options{Addr: server.Addr()})
				require.NoError(t, common.RDB.Ping(context.Background()).Err())
			}
			t.Cleanup(func() { common.RedisEnabled, common.RDB = previousRedis, previousClient })

			release, ok, err := service.AcquireLogExportLock(1)
			require.NoError(t, err)
			require.True(t, ok)
			_, ok, err = service.AcquireLogExportLock(1)
			require.NoError(t, err)
			assert.False(t, ok, "a second export of the same user must wait")
			releaseOther, ok, err := service.AcquireLogExportLock(2)
			require.NoError(t, err)
			assert.True(t, ok)
			releaseOther()
			release()
			releaseAgain, ok, err := service.AcquireLogExportLock(1)
			require.NoError(t, err)
			assert.True(t, ok, "a finished export frees the user")
			releaseAgain()
		})
	}
}
