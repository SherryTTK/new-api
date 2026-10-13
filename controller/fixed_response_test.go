package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"gorm.io/gorm"
)

func TestFixedResponseRelayBilling(t *testing.T) {
	service.InitTokenEncoders()
	require.NoError(t, i18n.Init())
	previous, err := common.Marshal(ratio_setting.GetModelRatioCopy())
	require.NoError(t, err)
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"gpt-4o-mini":1}`))
	t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(string(previous))) })
	for _, tc := range []struct {
		name                                 string
		stream, cancel, denied, writeFailure bool
		quota                                int
		cacheRead, cacheWrite                int
		claude                               bool
	}{
		{name: "nonstream", quota: 100000}, {name: "stream", stream: true, quota: 100000},
		{name: "insufficient", quota: 1}, {name: "cancelled", cancel: true, quota: 100000},
		{name: "model permission", denied: true, quota: 100000},
		{name: "write failure", writeFailure: true, quota: 100000},
		{name: "cache read", cacheRead: 1000, quota: 100000},
		{name: "cache write", cacheWrite: 2000, quota: 100000},
		{name: "cache both stream", cacheRead: 1000, cacheWrite: 2000, stream: true, quota: 100000},
		{name: "claude cache read", claude: true, cacheRead: 1000, quota: 100000},
		{name: "claude cache both", claude: true, cacheRead: 1000, cacheWrite: 2000, quota: 100000},
		{name: "cache insufficient", cacheRead: 1000000, cacheWrite: 1000000, quota: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			oldDB, oldLogDB := model.DB, model.LOG_DB
			oldRedis, oldBatch, oldLog, oldCount := common.RedisEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled, constant.CountToken
			common.RedisEnabled = false
			common.BatchUpdateEnabled = false
			common.LogConsumeEnabled = true
			constant.CountToken = false
			t.Cleanup(func() {
				model.DB = oldDB
				model.LOG_DB = oldLogDB
				common.RedisEnabled = oldRedis
				common.BatchUpdateEnabled = oldBatch
				common.LogConsumeEnabled = oldLog
				constant.CountToken = oldCount
			})
			db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:fixed_%s?mode=memory&cache=shared", strings.ReplaceAll(tc.name, " ", "_"))), &gorm.Config{})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(1)
			t.Cleanup(func() { assert.NoError(t, sqlDB.Close()) })
			model.DB = db
			model.LOG_DB = db
			common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
			// No channel table: any accidental channel selection fails this fixture.
			require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}, &model.Log{}, &model.TokenLastRequest{}))
			user := model.User{Username: "fixed_user", Quota: tc.quota, Status: common.UserStatusEnabled, Group: "default", Setting: `{"billing_preference":"wallet_only"}`}
			require.NoError(t, db.Create(&user).Error)
			config := &types.FixedResponseConfig{Enabled: true, Content: strings.Repeat("固定回复 ", 20), CacheReadTokens: tc.cacheRead, CacheWriteTokens: tc.cacheWrite}
			if tc.cancel {
				config.MinDelayMS = 300000
				config.MaxDelayMS = 300000
			}
			token := model.Token{UserId: user.Id, Key: strings.Repeat("a", 48), Status: common.TokenStatusEnabled, ExpiredTime: -1, RemainQuota: 100000, FixedResponse: config, RecordLastRequest: common.GetPointer(true), ModelLimitsEnabled: tc.denied, ModelLimits: "other-model"}
			require.NoError(t, db.Create(&token).Error)
			updated := make(chan struct{}, 4)
			if tc.cancel || tc.writeFailure {
				require.NoError(t, db.Callback().Update().After("gorm:commit_or_rollback_transaction").Register("fixed_refund", func(tx *gorm.DB) {
					if tx.Statement.Table == "tokens" {
						updated <- struct{}{}
					}
				}))
			}
			router := gin.New()
			router.Use(middleware.BodyStorageCleanup(), middleware.TokenAuth(), middleware.Distribute())
			router.POST("/v1/chat/completions", func(c *gin.Context) { Relay(c, types.RelayFormatOpenAI) })
			router.POST("/v1/messages", func(c *gin.Context) { Relay(c, types.RelayFormatClaude) })
			body := fmt.Sprintf(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello"}],"stream":%t,"stream_options":{"include_usage":true}}`, tc.stream)
			req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
			if tc.claude {
				req = httptest.NewRequest("POST", "/v1/messages", strings.NewReader(`{"model":"gpt-4o-mini","max_tokens":1000,"messages":[{"role":"user","content":"hello"}]}`))
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer sk-"+token.Key)
			if tc.cancel {
				ctx, cancel := context.WithCancel(req.Context())
				cancel()
				req = req.WithContext(ctx)
			}
			w := httptest.NewRecorder()
			var writer http.ResponseWriter = w
			if tc.writeFailure {
				writer = failingFixedResponseWriter{w}
			}
			router.ServeHTTP(writer, req)
			if tc.cancel || tc.writeFailure {
				// Synchronize on the pre-consume and asynchronous refund database writes.
				for i := 0; i < 2; i++ {
					select {
					case <-updated:
					case <-time.After(3 * time.Second):
						t.Fatal("token refund did not complete")
					}
				}
			}
			var actualUser model.User
			require.NoError(t, db.First(&actualUser, user.Id).Error)
			var actualToken model.Token
			require.NoError(t, db.First(&actualToken, token.Id).Error)
			var logs []model.Log
			require.NoError(t, db.Find(&logs).Error)
			recorded, err := model.GetTokenLastRequest(token.Id, user.Id)
			require.NoError(t, err)
			if tc.denied {
				assert.Nil(t, recorded)
			} else {
				require.NotNil(t, recorded)
				assert.Equal(t, "hello", recorded.Question)
				switch {
				case tc.cancel:
					assert.Equal(t, "cancelled", recorded.Status)
				case tc.writeFailure, tc.quota == 1:
					assert.Equal(t, "failed", recorded.Status)
				default:
					assert.Equal(t, "completed", recorded.Status)
					assert.Equal(t, config.Content, recorded.Answer)
				}
			}
			if tc.cancel || tc.writeFailure || tc.denied || tc.quota == 1 {
				if !tc.cancel && !tc.writeFailure {
					assert.Equal(t, 403, w.Code, w.Body.String())
				}
				assert.Equal(t, tc.quota, actualUser.Quota)
				assert.Equal(t, 100000, actualToken.RemainQuota)
				assert.Empty(t, logs)
				return
			}
			require.Equal(t, 200, w.Code, w.Body.String())
			require.Len(t, logs, 1)
			log := logs[0]
			assert.Positive(t, log.PromptTokens, "local counting must work when global estimation is disabled")
			assert.Equal(t, service.CountTextToken(config.Content, "gpt-4o-mini"), log.CompletionTokens)
			ratio, ok, _ := ratio_setting.GetModelRatio("gpt-4o-mini")
			require.True(t, ok)
			cacheRatio, _ := ratio_setting.GetCacheRatio("gpt-4o-mini")
			writeRatio, _ := ratio_setting.GetCreateCacheRatio("gpt-4o-mini")
			expected := common.QuotaRound((float64(log.PromptTokens-tc.cacheRead-tc.cacheWrite) + float64(tc.cacheRead)*cacheRatio + float64(tc.cacheWrite)*writeRatio + float64(log.CompletionTokens)*ratio_setting.GetCompletionRatio("gpt-4o-mini")) * ratio * ratio_setting.GetGroupRatio("default"))
			require.Positive(t, expected)
			assert.Equal(t, expected, log.Quota)
			assert.Equal(t, tc.quota-expected, actualUser.Quota)
			assert.Equal(t, 100000-expected, actualToken.RemainQuota)
			assert.Equal(t, expected, actualToken.UsedQuota)
			assert.Equal(t, 0, log.ChannelId)
			assert.True(t, gjson.Get(log.Other, "fixed_response").Bool())
			assert.EqualValues(t, tc.cacheRead, gjson.Get(log.Other, "cache_tokens").Int())
			assert.EqualValues(t, tc.cacheWrite, gjson.Get(log.Other, "cache_write_tokens").Int())
			assert.EqualValues(t, tc.cacheWrite, gjson.Get(log.Other, "cache_creation_tokens").Int())
			if tc.claude {
				assert.EqualValues(t, tc.cacheRead, gjson.Get(w.Body.String(), "usage.cache_read_input_tokens").Int())
				assert.EqualValues(t, tc.cacheWrite, gjson.Get(w.Body.String(), "usage.cache_creation_input_tokens").Int())
				assert.EqualValues(t, log.PromptTokens-tc.cacheRead-tc.cacheWrite, gjson.Get(w.Body.String(), "usage.input_tokens").Int())
			} else if !tc.stream {
				assert.EqualValues(t, tc.cacheRead, gjson.Get(w.Body.String(), "usage.prompt_tokens_details.cached_tokens").Int())
				assert.EqualValues(t, tc.cacheWrite, gjson.Get(w.Body.String(), "usage.prompt_tokens_details.cache_write_tokens").Int())
				assert.Equal(t, config.Content, gjson.Get(w.Body.String(), "choices.0.message.content").String())
			}
		})
	}
}

