package router

import (
	"bytes"
	"io"
	"maps"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const claudeCodeGateBody = `{"model":"claude-test","metadata":{"user_id":"{\"device_id\":\"device\",\"session_id\":\"session\"}"},"system":[{"type":"text","text":"x-anthropic-billing-header: cc_version=2.1.288;"}],"messages":[{"role":"user","content":"hi"}],"max_tokens":16}`

func setupClaudeCodeGateTest(t *testing.T) {
	t.Helper()
	setupRelayRouterTestDB(t)
	require.NoError(t, i18n.Init())
	require.NoError(t, model.DB.AutoMigrate(&model.Option{}))
	previous := maps.Clone(model.CurrentRequestPolicy().Options)
	common.OptionMapRWMutex.Lock()
	options := maps.Clone(common.OptionMap)
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	common.OptionMapRWMutex.Unlock()
	oldRatios := ratio_setting.GroupRatio2JSONString()
	oldUsable := setting.UserUsableGroups2JSONString()
	oldAuto := setting.AutoGroups2JsonString()
	oldErrorLogEnabled := constant.ErrorLogEnabled
	constant.ErrorLogEnabled = false
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"cc-only":1,"cc-pro":1}`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","cc-only":"CC","cc-pro":"CC Pro","auto":"Auto"}`))
	t.Cleanup(func() {
		constant.ErrorLogEnabled = oldErrorLogEnabled
		require.NoError(t, model.UpdateRequestPolicyOptions(previous))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(oldRatios))
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(oldUsable))
		require.NoError(t, setting.UpdateAutoGroupsByJsonString(oldAuto))
		common.OptionMapRWMutex.Lock()
		common.OptionMap = options
		common.OptionMapRWMutex.Unlock()
	})
	user := model.User{Username: "cc-gate-user", Status: common.UserStatusEnabled, Group: "cc-only", Quota: 100}
	require.NoError(t, model.DB.Create(&user).Error)
	for key, group := range map[string]string{"ccinherit": "", "ccexplicit": "cc-pro", "ccplain": "default"} {
		require.NoError(t, model.DB.Create(&model.Token{
			UserId: user.Id, Key: key, Group: group, Status: common.TokenStatusEnabled,
			ExpiredTime: -1, UnlimitedQuota: true,
		}).Error)
	}
	setClaudeCodeGateTestPolicy(t, "enforce", "enforce")
}

