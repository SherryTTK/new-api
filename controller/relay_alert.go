package controller

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
)

type relayAlertSettingResponse struct {
	Enabled                  bool                               `json:"enabled"`
	AggregationWindowMinutes int                                `json:"aggregation_window_minutes"`
	Rules                    []operation_setting.RelayAlertRule `json:"rules"`
	WebhookConfigured        bool                               `json:"webhook_configured"`
	WebhookMasked            string                             `json:"webhook_masked"`
	SecretConfigured         bool                               `json:"secret_configured"`
}

type updateRelayAlertSettingRequest struct {
	Enabled                  bool                               `json:"enabled"`
	AggregationWindowMinutes int                                `json:"aggregation_window_minutes"`
	Rules                    []operation_setting.RelayAlertRule `json:"rules"`
	FeishuWebhookURL         *string                            `json:"feishu_webhook_url"`
	FeishuSecret             *string                            `json:"feishu_secret"`
	ClearWebhook             bool                               `json:"clear_webhook"`
	ClearSecret              bool                               `json:"clear_secret"`
}

type relayAlertTarget struct {
	ID          int    `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

func GetRelayAlertSetting(c *gin.Context) {
	setting := operation_setting.GetRelayAlertSetting()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": relayAlertSettingResponse{
			Enabled:                  setting.Enabled,
			AggregationWindowMinutes: setting.AggregationWindowMinutes,
			Rules:                    setting.Rules,
			WebhookConfigured:        setting.FeishuWebhookURL != "",
			WebhookMasked:            maskRelayAlertWebhook(setting.FeishuWebhookURL),
			SecretConfigured:         setting.FeishuSecret != "",
		},
	})
}

func UpdateRelayAlertSetting(c *gin.Context) {
	var request updateRelayAlertSettingRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorMsg(c, "无效的请求异常告警配置")
		return
	}
	if err := normalizeRelayAlertSettingRequest(&request); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}

	current := operation_setting.GetRelayAlertSetting()
	webhook := current.FeishuWebhookURL
	secret := current.FeishuSecret
	if request.ClearWebhook {
		webhook = ""
	}
	if request.ClearSecret {
		secret = ""
	}
	if request.FeishuWebhookURL != nil {
		webhook = strings.TrimSpace(*request.FeishuWebhookURL)
	}
	if request.FeishuSecret != nil {
		secret = strings.TrimSpace(*request.FeishuSecret)
	}
	if webhook != "" {
		parsedURL, err := url.Parse(webhook)
		if err != nil || parsedURL.Host == "" || (parsedURL.Scheme != "https" && parsedURL.Scheme != "http") {
			common.ApiErrorMsg(c, "飞书机器人 Webhook URL 无效")
			return
		}
	}
	if len(secret) > 512 {
		common.ApiErrorMsg(c, "飞书机器人签名密钥过长")
		return
	}
	if request.Enabled && webhook == "" {
		common.ApiErrorMsg(c, "启用请求异常告警前必须配置飞书机器人 Webhook URL")
		return
	}

	rulesJSON, err := common.Marshal(request.Rules)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	values := map[string]string{
		"relay_alert_setting.enabled":                    strconv.FormatBool(request.Enabled),
		"relay_alert_setting.aggregation_window_minutes": strconv.Itoa(request.AggregationWindowMinutes),
		"relay_alert_setting.rules":                      string(rulesJSON),
		"relay_alert_setting.feishu_webhook_url":         webhook,
		"relay_alert_setting.feishu_secret":              secret,
	}
	if err := model.UpdateOptionsBulk(values); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "option.update", map[string]interface{}{"key": "relay_alert_setting"})
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func TestRelayAlertSetting(c *gin.Context) {
	var request struct {
		Level string `json:"level"`
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorMsg(c, "无效的测试告警请求")
		return
	}
	if err := service.TestFeishuRelayAlert(request.Level); err != nil {
		common.ApiErrorMsg(c, "飞书测试消息发送失败: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func SearchRelayAlertTargets(c *gin.Context) {
	targetType := c.Query("type")
	keyword := strings.TrimSpace(c.Query("keyword"))
	results := make([]relayAlertTarget, 0)
	switch targetType {
	case operation_setting.RelayAlertTargetUser:
		users, err := model.SearchRelayAlertUsers(keyword, 20)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		for _, user := range users {
			label := user.Username
			if user.DisplayName != "" {
				label = user.DisplayName + " (" + user.Username + ")"
			}
			results = append(results, relayAlertTarget{ID: user.Id, Label: label, Description: user.Email})
		}
	case operation_setting.RelayAlertTargetToken:
		tokens, err := model.SearchRelayAlertTokens(keyword, 20)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		for _, token := range tokens {
			results = append(results, relayAlertTarget{ID: token.Id, Label: token.Name, Description: fmt.Sprintf("用户 #%d", token.UserId)})
		}
	case operation_setting.RelayAlertTargetChannel:
		channels, err := model.SearchRelayAlertChannels(keyword, 20)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		for _, channel := range channels {
			results = append(results, relayAlertTarget{ID: channel.Id, Label: channel.Name, Description: fmt.Sprintf("类型 %d", channel.Type)})
		}
	default:
		common.ApiErrorMsg(c, "无效的告警目标类型")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": results})
}

func normalizeRelayAlertSettingRequest(request *updateRelayAlertSettingRequest) error {
	if request.AggregationWindowMinutes < 1 || request.AggregationWindowMinutes > 60 {
		return errors.New("聚合窗口必须在 1 到 60 分钟之间")
	}
	if len(request.Rules) > 100 {
		return errors.New("告警规则不能超过 100 条")
	}
	seenRuleIDs := make(map[string]struct{}, len(request.Rules))
	for i := range request.Rules {
		rule := &request.Rules[i]
		rule.Name = strings.TrimSpace(rule.Name)
		if rule.Name == "" || len(rule.Name) > 100 {
			return fmt.Errorf("第 %d 条规则名称无效", i+1)
		}
		if rule.ID == "" {
			rule.ID = common.GetUUID()
		}
		if _, exists := seenRuleIDs[rule.ID]; exists {
			return fmt.Errorf("第 %d 条规则 ID 重复", i+1)
		}
		seenRuleIDs[rule.ID] = struct{}{}
		if rule.TargetType != operation_setting.RelayAlertTargetUser &&
			rule.TargetType != operation_setting.RelayAlertTargetToken &&
			rule.TargetType != operation_setting.RelayAlertTargetChannel {
			return fmt.Errorf("第 %d 条规则目标类型无效", i+1)
		}
		if len(rule.TargetIDs) == 0 || len(rule.TargetIDs) > 500 {
			return fmt.Errorf("第 %d 条规则必须选择 1 到 500 个目标", i+1)
		}
		rule.TargetIDs = uniquePositiveRelayAlertIDs(rule.TargetIDs)
		if len(rule.TargetIDs) == 0 {
			return fmt.Errorf("第 %d 条规则目标无效", i+1)
		}
		if len(rule.Categories) == 0 {
			return fmt.Errorf("第 %d 条规则必须选择异常类型", i+1)
		}
		categorySet := make(map[string]struct{}, len(rule.Categories))
		for _, category := range rule.Categories {
			if !operation_setting.IsRelayAlertCategory(category) {
				return fmt.Errorf("第 %d 条规则包含无效异常类型", i+1)
			}
			categorySet[category] = struct{}{}
		}
		rule.Categories = rule.Categories[:0]
		for category := range categorySet {
			rule.Categories = append(rule.Categories, category)
		}
		sort.Strings(rule.Categories)
		if rule.Level != operation_setting.RelayAlertLevelWarning && rule.Level != operation_setting.RelayAlertLevelUrgent {
			return fmt.Errorf("第 %d 条规则告警级别无效", i+1)
		}
	}
	return nil
}

func uniquePositiveRelayAlertIDs(ids []int) []int {
	seen := make(map[int]struct{}, len(ids))
	result := make([]int, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	sort.Ints(result)
	return result
}

func maskRelayAlertWebhook(value string) string {
	if value == "" {
		return ""
	}
	parsedURL, err := url.Parse(value)
	if err != nil || parsedURL.Host == "" {
		return "********"
	}
	path := strings.TrimRight(parsedURL.Path, "/")
	lastSlash := strings.LastIndex(path, "/")
	if lastSlash >= 0 {
		path = path[:lastSlash+1] + "********"
	} else {
		path = "/********"
	}
	return parsedURL.Scheme + "://" + parsedURL.Host + path
}