func TestTokenFixedResponseConfiguration(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	token := model.Token{UserId: 42, Key: strings.Repeat("b", 48), Name: "original", ExpiredTime: -1, UnlimitedQuota: true}
	require.NoError(t, token.Insert())
	for _, tc := range []struct {
		name, config string
		enabled      bool
		success      bool
	}{
		{"enable", `,"fixed_response":{"enabled":true,"min_delay_ms":10,"max_delay_ms":20,"content":"hello","cache_read_tokens":1000,"cache_write_tokens":2000}`, true, true},
		{"legacy update", "", true, true},
		{"invalid", `,"fixed_response":{"enabled":true,"min_delay_ms":20,"max_delay_ms":10,"content":"hello"}`, true, false},
		{"negative cache", `,"fixed_response":{"enabled":true,"content":"hello","cache_read_tokens":-1}`, true, false},
		{"excessive cache", `,"fixed_response":{"enabled":true,"content":"hello","cache_write_tokens":1000001}`, true, false},
		{"fractional cache", `,"fixed_response":{"enabled":true,"content":"hello","cache_read_tokens":1.5}`, true, false},
		{"disable", `,"fixed_response":{"enabled":false,"min_delay_ms":0,"max_delay_ms":0,"content":"hello"}`, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Set("id", 42)
			c.Set("role", common.RoleRootUser)
			c.Request = httptest.NewRequest("PUT", "/api/token/", strings.NewReader(fmt.Sprintf(`{"id":%d,"name":"updated","unlimited_quota":true,"expired_time":-1%s}`, token.Id, tc.config)))
			c.Request.Header.Set("Content-Type", "application/json")
			UpdateToken(c)
			assert.Equal(t, tc.success, gjson.Get(w.Body.String(), "success").Bool(), w.Body.String())
			var saved model.Token
			require.NoError(t, db.First(&saved, token.Id).Error)
			require.NotNil(t, saved.FixedResponse)
			assert.Equal(t, tc.enabled, saved.FixedResponse.Enabled)
			assert.Equal(t, "hello", saved.FixedResponse.Content)
			if tc.enabled {
				assert.Equal(t, 1000, saved.FixedResponse.CacheReadTokens)
				assert.Equal(t, 2000, saved.FixedResponse.CacheWriteTokens)
			} else {
				assert.Zero(t, saved.FixedResponse.CacheReadTokens)
				assert.Zero(t, saved.FixedResponse.CacheWriteTokens)
			}
		})
	}
}

type failingFixedResponseWriter struct{ *httptest.ResponseRecorder }

func (w failingFixedResponseWriter) Write(_ []byte) (int, error) {
	return 0, errors.New("client disconnected")
}