func TestClaudeCodeGateRejectedRequestLogs(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			dsn := ""
			if dialect != "sqlite" {
				dsn = os.Getenv("TEST_" + strings.ToUpper(dialect) + "_DSN")
				if dsn == "" {
					t.Skip("test database DSN not configured")
				}
			}
			setupClaudeCodeGateTest(t)
			t.Setenv("LOG_SQL_DSN", dsn)
			require.NoError(t, model.InitLogDB())
			logDB := model.LOG_DB
			t.Cleanup(func() {
				if logDB != model.DB {
					sqlDB, err := logDB.DB()
					require.NoError(t, err)
					require.NoError(t, sqlDB.Close())
				}
				model.LOG_DB = model.DB
			})
			require.NoError(t, logDB.AutoMigrate(&model.Log{}))
			var version string
			versionQuery := "SELECT version()"
			if dialect == "sqlite" {
				versionQuery = "SELECT sqlite_version()"
			}
			require.NoError(t, logDB.Raw(versionQuery).Scan(&version).Error)
			t.Logf("log database version: %s", version)
			var token model.Token
			require.NoError(t, model.DB.Where(&model.Token{Key: "ccinherit"}).First(&token).Error)
			require.NoError(t, model.DB.Model(&token).Update("name", "gate-test-token").Error)

			engine := gin.New()
			engine.Use(middleware.RequestId(), middleware.BodyStorageCleanup())
			api := engine.Group("/v1", middleware.TokenAuth(), middleware.ClaudeCodeOnly())
			for _, path := range []string{"/messages", "/models", "/chat/completions"} {
				api.Any(path, func(c *gin.Context) { c.Status(http.StatusNoContent) })
			}
			engine.POST("/pg/chat/completions", func(c *gin.Context) {
				c.Set("id", token.UserId)
				c.Set("username", "cc-gate-user")
				common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
				c.Next()
			}, middleware.ClaudeCodePlaygroundGuard(), func(c *gin.Context) { c.Status(http.StatusNoContent) })

			for _, test := range []struct {
				name, method, path, body, missingHeader, mode, bodyMode, key, message, group string
				loggingDisabled                                                              bool
				status                                                                       int
			}{
				{name: "protocol", path: "/v1/chat/completions", message: "Claude Code gate rejected the request because the protocol is not allowed", status: 403},
				{name: "header", missingHeader: "x-app", message: "Claude Code gate rejected the request because x-app must be cli or cli-bg", status: 403},
				{name: "session", missingHeader: "x-claude-code-session-id", message: "Claude Code gate rejected the request because x-claude-code-session-id is missing", status: 403},
				{name: "beta", missingHeader: "anthropic-beta", message: "Claude Code gate rejected the request because the Claude Code beta header is missing", status: 403},
				{name: "invalid metadata type", body: `{"model":"claude-test","metadata":[]}`, message: "Claude Code gate rejected the request because the request body could not be parsed", status: 403},
				{name: "metadata", body: `{"model":"claude-test"}`, message: "Claude Code gate rejected the request because Claude Code metadata is missing or invalid", status: 403},
				{name: "billing", body: strings.Replace(claudeCodeGateBody, "x-anthropic-billing-header:", "other:", 1), message: "Claude Code gate rejected the request because the billing marker is missing", status: 403},
				{name: "models without CLI headers", method: "GET", path: "/v1/models", missingHeader: "User-Agent", status: 204},
				{name: "playground chosen group", path: "/pg/chat/completions", body: `{"group":"cc-pro","model":"claude-test"}`, message: "Claude Code gate rejected the request because Playground is not allowed for this group", group: "cc-pro", status: 403},
				{name: "allowed", status: 204},
				{name: "mixed group GPT messages bypass", body: `{"model":"gpt-5"}`, missingHeader: "User-Agent", status: 204},
				{name: "mixed group GPT OpenAI bypass", path: "/v1/chat/completions", body: `{"model":"gpt-5"}`, missingHeader: "User-Agent", status: 204},
				{name: "unrestricted group", key: "ccplain", missingHeader: "x-app", status: 204},
				{name: "observe", mode: "shadow", missingHeader: "x-app", body: `{"model":"claude-test"}`, status: 204},
				{name: "body observe", bodyMode: "shadow", body: `{"model":"claude-test"}`, status: 204},
				{name: "off", mode: "off", missingHeader: "x-app", status: 204},
				{name: "logging disabled", loggingDisabled: true, missingHeader: "x-app", status: 403},
			} {
				t.Run(test.name, func(t *testing.T) {
					mode, bodyMode := test.mode, test.bodyMode
					if mode == "" {
						mode = "enforce"
					}
					if bodyMode == "" {
						bodyMode = "enforce"
					}
					setClaudeCodeGateTestPolicy(t, mode, bodyMode)
					constant.ErrorLogEnabled = !test.loggingDisabled
					method, path, key, body, group := test.method, test.path, test.key, test.body, test.group
					if method == "" {
						method = http.MethodPost
					}
					if path == "" {
						path = "/v1/messages"
					}
					if key == "" {
						key = "ccinherit"
					}
					if body == "" {
						body = strings.Replace(claudeCodeGateBody, `"hi"`, `"log-body-secret"`, 1)
					}
					if group == "" {
						group = "cc-only"
					}
					request := claudeCodeGateTestRequest(method, path+"?api_key=log-query-secret", key, body)
					request.Header.Set("x-claude-code-session-id", "log-session-secret")
					request.Header.Set(common.RequestIdKey, "client-supplied-request-id")
					request.Header.Del(test.missingHeader)
					recorder := httptest.NewRecorder()
					engine.ServeHTTP(recorder, request)
					require.Equal(t, test.status, recorder.Code, recorder.Body.String())
					requestID := recorder.Header().Get(common.RequestIdKey)
					require.NotEmpty(t, requestID)
					assert.NotEqual(t, "client-supplied-request-id", requestID)
					t.Cleanup(func() { require.NoError(t, logDB.Where("request_id = ?", requestID).Delete(&model.Log{}).Error) })
					filter := model.LogFilter{Type: model.LogTypeError, RequestId: requestID}
					logs, total, err := model.GetAllLogs(filter, 0, 10)
					require.NoError(t, err)
					if test.status != 403 || test.loggingDisabled {
						assert.Zero(t, total)
						assert.Empty(t, logs)
						return
					}
					require.Equal(t, int64(1), total)
					require.Len(t, logs, 1)
					entry := logs[0]
					assert.Equal(t, model.LogTypeError, entry.Type)
					assert.Equal(t, token.UserId, entry.UserId)
					assert.Equal(t, "cc-gate-user", entry.Username)
					assert.Equal(t, group, entry.Group)
					assert.Zero(t, entry.ChannelId)
					assert.Equal(t, "claude-test", entry.ModelName)
					assert.Empty(t, entry.Ip)
					if strings.HasPrefix(path, "/pg/") {
						assert.Zero(t, entry.TokenId)
					} else {
						assert.Equal(t, token.Id, entry.TokenId)
						assert.Equal(t, "gate-test-token", entry.TokenName)
					}
					other, err := common.StrToMap(entry.Other)
					require.NoError(t, err)
					assert.Equal(t, path, other["request_path"])
					assert.Equal(t, method, other["request_method"])
					assert.Equal(t, "permission_error", other["error_type"])
					assert.Equal(t, "claude_code_only", other["error_code"])
					assert.Equal(t, float64(403), other["status_code"])
					assert.Equal(t, map[string]any{"reject_reason": test.message}, other["admin_info"])
					assert.Equal(t, []any{map[string]any{"key": test.message}}, other["content_parts"])
					for _, secret := range []string{"ccinherit", "log-query-secret", "log-session-secret", "log-body-secret"} {
						assert.NotContains(t, entry.Content+entry.Other, secret)
					}
					userLogs, userTotal, err := model.GetUserLogs(token.UserId, filter, 0, 10)
					require.NoError(t, err)
					require.Equal(t, int64(1), userTotal)
					require.Len(t, userLogs, 1)
					assert.Equal(t, "status_code=403, Request ID="+requestID, userLogs[0].Content)
					assert.NotContains(t, userLogs[0].Other, "reject_reason")
					assert.NotContains(t, userLogs[0].Other, "content_parts")
					_, unrelatedTotal, err := model.GetUserLogs(token.UserId+1000, filter, 0, 10)
					require.NoError(t, err)
					assert.Zero(t, unrelatedTotal)
				})
			}
			t.Run("failed log insert still blocks forwarding", func(t *testing.T) {
				constant.ErrorLogEnabled = true
				setClaudeCodeGateTestPolicy(t, "enforce", "enforce")
				require.NoError(t, logDB.Callback().Create().Before("gorm:create").Register("cc_gate_log_failure", func(tx *gorm.DB) {
					_ = tx.AddError(gorm.ErrInvalidDB)
				}))
				t.Cleanup(func() { require.NoError(t, logDB.Callback().Create().Remove("cc_gate_log_failure")) })
				request := claudeCodeGateTestRequest(http.MethodPost, "/v1/messages", "ccinherit", claudeCodeGateBody)
				request.Header.Del("x-app")
				recorder := httptest.NewRecorder()
				engine.ServeHTTP(recorder, request)
				assert.Equal(t, http.StatusForbidden, recorder.Code)
				assert.Contains(t, recorder.Body.String(), `"permission_error"`)
				logs, total, err := model.GetAllLogs(model.LogFilter{RequestId: recorder.Header().Get(common.RequestIdKey)}, 0, 10)
				require.NoError(t, err)
				assert.Zero(t, total)
				assert.Empty(t, logs)
			})
		})
	}
}

