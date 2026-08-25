package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

const (
	relayAlertAggregatePrefix = "relay_alert:aggregate:"
	relayAlertDueKey          = "relay_alert:due"
	relayAlertQueueSize       = 256
)

type RelayAlertEvent struct {
	RequestID      string    `json:"request_id"`
	OccurredAt     time.Time `json:"occurred_at"`
	DurationMillis int64     `json:"duration_millis"`
	NodeName       string    `json:"node_name"`
	RelayFormat    string    `json:"relay_format"`
	RequestPath    string    `json:"request_path"`
	Model          string    `json:"model"`
	Stream         bool      `json:"stream"`
	UserID         int       `json:"user_id"`
	UserName       string    `json:"user_name"`
	TokenID        int       `json:"token_id"`
	TokenName      string    `json:"token_name"`
	ChannelID      int       `json:"channel_id"`
	ChannelName    string    `json:"channel_name"`
	ChannelChain   []string  `json:"channel_chain"`
	AttemptCount   int       `json:"attempt_count"`
	Category       string    `json:"category"`
	StatusCode     int       `json:"status_code"`
	ErrorCode      string    `json:"error_code"`
	ErrorType      string    `json:"error_type"`
	StreamEnd      string    `json:"stream_end"`
	MatchedRuleIDs []string  `json:"matched_rule_ids"`
	MatchedRules   []string  `json:"matched_rules"`
	Level          string    `json:"level"`
	Count          int64     `json:"count"`
}

type relayAlertDelivery struct {
	Event   RelayAlertEvent
	Summary bool
}

var (
	relayAlertStartOnce  sync.Once
	relayAlertQueue      = make(chan relayAlertDelivery, relayAlertQueueSize)
	relayAlertHTTPClient = &http.Client{
		Timeout: 10 * time.Second,
	}
)

func IsMonitoredRelayAlertPath(path string) bool {
	switch path {
	case "/v1/chat/completions", "/v1/completions", "/v1/responses", "/v1/responses/compact", "/v1/messages":
		return true
	}
	return (strings.HasPrefix(path, "/v1beta/models/") || strings.HasPrefix(path, "/v1/models/")) &&
		(strings.HasSuffix(path, ":generateContent") || strings.HasSuffix(path, ":streamGenerateContent"))
}

func FinalizeRelayAlert(c *gin.Context, info *relaycommon.RelayInfo, apiErr *types.NewAPIError) {
	if c == nil || info == nil || c.Request == nil || c.Request.URL == nil || !IsMonitoredRelayAlertPath(c.Request.URL.Path) {
		return
	}
	category, streamEnd := classifyRelayAlert(c, info, apiErr)
	if category == "" {
		return
	}

	channelID := 0
	if info.ChannelMeta != nil {
		channelID = info.ChannelMeta.ChannelId
	}
	channelName := c.GetString("channel_name")
	if apiErr != nil && apiErr.GetErrorCode() == types.ErrorCodeGetChannelFailed {
		channelID = 0
		channelName = ""
	}

	event := RelayAlertEvent{
		RequestID:      info.RequestId,
		OccurredAt:     time.Now(),
		DurationMillis: time.Since(info.StartTime).Milliseconds(),
		NodeName:       common.NodeName,
		RelayFormat:    string(info.RelayFormat),
		RequestPath:    c.Request.URL.Path,
		Model:          info.OriginModelName,
		Stream:         info.IsStream,
		UserID:         info.UserId,
		UserName:       c.GetString("username"),
		TokenID:        info.TokenId,
		TokenName:      c.GetString("token_name"),
		ChannelID:      channelID,
		ChannelName:    channelName,
		ChannelChain:   append([]string(nil), c.GetStringSlice("use_channel")...),
		Category:       category,
		StreamEnd:      streamEnd,
		Count:          1,
	}
	event.AttemptCount = len(event.ChannelChain)
	if apiErr != nil {
		event.StatusCode = apiErr.GetOriginalStatusCode()
		event.ErrorCode = category
		event.ErrorType = string(apiErr.GetErrorType())
	}
	submitRelayAlertEvent(event)
}

