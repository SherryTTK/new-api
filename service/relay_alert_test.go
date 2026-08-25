package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatchRelayAlertRulesUsesORAndUrgentWins(t *testing.T) {
	event := RelayAlertEvent{
		Category:  operation_setting.RelayAlertCategoryHTTP5xx,
		UserID:    7,
		TokenID:   9,
		ChannelID: 11,
	}
	rules := []operation_setting.RelayAlertRule{
		{
			ID:         "warning-user",
			Name:       "Warning user",
			Enabled:    true,
			TargetType: operation_setting.RelayAlertTargetUser,
			TargetIDs:  []int{7},
			Categories: []string{operation_setting.RelayAlertCategoryHTTP5xx},
			Level:      operation_setting.RelayAlertLevelWarning,
		},
		{
			ID:         "urgent-channel",
			Name:       "Urgent channel",
			Enabled:    true,
			TargetType: operation_setting.RelayAlertTargetChannel,
			TargetIDs:  []int{11},
			Categories: []string{operation_setting.RelayAlertCategoryHTTP5xx},
			Level:      operation_setting.RelayAlertLevelUrgent,
		},
		{
			ID:         "different-token",
			Name:       "Different token",
			Enabled:    true,
			TargetType: operation_setting.RelayAlertTargetToken,
			TargetIDs:  []int{99},
			Categories: []string{operation_setting.RelayAlertCategoryHTTP5xx},
			Level:      operation_setting.RelayAlertLevelUrgent,
		},
	}

	ids, names, level := matchRelayAlertRules(rules, event)

	assert.Equal(t, []string{"urgent-channel", "warning-user"}, ids)
	assert.Equal(t, []string{"Urgent channel", "Warning user"}, names)
	assert.Equal(t, operation_setting.RelayAlertLevelUrgent, level)
}

func TestClassifyRelayAlert(t *testing.T) {
	gin.SetMode(gin.TestMode)
	newContext := func() *gin.Context {
		context, _ := gin.CreateTestContext(httptest.NewRecorder())
		context.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		return context
	}

	t.Run("successful semantic output does not alert", func(t *testing.T) {
		context := newContext()
		info := &relaycommon.RelayInfo{}
		info.MarkValidOutput()
		category, _ := classifyRelayAlert(context, info, nil)
		assert.Empty(t, category)
	})

	t.Run("successful usage-only response is empty", func(t *testing.T) {
		context := newContext()
		category, _ := classifyRelayAlert(context, &relaycommon.RelayInfo{}, nil)
		assert.Equal(t, operation_setting.RelayAlertCategoryEmptyResponse, category)
	})

	t.Run("final upstream 429", func(t *testing.T) {
		context := newContext()
		info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 3}}
		apiErr := types.InitOpenAIError(types.ErrorCodeBadResponseStatusCode, http.StatusTooManyRequests)
		category, _ := classifyRelayAlert(context, info, apiErr)
		assert.Equal(t, operation_setting.RelayAlertCategoryHTTP429, category)
	})

	t.Run("status mapping preserves upstream 5xx classification", func(t *testing.T) {
		context := newContext()
		info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 3}}
		apiErr := types.InitOpenAIError(types.ErrorCodeBadResponseStatusCode, http.StatusBadGateway)
		ResetStatusCode(apiErr, `{"502":400}`)
		category, _ := classifyRelayAlert(context, info, apiErr)
		assert.Equal(t, operation_setting.RelayAlertCategoryHTTP5xx, category)
		assert.Equal(t, http.StatusBadGateway, apiErr.GetOriginalStatusCode())
		assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	})

	t.Run("client-gone stream is excluded", func(t *testing.T) {
		context := newContext()
		status := relaycommon.NewStreamStatus()
		status.SetEndReason(relaycommon.StreamEndReasonClientGone, assert.AnError)
		info := &relaycommon.RelayInfo{IsStream: true, StreamStatus: status}
		category, _ := classifyRelayAlert(context, info, nil)
		assert.Empty(t, category)
	})

	t.Run("partial abnormal stream alerts", func(t *testing.T) {
		context := newContext()
		status := relaycommon.NewStreamStatus()
		status.SetEndReason(relaycommon.StreamEndReasonTimeout, nil)
		info := &relaycommon.RelayInfo{IsStream: true, StreamStatus: status}
		info.MarkValidOutput()
		category, endReason := classifyRelayAlert(context, info, nil)
		assert.Equal(t, operation_setting.RelayAlertCategoryStreamError, category)
		assert.Equal(t, string(relaycommon.StreamEndReasonTimeout), endReason)
	})
}

func TestSendFeishuRelayAlertUrgentSignatureAndMention(t *testing.T) {
	var received map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.NoError(t, common.DecodeJson(request.Body, &received))
		body, err := common.Marshal(map[string]any{"code": 0, "msg": "success"})
		require.NoError(t, err)
		writer.Header().Set("Content-Type", "application/json")
		_, err = writer.Write(body)
		require.NoError(t, err)
	}))
	defer server.Close()

	relayAlertConfig := config.GlobalConfig.Get("relay_alert_setting")
	require.NotNil(t, relayAlertConfig)
	original, err := config.ConfigToMap(relayAlertConfig)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, config.UpdateConfigFromMap(relayAlertConfig, original))
	})
	require.NoError(t, config.UpdateConfigFromMap(relayAlertConfig, map[string]string{
		"feishu_webhook_url": server.URL,
		"feishu_secret":      "test-secret",
	}))

	event := RelayAlertEvent{
		RequestID:   "req-test",
		OccurredAt:  time.Unix(100, 0),
		NodeName:    "node-a",
		RequestPath: "/v1/chat/completions",
		Category:    operation_setting.RelayAlertCategoryHTTP5xx,
		Level:       operation_setting.RelayAlertLevelUrgent,
		Count:       1,
	}
	require.NoError(t, sendFeishuRelayAlert(t.Context(), event, false))

	assert.Equal(t, "interactive", received["msg_type"])
	timestamp, ok := received["timestamp"].(string)
	require.True(t, ok)
	mac := hmac.New(sha256.New, []byte(timestamp+"\n"+"test-secret"))
	expectedSignature := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	assert.Equal(t, expectedSignature, received["sign"])

	card, ok := received["card"].(map[string]any)
	require.True(t, ok)
	header, ok := card["header"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "red", header["template"])
	elements, ok := card["elements"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, elements)
	markdown, ok := elements[0].(map[string]any)
	require.True(t, ok)
	content, ok := markdown["content"].(string)
	require.True(t, ok)
	assert.True(t, strings.Contains(content, "<at id=all></at>"))
}