func setClaudeCodeGateTestPolicy(t *testing.T, mode, bodyMode string) {
	t.Helper()
	require.NoError(t, model.UpdateRequestPolicyOptions(map[string]string{
		"claude_code_gate.mode": mode, "claude_code_gate.body_mode": bodyMode,
		"claude_code_gate.groups": `["cc-only","cc-pro"]`,
	}))
}

func claudeCodeGateTestRequest(method, path, key, body string) *http.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+key)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "claude-cli/2.1.288 (external, cli)")
	request.Header.Set("x-app", "cli")
	request.Header.Add("anthropic-beta", "other-feature-20260101")
	request.Header.Add("anthropic-beta", "claude-code-20250219")
	request.Header.Set("x-claude-code-session-id", "session")
	return request
}

func TestClaudeCodeGateRequestContract(t *testing.T) {
	setupClaudeCodeGateTest(t)
	var owner model.User
	require.NoError(t, model.DB.Where(&model.User{Username: "cc-gate-user"}).First(&owner).Error)
	for _, token := range []model.Token{
		{UserId: owner.Id, Key: "ccdisabled", Status: common.TokenStatusDisabled, ExpiredTime: -1, UnlimitedQuota: true},
		{UserId: owner.Id, Key: "ccexpired", Status: common.TokenStatusEnabled, ExpiredTime: 1, UnlimitedQuota: true},
		{UserId: owner.Id, Key: "ccip", Status: common.TokenStatusEnabled, ExpiredTime: -1, UnlimitedQuota: true, AllowIps: common.GetPointer("203.0.113.1")},
	} {
		require.NoError(t, model.DB.Create(&token).Error)
	}
	bannedUser := model.User{Username: "cc-gate-banned", Status: common.UserStatusDisabled, Group: "cc-only", AffCode: "ccban1"}
	require.NoError(t, model.DB.Create(&bannedUser).Error)
	require.NoError(t, model.DB.Create(&model.Token{UserId: bannedUser.Id, Key: "ccbanned", Status: common.TokenStatusEnabled, ExpiredTime: -1, UnlimitedQuota: true}).Error)
	engine := gin.New()
	engine.Use(middleware.BodyStorageCleanup(), middleware.TokenAuth(), middleware.ClaudeCodeOnly())
	for _, path := range []string{"/v1/messages", "/v1/models", "/v1/chat/completions"} {
		engine.Any(path, func(c *gin.Context) {
			data, err := io.ReadAll(c.Request.Body)
			require.NoError(t, err)
			c.Data(http.StatusOK, "application/json", data)
		})
	}
	tests := []struct {
		name, method, path, key, mode, bodyMode, body, missingHeader string
		headers                                                      map[string][]string
		authFailure                                                  bool
		status                                                       int
	}{
		{name: "inherited restricted group", status: 200},
		{name: "official background CLI is accepted", headers: map[string][]string{"x-app": {"cli-bg"}}, status: 200},
		{name: "background app whitespace and case are normalized", headers: map[string][]string{"x-app": {" CLI-BG "}}, status: 200},
		{name: "background CLI still requires token authentication", headers: map[string][]string{"x-app": {"cli-bg"}}, key: "invalid", status: 401},
		{name: "background CLI still requires UA", headers: map[string][]string{"x-app": {"cli-bg"}}, missingHeader: "User-Agent", status: 403},
		{name: "background CLI still requires beta", headers: map[string][]string{"x-app": {"cli-bg"}}, missingHeader: "anthropic-beta", status: 403},
		{name: "background CLI still requires session", headers: map[string][]string{"x-app": {"cli-bg"}}, missingHeader: "x-claude-code-session-id", status: 403},
		{name: "background CLI still requires metadata", headers: map[string][]string{"x-app": {"cli-bg"}}, body: strings.Replace(claudeCodeGateBody, "device_id", "missing_device_id", 1), status: 403},
		{name: "background CLI still requires billing marker", headers: map[string][]string{"x-app": {"cli-bg"}}, body: strings.Replace(claudeCodeGateBody, "x-anthropic-billing-header:", "other:", 1), status: 403},
		{name: "background CLI still requires Claude protocol", headers: map[string][]string{"x-app": {"cli-bg"}}, path: "/v1/chat/completions", status: 403},
		{name: "copied fingerprints cannot replace token authentication", key: "invalid", status: 401},
		{name: "copied fingerprints cannot enable a disabled token", key: "ccdisabled", status: 401},
		{name: "copied fingerprints cannot renew an expired token", key: "ccexpired", status: 401},
		{name: "copied fingerprints cannot bypass token IP restrictions", key: "ccip", status: 403, authFailure: true},
		{name: "copied fingerprints cannot enable a banned user", key: "ccbanned", status: 403, authFailure: true},
		{name: "second selected token group", key: "ccexplicit", status: 200},
		{name: "ordinary token group overrides restricted user group", key: "ccplain", missingHeader: "User-Agent", body: `{}`, status: 200},
		{name: "messages rejects missing UA", missingHeader: "User-Agent", status: 403},
		{name: "messages rejects missing app", missingHeader: "x-app", status: 403},
		{name: "messages rejects missing beta", missingHeader: "anthropic-beta", status: 403},
		{name: "messages rejects missing session", missingHeader: "x-claude-code-session-id", status: 403},
		{name: "UA cannot contain an embedded CLI name", headers: map[string][]string{"User-Agent": {"curl claude-cli/2.1.288"}}, status: 403},
		{name: "UA must include a complete version", headers: map[string][]string{"User-Agent": {"claude-cli/2.1"}}, status: 403},
		{name: "UA rejects an ambiguous version suffix", headers: map[string][]string{"User-Agent": {"claude-cli/2.1.288.invalid"}}, status: 403},
		{name: "app cannot name another client", headers: map[string][]string{"x-app": {"sdk"}}, status: 403},
		{name: "app cannot name an unknown CLI variant", headers: map[string][]string{"x-app": {"cli-other"}}, status: 403},
		{name: "session cannot be whitespace", headers: map[string][]string{"x-claude-code-session-id": {" \t "}}, status: 403},
		{name: "beta token requires a suffix", headers: map[string][]string{"anthropic-beta": {"claude-code-"}}, status: 403},
		{name: "beta token must start with the CLI prefix", headers: map[string][]string{"anthropic-beta": {"other-claude-code-20250219"}}, status: 403},
		{name: "comma separated beta values are accepted", headers: map[string][]string{"anthropic-beta": {"other-feature, claude-code-20250219, another-feature"}}, status: 200},
		{name: "header whitespace and case are normalized", headers: map[string][]string{"User-Agent": {" CLAUDE-CLI/2.1.288 (external) "}, "x-app": {" CLI "}}, status: 200},
		{name: "SDK query parameters preserve the allowed path", path: "/v1/messages?beta=true", status: 200},
		{name: "metadata alone is insufficient", body: strings.Replace(claudeCodeGateBody, "x-anthropic-billing-header:", "other:", 1), status: 403},
		{name: "billing marker alone is insufficient", body: strings.Replace(claudeCodeGateBody, "device_id", "missing_device_id", 1), status: 403},
		{name: "malformed metadata is forbidden", body: `{"model":"claude-test","metadata":{"user_id":"invalid"},"system":"x-anthropic-billing-header:"}`, status: 403},
		{name: "malformed JSON is left to protocol validation", body: `{`, status: 200},
		{name: "top level JSON array is left to protocol validation", body: `[]`, status: 200},
		{name: "metadata null is forbidden", body: strings.Replace(claudeCodeGateBody, `"metadata":{`, `"metadata":null,"unused":{`, 1), status: 403},
		{name: "metadata user ID must be a JSON encoded string", body: `{"model":"claude-test","metadata":{"user_id":{"device_id":"d","session_id":"s"}},"system":"x-anthropic-billing-header:"}`, status: 403},
		{name: "empty identity values are forbidden", body: strings.Replace(claudeCodeGateBody, `\"device\"`, `\" \"`, 1), status: 403},
		{name: "marker in message text is insufficient", body: strings.Replace(strings.Replace(claudeCodeGateBody, "x-anthropic-billing-header:", "other:", 1), `"hi"`, `"x-anthropic-billing-header:"`, 1), status: 403},
		{name: "only system text blocks count", body: strings.Replace(claudeCodeGateBody, `"type":"text"`, `"type":"image"`, 1), status: 403},
		{name: "copied fingerprints do not validate message schema", body: strings.Replace(claudeCodeGateBody, `[{"role":"user","content":"hi"}]`, `"not a Claude messages array"`, 1), status: 200},
		{name: "plain system text with metadata passes", body: strings.Replace(claudeCodeGateBody, `[{"type":"text","text":"x-anthropic-billing-header: cc_version=2.1.288;"}]`, `"x-anthropic-billing-header: cc_version=2.1.288;"`, 1), status: 200},
		{name: "headers do not validate the body schema", bodyMode: "off", body: `{"model":"claude-test","messages":[]}`, status: 200},
		{name: "body observation preserves request bytes", bodyMode: "shadow", body: `{"model":"claude-test","messages":[],"unknown":false}`, status: 200},
		{name: "global observation caps body enforcement", mode: "shadow", missingHeader: "x-app", body: `{"model":"claude-test"}`, status: 200},
		{name: "global off bypasses all checks", mode: "off", body: `{}`, missingHeader: "User-Agent", status: 200},
		{name: "models only needs UA", method: "GET", path: "/v1/models", body: `{}`, missingHeader: "x-app", status: 200},
		{name: "models accepts ordinary clients", method: "GET", path: "/v1/models", missingHeader: "User-Agent", status: 200},
		{name: "method is part of allowlist", method: "PUT", status: 403},
		{name: "GPT bypasses header and body checks on messages", body: `{"model":"gpt-5","messages":[]}`, missingHeader: "User-Agent", status: 200},
		{name: "GPT uses OpenAI protocol in the selected group", path: "/v1/chat/completions", body: `{"model":"gpt-5","messages":[]}`, missingHeader: "User-Agent", status: 200},
		{name: "Gemini bypasses the gate", body: `{"model":"gemini-2.5-pro"}`, missingHeader: "User-Agent", status: 200},
		{name: "model modifiers do not change family", body: `{"model":"gpt-5@tag:claude-test"}`, missingHeader: "User-Agent", status: 200},
		{name: "OpenRouter Claude is gated", body: `{"model":"anthropic/claude-sonnet-4-6"}`, missingHeader: "User-Agent", status: 403},
		{name: "Bedrock Claude is gated", body: `{"model":"us.anthropic.claude-sonnet-4-6-v1:0"}`, missingHeader: "User-Agent", status: 403},
		{name: "case normalized Claude is gated", body: `{"model":"CLAUDE-SONNET-4-6"}`, missingHeader: "User-Agent", status: 403},
		{name: "Claude modifiers remain gated", body: `{"model":"claude-sonnet-4-6@thinking:on"}`, missingHeader: "User-Agent", status: 403},
		{name: "OpenAI protocol is forbidden", path: "/v1/chat/completions", status: 403},
		{name: "observation preserves non-Claude protocol", mode: "shadow", path: "/v1/chat/completions", status: 200},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			method, path, key, mode, bodyMode, body := test.method, test.path, test.key, test.mode, test.bodyMode, test.body
			if method == "" {
				method = "POST"
			}
			if path == "" {
				path = "/v1/messages"
			}
			if key == "" {
				key = "ccinherit"
			}
			if mode == "" {
				mode = "enforce"
			}
			if bodyMode == "" {
				bodyMode = "enforce"
			}
			if body == "" {
				body = claudeCodeGateBody
			}
			setClaudeCodeGateTestPolicy(t, mode, bodyMode)
			request := claudeCodeGateTestRequest(method, path, key, body)
			for header, values := range test.headers {
				request.Header.Del(header)
				for _, value := range values {
					request.Header.Add(header, value)
				}
			}
			if test.missingHeader != "" {
				request.Header.Del(test.missingHeader)
			}
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, request)
			require.Equal(t, test.status, recorder.Code, recorder.Body.String())
			if recorder.Code == 200 {
				assert.Equal(t, body, recorder.Body.String(), "fingerprint checks preserve the original body")
			} else if path == "/v1/messages" && test.status == http.StatusForbidden && !test.authFailure {
				assert.Contains(t, recorder.Body.String(), `"permission_error"`)
			}
		})
	}
	t.Run("clearing selected groups takes effect immediately", func(t *testing.T) {
		setClaudeCodeGateTestPolicy(t, "enforce", "enforce")
		require.NoError(t, model.UpdateRequestPolicyOptions(map[string]string{"claude_code_gate.groups": `[]`}))
		recorder := httptest.NewRecorder()
		request := claudeCodeGateTestRequest("POST", "/v1/messages", "ccinherit", `{}`)
		request.Header.Del("User-Agent")
		engine.ServeHTTP(recorder, request)
		assert.Equal(t, 200, recorder.Code)
	})
}