func SubmitTokenAuthRelayAlert(c *gin.Context, token *model.Token, category string) {
	if c == nil || token == nil || c.Request == nil || c.Request.URL == nil || !IsMonitoredRelayAlertPath(c.Request.URL.Path) {
		return
	}
	if category != operation_setting.RelayAlertCategoryTokenExpired && category != operation_setting.RelayAlertCategoryTokenExhausted {
		return
	}
	event := RelayAlertEvent{
		RequestID:   c.GetString(common.RequestIdKey),
		OccurredAt:  time.Now(),
		NodeName:    common.NodeName,
		RequestPath: c.Request.URL.Path,
		UserID:      token.UserId,
		TokenID:     token.Id,
		TokenName:   token.Name,
		Category:    category,
		StatusCode:  http.StatusUnauthorized,
		ErrorCode:   category,
		ErrorType:   "token_auth_error",
		Count:       1,
	}
	submitRelayAlertEvent(event)
}

func TestFeishuRelayAlert(level string) error {
	if level != operation_setting.RelayAlertLevelWarning && level != operation_setting.RelayAlertLevelUrgent {
		return errors.New("invalid alert level")
	}
	event := RelayAlertEvent{
		RequestID:    "test-" + common.NewRequestId(),
		OccurredAt:   time.Now(),
		NodeName:     common.NodeName,
		RelayFormat:  "openai",
		RequestPath:  "/v1/chat/completions",
		Model:        "test-model",
		Category:     operation_setting.RelayAlertCategoryHTTP5xx,
		StatusCode:   http.StatusBadGateway,
		ErrorCode:    "test_alert",
		ErrorType:    "test",
		MatchedRules: []string{"Test rule"},
		Level:        level,
		Count:        1,
	}
	return sendFeishuRelayAlert(context.Background(), event, false)
}

func classifyRelayAlert(c *gin.Context, info *relaycommon.RelayInfo, apiErr *types.NewAPIError) (string, string) {
	if errors.Is(c.Request.Context().Err(), context.Canceled) {
		return "", ""
	}
	if info.IsStream && info.StreamStatus != nil {
		endReason := info.StreamStatus.EndReason
		if endReason == relaycommon.StreamEndReasonClientGone {
			return "", string(endReason)
		}
		if !info.StreamStatus.IsNormalEnd() || info.StreamStatus.HasErrors() {
			return operation_setting.RelayAlertCategoryStreamError, string(endReason)
		}
	}
	if apiErr == nil {
		if !info.HasValidOutput() {
			return operation_setting.RelayAlertCategoryEmptyResponse, ""
		}
		return "", ""
	}

	switch apiErr.GetErrorCode() {
	case types.ErrorCodeInsufficientUserQuota:
		return operation_setting.RelayAlertCategoryInsufficientQuota, ""
	case types.ErrorCodePreConsumeTokenQuotaFailed:
		if strings.Contains(strings.ToLower(apiErr.Error()), "token quota is not enough") {
			return operation_setting.RelayAlertCategoryTokenExhausted, ""
		}
		return "", ""
	case types.ErrorCodeEmptyResponse:
		return operation_setting.RelayAlertCategoryEmptyResponse, ""
	case types.ErrorCodeDoRequestFailed, types.ErrorCodeChannelResponseTimeExceeded:
		return operation_setting.RelayAlertCategoryNetworkError, ""
	case types.ErrorCodeReadResponseBodyFailed, types.ErrorCodeBadResponse, types.ErrorCodeBadResponseBody:
		return operation_setting.RelayAlertCategoryResponseParse, ""
	}

	var networkErr net.Error
	if errors.As(apiErr, &networkErr) {
		return operation_setting.RelayAlertCategoryNetworkError, ""
	}
	if info.ChannelMeta == nil || info.ChannelMeta.ChannelId <= 0 || apiErr.GetErrorCode() == types.ErrorCodeGetChannelFailed {
		return "", ""
	}
	isUpstreamStatusError := apiErr.GetErrorType() != types.ErrorTypeNewAPIError ||
		apiErr.GetErrorCode() == types.ErrorCodeBadResponseStatusCode
	if !isUpstreamStatusError {
		return "", ""
	}
	statusCode := apiErr.GetOriginalStatusCode()
	if statusCode == http.StatusTooManyRequests {
		return operation_setting.RelayAlertCategoryHTTP429, ""
	}
	if statusCode >= http.StatusInternalServerError && statusCode <= 599 {
		return operation_setting.RelayAlertCategoryHTTP5xx, ""
	}
	return "", ""
}

