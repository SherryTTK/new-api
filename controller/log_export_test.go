package controller

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupLogExportTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
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
	if !strings.Contains(body, "<th>总花费金额(CNY)</th>") {
		t.Fatalf("expected summary cny header in export body, got %q", body)
	}
	if !strings.Contains(body, "<td>alpha-key</td>") || !strings.Contains(body, "<td>1</td><td>150</td><td>2000</td><td>0.004000</td><td>0.028800</td>") {
		t.Fatalf("expected alpha token summary row, got %q", body)
	}
	if !strings.Contains(body, "<td>beta-key</td>") || !strings.Contains(body, "<td>0</td><td>0</td><td>0</td><td>0.000000</td><td>0.000000</td>") {
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

func TestExportLogDetailIncludesSuccessAndErrorRowsForRegularUser(t *testing.T) {
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
	if !strings.Contains(body, "<th>状态</th>") || !strings.Contains(body, "<th>花费金额(CNY)</th>") || !strings.Contains(body, "<th>Request ID</th>") {
		t.Fatalf("expected detail header in export body, got %q", body)
	}
	if !strings.Contains(body, "<td>成功</td>") || !strings.Contains(body, "<td>消费</td>") || !strings.Contains(body, "<td>alpha-key</td>") || !strings.Contains(body, "<td>"+fmt.Sprintf("%d", tokenA.Id)+"</td>") {
		t.Fatalf("expected alpha success detail row, got %q", body)
	}
	if !strings.Contains(body, "<td>150</td><td>3000</td><td>0.006000</td><td>0.043200</td>") || !strings.Contains(body, "<td>req-alpha</td>") {
		t.Fatalf("unexpected detail export values: %q", body)
	}
	if !strings.Contains(body, "<td>失败</td>") || !strings.Contains(body, "<td>错误</td>") || !strings.Contains(body, "<td>req-alpha-error</td>") || !strings.Contains(body, "upstream error") {
		t.Fatalf("expected alpha error detail row, got %q", body)
	}
	if strings.Contains(body, "beta-key") || strings.Contains(body, "claude-3-7-sonnet") {
		t.Fatalf("detail export should exclude other users' rows, got %q", body)
	}
}