func TestClaudeCodeGateCoversRegisteredRelayEntrypoints(t *testing.T) {
	setupClaudeCodeGateTest(t)
	engine := gin.New()
	SetRelayRouter(engine)
	SetTaskPluginProtocolRouter(engine)
	SetVideoRouter(engine)
	SetTaskRouter(engine)
	for _, path := range []string{
		"/v1/chat/completions", "/v1/completions", "/v1/responses", "/v1/responses/compact",
		"/v1/embeddings", "/v1/images/generations", "/v1/images/edits", "/v1/audio/speech",
		"/v1beta/models/claude-test:generateContent", "/v1/models/claude-test:generateContent",
		"/v1/engines/claude-test/embeddings",
	} {
		t.Run(path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := claudeCodeGateTestRequest("POST", path, "ccinherit", `{"model":"claude-test"}`)
			request.Header.Del("User-Agent")
			engine.ServeHTTP(recorder, request)
			assert.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
			assert.Contains(t, recorder.Body.String(), "Claude")
		})
	}
	for _, body := range []string{`{`, `{"model":42}`, `{"model":"gpt-5","model":"claude-test"}`, `{"model":"claude-test","model":"gpt-5"}`} {
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, claudeCodeGateTestRequest("POST", "/v1/chat/completions", "ccinherit", body))
		assert.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}
	for _, path := range []string{"/v1/models", "/v1beta/models", "/v1beta/openai/models"} {
		recorder := httptest.NewRecorder()
		request := claudeCodeGateTestRequest("GET", path, "ccinherit", "")
		request.Header.Del("User-Agent")
		engine.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	}
	for _, modelName := range []string{"gpt-4o-realtime-preview", "claude-test"} {
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, claudeCodeGateTestRequest("GET", "/v1/realtime?model="+modelName, "ccinherit", ""))
		if modelName == "claude-test" {
			assert.Equal(t, http.StatusForbidden, recorder.Code)
		} else {
			assert.NotContains(t, recorder.Body.String(), "Claude Code")
		}
	}
}