func submitRelayAlertEvent(event RelayAlertEvent) {
	setting := operation_setting.GetRelayAlertSetting()
	if !setting.Enabled || setting.FeishuWebhookURL == "" {
		return
	}

	matchedIDs, matchedNames, level := matchRelayAlertRules(setting.Rules, event)
	if len(matchedIDs) == 0 {
		return
	}
	sort.Strings(matchedIDs)
	sort.Strings(matchedNames)
	event.MatchedRuleIDs = matchedIDs
	event.MatchedRules = matchedNames
	event.Level = level

	startRelayAlertWorkers()
	if !common.RedisEnabled || common.RDB == nil {
		enqueueRelayAlert(relayAlertDelivery{Event: event})
		return
	}
	first, err := aggregateRelayAlert(event, time.Duration(setting.AggregationWindowMinutes)*time.Minute)
	if err != nil {
		common.SysError("relay alert aggregation failed: " + err.Error())
		enqueueRelayAlert(relayAlertDelivery{Event: event})
		return
	}
	if first {
		enqueueRelayAlert(relayAlertDelivery{Event: event})
	}
}

func matchRelayAlertRules(rules []operation_setting.RelayAlertRule, event RelayAlertEvent) ([]string, []string, string) {
	matchedIDs := make([]string, 0)
	matchedNames := make([]string, 0)
	level := operation_setting.RelayAlertLevelWarning
	for _, rule := range rules {
		if !rule.Enabled || !relayAlertRuleMatches(rule, event) {
			continue
		}
		matchedIDs = append(matchedIDs, rule.ID)
		matchedNames = append(matchedNames, rule.Name)
		if rule.Level == operation_setting.RelayAlertLevelUrgent {
			level = operation_setting.RelayAlertLevelUrgent
		}
	}
	sort.Strings(matchedIDs)
	sort.Strings(matchedNames)
	return matchedIDs, matchedNames, level
}

func relayAlertRuleMatches(rule operation_setting.RelayAlertRule, event RelayAlertEvent) bool {
	if !containsRelayAlertString(rule.Categories, event.Category) {
		return false
	}
	var targetID int
	switch rule.TargetType {
	case operation_setting.RelayAlertTargetUser:
		targetID = event.UserID
	case operation_setting.RelayAlertTargetToken:
		targetID = event.TokenID
	case operation_setting.RelayAlertTargetChannel:
		targetID = event.ChannelID
	default:
		return false
	}
	if targetID <= 0 {
		return false
	}
	for _, id := range rule.TargetIDs {
		if id == targetID {
			return true
		}
	}
	return false
}

func aggregateRelayAlert(event RelayAlertEvent, window time.Duration) (bool, error) {
	fingerprint := relayAlertFingerprint(event)
	key := relayAlertAggregatePrefix + fingerprint
	dueAt := time.Now().Add(window).Unix()
	eventJSON, err := common.Marshal(event)
	if err != nil {
		return false, err
	}
	ttlSeconds := int64((window + 24*time.Hour).Seconds())
	script := redis.NewScript(`
if redis.call('EXISTS', KEYS[1]) == 0 then
  redis.call('HSET', KEYS[1], 'count', 1, 'event', ARGV[1])
  redis.call('EXPIRE', KEYS[1], ARGV[2])
  redis.call('ZADD', KEYS[2], ARGV[3], ARGV[4])
  return 1
end
redis.call('HINCRBY', KEYS[1], 'count', 1)
return 0
`)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	result, err := script.Run(ctx, common.RDB, []string{key, relayAlertDueKey}, string(eventJSON), ttlSeconds, dueAt, fingerprint).Int()
	return result == 1, err
}

func startRelayAlertWorkers() {
	relayAlertStartOnce.Do(func() {
		gopool.Go(func() {
			for delivery := range relayAlertQueue {
				if err := sendFeishuRelayAlert(context.Background(), delivery.Event, delivery.Summary); err != nil {
					common.SysError("failed to send Feishu relay alert: " + err.Error())
				}
			}
		})
		gopool.Go(func() {
			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				flushDueRelayAlerts()
			}
		})
	})
}

func StartRelayAlertService() {
	startRelayAlertWorkers()
}

func flushDueRelayAlerts() {
	if !common.RedisEnabled || common.RDB == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	fingerprints, err := common.RDB.ZRangeByScore(ctx, relayAlertDueKey, &redis.ZRangeBy{
		Min:    "-inf",
		Max:    strconv.FormatInt(time.Now().Unix(), 10),
		Offset: 0,
		Count:  50,
	}).Result()
	if err != nil {
		common.SysError("failed to query due relay alerts: " + err.Error())
		return
	}
	for _, fingerprint := range fingerprints {
		flushRelayAlertAggregate(ctx, fingerprint)
	}
}