func TestClaudeCodeGatePlaygroundUsesSelectedGroup(t *testing.T) {
	setupClaudeCodeGateTest(t)
	engine := gin.New()
	engine.Use(middleware.BodyStorageCleanup())
	engine.POST("/pg/chat/completions", func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyUsingGroup, c.GetHeader("test-user-group"))
		c.Next()
	}, middleware.ClaudeCodePlaygroundGuard(), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	for _, test := range []struct {
		userGroup, chosenGroup, mode, modelName string
		status                                  int
	}{
		{"default", "cc-only", "enforce", "claude-test", 403},
		{"cc-only", "", "enforce", "claude-test", 403},
		{"cc-only", "default", "enforce", "claude-test", 204},
		{"default", "cc-pro", "shadow", "claude-test", 204},
		{"default", "cc-pro", "off", "claude-test", 204},
		{"default", "cc-only", "enforce", "gpt-5", 204},
		{"cc-only", "", "enforce", "gemini-2.5-pro", 204},
	} {
		setClaudeCodeGateTestPolicy(t, test.mode, "off")
		data, err := common.Marshal(map[string]string{"group": test.chosenGroup, "model": test.modelName})
		require.NoError(t, err)
		request := httptest.NewRequest("POST", "/pg/chat/completions", strings.NewReader(string(data)))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("test-user-group", test.userGroup)
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, request)
		assert.Equal(t, test.status, recorder.Code)
	}
}

func TestClaudeCodeGateAutoGroupsRespectModeAndTokenSnapshot(t *testing.T) {
	setupClaudeCodeGateTest(t)
	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`["cc-only","default","cc-pro"]`))
	for _, mode := range []string{"off", "shadow", "enforce"} {
		setClaudeCodeGateTestPolicy(t, mode, "off")
		for _, custom := range []bool{false, true} {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("POST", "/v1/messages", nil)
			if custom {
				common.SetContextKey(c, constant.ContextKeyTokenAutoGroups, []string{"cc-only", "default"})
			}
			for _, modelName := range []string{"gpt-5", "gemini-2.5-pro", ""} {
				groups := service.GetRequestAutoGroups(c, "default", modelName)
				assert.Contains(t, groups, "cc-only", "non-Claude requests and discovery keep selected groups")
			}
			groups := service.GetRequestAutoGroups(c, "default", "claude-test")
			if mode == "enforce" {
				assert.Equal(t, []string{"default"}, groups)
			} else {
				assert.Contains(t, groups, "cc-only")
			}
		}
	}
	setClaudeCodeGateTestPolicy(t, "enforce", "off")
	oldMemoryCache, oldRetries := common.MemoryCacheEnabled, common.RetryTimes
	common.MemoryCacheEnabled, common.RetryTimes = false, 0
	t.Cleanup(func() { common.MemoryCacheEnabled, common.RetryTimes = oldMemoryCache, oldRetries })
	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}))
	for _, group := range []string{"cc-only", "default", "cc-pro"} {
		channel := model.Channel{Type: constant.ChannelTypeAnthropic, Key: "test-upstream", Status: common.ChannelStatusEnabled, Name: group, Models: "claude-test,gpt-5", Group: group}
		require.NoError(t, model.DB.Create(&channel).Error)
		for _, modelName := range []string{"claude-test", "gpt-5"} {
			require.NoError(t, model.DB.Create(&model.Ability{Group: group, Model: modelName, ChannelId: channel.Id, Enabled: true, Priority: common.GetPointer(int64(0)), Weight: 100}).Error)
		}
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/messages", nil)
	common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(c, constant.ContextKeyTokenCrossGroupRetry, true)
	param := &service.RetryParam{Ctx: c, TokenGroup: "auto", ModelName: "claude-test", RequestPath: "/v1/messages", Retry: common.GetPointer(0)}
	first, group, err := service.CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.NotNil(t, first)
	assert.Equal(t, "default", group)
	param.IncreaseRetry()
	next, _, err := service.CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	assert.Nil(t, next, "cross-group retry cannot enter either restricted group")
	c, _ = gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(c, constant.ContextKeyTokenCrossGroupRetry, true)
	param = &service.RetryParam{Ctx: c, TokenGroup: "auto", ModelName: "gpt-5", RequestPath: "/v1/chat/completions", Retry: common.GetPointer(0)}
	for _, expectedGroup := range []string{"cc-only", "default", "cc-pro"} {
		selected, group, err := service.CacheGetRandomSatisfiedChannel(param)
		require.NoError(t, err)
		require.NotNil(t, selected)
		assert.Equal(t, expectedGroup, group, "GPT retries can use every permitted group")
		param.IncreaseRetry()
	}
}