func flushRelayAlertAggregate(ctx context.Context, fingerprint string) {
	key := relayAlertAggregatePrefix + fingerprint
	script := redis.NewScript(`
local score = redis.call('ZSCORE', KEYS[2], ARGV[1])
if not score or tonumber(score) > tonumber(ARGV[2]) then
  return {}
end
local count = redis.call('HGET', KEYS[1], 'count')
local event = redis.call('HGET', KEYS[1], 'event')
redis.call('DEL', KEYS[1])
redis.call('ZREM', KEYS[2], ARGV[1])
if not count or not event then
  return {}
end
return {count, event}
`)
	values, err := script.Run(ctx, common.RDB, []string{key, relayAlertDueKey}, fingerprint, time.Now().Unix()).Slice()
	if err != nil {
		common.SysError("failed to pop relay alert aggregate: " + err.Error())
		return
	}
	if len(values) != 2 {
		return
	}
	count, _ := strconv.ParseInt(fmt.Sprint(values[0]), 10, 64)
	var event RelayAlertEvent
	if err := common.Unmarshal([]byte(fmt.Sprint(values[1])), &event); err != nil {
		common.SysError("failed to decode relay alert aggregate: " + err.Error())
		return
	}
	if count > 1 {
		event.Count = count
		enqueueRelayAlert(relayAlertDelivery{Event: event, Summary: true})
	}
}

func enqueueRelayAlert(delivery relayAlertDelivery) {
	select {
	case relayAlertQueue <- delivery:
	default:
		common.SysError("relay alert delivery queue is full; dropping alert")
	}
}

func relayAlertFingerprint(event RelayAlertEvent) string {
	value := strings.Join([]string{
		event.Category,
		strconv.Itoa(event.UserID),
		strconv.Itoa(event.TokenID),
		strconv.Itoa(event.ChannelID),
		event.RequestPath,
		event.Model,
		event.Level,
		strings.Join(event.MatchedRuleIDs, ","),
	}, "|")
	hash := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%x", hash[:])
}

func sendFeishuRelayAlert(ctx context.Context, event RelayAlertEvent, summary bool) error {
	setting := operation_setting.GetRelayAlertSetting()
	if setting.FeishuWebhookURL == "" {
		return errors.New("Feishu webhook URL is not configured")
	}
	parsedURL, err := url.Parse(setting.FeishuWebhookURL)
	if err != nil || parsedURL.Host == "" || (parsedURL.Scheme != "https" && parsedURL.Scheme != "http") {
		return errors.New("invalid Feishu webhook URL")
	}

	title := "AI 请求异常告警"
	template := "orange"
	if event.Level == operation_setting.RelayAlertLevelUrgent {
		title = "AI 请求紧急告警"
		template = "red"
	}
	if summary {
		title += "（聚合）"
	}

	content := buildRelayAlertCardContent(event, summary)
	elements := []map[string]any{
		{
			"tag":     "markdown",
			"content": content,
		},
	}
	payload := map[string]any{
		"msg_type": "interactive",
		"card": map[string]any{
			"header": map[string]any{
				"template": template,
				"title": map[string]any{
					"tag":     "plain_text",
					"content": title,
				},
			},
			"elements": elements,
		},
	}
	if setting.FeishuSecret != "" {
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		stringToSign := timestamp + "\n" + setting.FeishuSecret
		mac := hmac.New(sha256.New, []byte(stringToSign))
		signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))
		payload["timestamp"] = timestamp
		payload["sign"] = signature
	}
	body, err := common.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, setting.FeishuWebhookURL, strings.NewReader(string(body)))
	if err != nil {
		return errors.New("failed to create Feishu request")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := relayAlertHTTPClient.Do(req)
	if err != nil {
		var urlErr *url.Error
		if errors.As(err, &urlErr) && urlErr.Err != nil {
			return fmt.Errorf("Feishu request failed: %v", urlErr.Err)
		}
		return errors.New("Feishu request failed")
	}
	defer CloseResponseBodyGracefully(resp)
	var response struct {
		Code          *int   `json:"code"`
		Msg           string `json:"msg"`
		StatusCode    *int   `json:"StatusCode"`
		StatusMessage string `json:"StatusMessage"`
	}
	if err := common.DecodeJson(resp.Body, &response); err != nil {
		return fmt.Errorf("invalid Feishu response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Feishu HTTP status %d", resp.StatusCode)
	}
	if response.Code == nil && response.StatusCode == nil {
		return errors.New("Feishu response did not include a result code")
	}
	if response.Code != nil && *response.Code != 0 {
		return fmt.Errorf("Feishu error %d: %s", *response.Code, response.Msg)
	}
	if response.StatusCode != nil && *response.StatusCode != 0 {
		return fmt.Errorf("Feishu error %d: %s", *response.StatusCode, response.StatusMessage)
	}
	return nil
}