func TestClaudeCodeGateObservingMalformedDiskBodyPreservesBytes(t *testing.T) {
	setupClaudeCodeGateTest(t)
	setClaudeCodeGateTestPolicy(t, "shadow", "enforce")
	previousCache := common.GetDiskCacheConfig()
	common.SetDiskCacheConfig(common.DiskCacheConfig{Enabled: true, ThresholdMB: 0, MaxSizeMB: 16, Path: t.TempDir()})
	t.Cleanup(func() { common.SetDiskCacheConfig(previousCache) })
	engine := gin.New()
	engine.Use(middleware.BodyStorageCleanup(), middleware.TokenAuth(), middleware.ClaudeCodeOnly())
	engine.POST("/v1/messages", func(c *gin.Context) {
		data, err := io.ReadAll(c.Request.Body)
		require.NoError(t, err)
		c.String(http.StatusOK, string(data))
	})
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, claudeCodeGateTestRequest("POST", "/v1/messages", "ccinherit", `{`))
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, `{`, recorder.Body.String())
}

func TestClaudeCodeGateDynamicPluginResolvedModel(t *testing.T) {
	setupClaudeCodeGateTest(t)
	require.NoError(t, model.DB.AutoMigrate(&model.Task{}, &model.Channel{}))
	source := routerPluginSource("cc-gate", "1.0.0", `[
		{method: "GET", path: "/vendor/cc-gate/:task_id", type: "query", render: "native"},
		{method: "POST", path: "/vendor/cc-gate", type: "submit", decode: "decode", render: "native"}
	]`)
	source = strings.Replace(source, `models: ["model"]`, `models: ["gpt-5", "claude-test"]`, 1)
	source = strings.Replace(source, `model: "model", requestBody: ctx.body.value`, `model: ctx.body.value.vendor_model, requestBody: ctx.body.value`, 1)
	plugin, err := jsplugin.CompilePlugin(source, jsplugin.Options{Key: "cc-gate", Version: "1.0.0"})
	require.NoError(t, err)
	engine, registry := newPluginRouterTest(t, []*jsplugin.LoadedPlugin{plugin}, productionPluginRouteHandlers)
	engine.NoRoute((&pluginRouteDispatcher{registry: registry}).dispatch)
	for _, test := range []struct {
		method, path, body string
		status             int
	}{
		{"GET", "/vendor/cc-gate/task-1", "", 404},
		{"POST", "/vendor/cc-gate", `{"vendor_model":"claude-test"}`, 403},
		{"POST", "/vendor/cc-gate", `{"vendor_model":"gpt-5"}`, 503},
	} {
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, claudeCodeGateTestRequest(test.method, test.path, "ccinherit", test.body))
		assert.Equal(t, test.status, recorder.Code, recorder.Body.String())
		if test.status == 403 {
			assert.Contains(t, recorder.Body.String(), "Claude")
		} else {
			assert.NotContains(t, recorder.Body.String(), "Claude Code")
		}
	}
}

func TestClaudeCodeGateMixedGroupDistribution(t *testing.T) {
	setupClaudeCodeGateTest(t)
	previousCache := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() { common.MemoryCacheEnabled = previousCache })
	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}))
	channel := model.Channel{Name: "mixed-models", Type: constant.ChannelTypeOpenAI, Key: "upstream", Status: common.ChannelStatusEnabled, Models: "gpt-5,claude-test,whisper-1", Group: "cc-only,default"}
	require.NoError(t, model.DB.Create(&channel).Error)
	for _, group := range []string{"cc-only", "default"} {
		for _, modelName := range []string{"gpt-5", "claude-test", "whisper-1"} {
			require.NoError(t, model.DB.Create(&model.Ability{Group: group, Model: modelName, ChannelId: channel.Id, Enabled: true}).Error)
		}
	}
	engine := gin.New()
	engine.Use(middleware.BodyStorageCleanup())
	api := engine.Group("/v1", middleware.TokenAuth(), middleware.ClaudeCodeOnly(), middleware.Distribute())
	for _, path := range []string{"/messages", "/chat/completions", "/responses", "/audio/transcriptions"} {
		api.POST(path, func(c *gin.Context) {
			assert.Equal(t, channel.Id, c.GetInt("channel_id"))
			c.Status(http.StatusNoContent)
		})
	}
	engine.POST("/pg/chat/completions", func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "cc-only")
		c.Next()
	}, middleware.ClaudeCodePlaygroundGuard(), middleware.Distribute(), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	for _, test := range []struct {
		path, key, body string
		status          int
	}{
		{"/v1/chat/completions", "ccinherit", `{"model":"gpt-5"}`, 204},
		{"/v1/responses", "ccinherit", `{"model":"gpt-5"}`, 204},
		{"/v1/messages", "ccinherit", `{"model":"gpt-5"}`, 204},
		{"/v1/messages", "ccinherit", claudeCodeGateBody, 403},
		{"/v1/chat/completions", "ccplain", `{"model":"claude-test"}`, 204},
		{"/pg/chat/completions", "", `{"model":"claude-test","group":"default"}`, 204},
		{"/pg/chat/completions", "", `{"model":"gpt-5","group":"cc-only"}`, 204},
	} {
		request := claudeCodeGateTestRequest("POST", test.path, test.key, test.body)
		request.Header.Set("User-Agent", "ordinary-sdk")
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, request)
		assert.Equal(t, test.status, recorder.Code, recorder.Body.String())
	}
	var form bytes.Buffer
	writer := multipart.NewWriter(&form)
	require.NoError(t, writer.WriteField("model", "whisper-1"))
	file, err := writer.CreateFormFile("file", "sample.wav")
	require.NoError(t, err)
	_, err = file.Write([]byte("test audio"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	request := httptest.NewRequest("POST", "/v1/audio/transcriptions", &form)
	request.Header.Set("Authorization", "Bearer ccinherit")
	request.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	assert.Equal(t, http.StatusNoContent, recorder.Code, recorder.Body.String())
}

func TestClaudeCodeGateResponsesWebSocketUsesEventModel(t *testing.T) {
	setupClaudeCodeGateTest(t)
	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}))
	engine := gin.New()
	SetRelayRouter(engine)
	server := httptest.NewServer(engine)
	t.Cleanup(server.Close)
	for _, test := range []struct {
		name, event string
		status      int
	}{
		{"GPT", `{"type":"response.create","model":"gpt-5","input":"hi"}`, 503},
		{"Claude", `{"type":"response.create","model":"claude-test","input":"hi"}`, 403},
		{"wrapped Claude", `{"type":"response.create","model":"gpt-5","response":{"model":"claude-test","input":"hi"}}`, 403},
		{"wrapped GPT", `{"type":"response.create","model":"claude-test","response":{"model":"gpt-5","input":"hi"}}`, 503},
	} {
		t.Run(test.name, func(t *testing.T) {
			client, response, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/v1/responses", http.Header{"Authorization": {"Bearer ccinherit"}})
			require.NoError(t, err)
			assert.Equal(t, http.StatusSwitchingProtocols, response.StatusCode)
			defer client.Close()
			require.NoError(t, client.SetReadDeadline(time.Now().Add(5*time.Second)))
			require.NoError(t, client.WriteMessage(websocket.TextMessage, []byte(test.event)))
			var event struct {
				Type   string `json:"type"`
				Status int    `json:"status"`
				Error  struct {
					Type    string `json:"type"`
					Message string `json:"message"`
				} `json:"error"`
			}
			_, payload, err := client.ReadMessage()
			require.NoError(t, err)
			require.NoError(t, common.Unmarshal(payload, &event))
			assert.Equal(t, "error", event.Type)
			assert.Equal(t, test.status, event.Status)
			if test.status == 403 {
				assert.Equal(t, "new_api_error", event.Error.Type)
				assert.Contains(t, event.Error.Message, "Claude")
			} else {
				assert.NotContains(t, event.Error.Message, "Claude Code")
			}
		})
	}
}