func buildRelayAlertCardContent(event RelayAlertEvent, summary bool) string {
	var builder strings.Builder
	if event.Level == operation_setting.RelayAlertLevelUrgent {
		builder.WriteString("<at id=all></at>\n")
	}
	if summary {
		fmt.Fprintf(&builder, "**聚合数量：** %d 条\n", event.Count)
	}
	fmt.Fprintf(&builder, "**异常类型：** %s\n", relayAlertCategoryLabel(event.Category))
	if event.StatusCode != 0 {
		fmt.Fprintf(&builder, "**HTTP 状态：** %d\n", event.StatusCode)
	}
	if event.Model != "" {
		fmt.Fprintf(&builder, "**模型：** %s\n", sanitizeRelayAlertMarkdown(event.Model))
	}
	fmt.Fprintf(&builder, "**接口：** %s\n", sanitizeRelayAlertMarkdown(event.RequestPath))
	if event.UserID > 0 {
		fmt.Fprintf(&builder, "**用户：** %s (#%d)\n", relayAlertDisplayName(event.UserName), event.UserID)
	}
	if event.TokenID > 0 {
		fmt.Fprintf(&builder, "**Key：** %s (#%d)\n", relayAlertDisplayName(event.TokenName), event.TokenID)
	}
	if event.ChannelID > 0 {
		fmt.Fprintf(&builder, "**渠道：** %s (#%d)\n", relayAlertDisplayName(event.ChannelName), event.ChannelID)
	}
	if event.AttemptCount > 0 {
		fmt.Fprintf(&builder, "**尝试次数：** %d\n", event.AttemptCount)
	}
	if len(event.ChannelChain) > 0 {
		fmt.Fprintf(&builder, "**渠道链：** %s\n", sanitizeRelayAlertMarkdown(strings.Join(event.ChannelChain, " → ")))
	}
	if event.StreamEnd != "" {
		fmt.Fprintf(&builder, "**流结束原因：** %s\n", sanitizeRelayAlertMarkdown(event.StreamEnd))
	}
	if event.ErrorCode != "" {
		fmt.Fprintf(&builder, "**错误码：** %s\n", sanitizeRelayAlertMarkdown(event.ErrorCode))
	}
	if len(event.MatchedRules) > 0 {
		fmt.Fprintf(&builder, "**命中规则：** %s\n", sanitizeRelayAlertMarkdown(strings.Join(event.MatchedRules, ", ")))
	}
	fmt.Fprintf(&builder, "**请求 ID：** %s\n", sanitizeRelayAlertMarkdown(event.RequestID))
	if event.DurationMillis > 0 {
		fmt.Fprintf(&builder, "**耗时：** %d ms\n", event.DurationMillis)
	}
	fmt.Fprintf(&builder, "**节点：** %s\n", relayAlertDisplayName(event.NodeName))
	fmt.Fprintf(&builder, "**时间：** %s", event.OccurredAt.In(time.Local).Format("2006-01-02 15:04:05"))
	return builder.String()
}

func relayAlertCategoryLabel(category string) string {
	switch category {
	case operation_setting.RelayAlertCategoryHTTP429:
		return "上游 429"
	case operation_setting.RelayAlertCategoryHTTP5xx:
		return "上游 5xx"
	case operation_setting.RelayAlertCategoryNetworkError:
		return "网络 / DNS / 超时"
	case operation_setting.RelayAlertCategoryResponseParse:
		return "响应读取 / 解析异常"
	case operation_setting.RelayAlertCategoryEmptyResponse:
		return "空回"
	case operation_setting.RelayAlertCategoryStreamError:
		return "流式异常终止"
	case operation_setting.RelayAlertCategoryInsufficientQuota:
		return "用户余额或订阅额度不足"
	case operation_setting.RelayAlertCategoryTokenExpired:
		return "API Key 已过期"
	case operation_setting.RelayAlertCategoryTokenExhausted:
		return "API Key 额度已耗尽"
	default:
		return category
	}
}

func containsRelayAlertString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func sanitizeRelayAlertMarkdown(value string) string {
	return strings.NewReplacer("<", "‹", ">", "›", "\r", " ", "\n", " ").Replace(value)
}

func relayAlertDisplayName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	return sanitizeRelayAlertMarkdown(value)
}
