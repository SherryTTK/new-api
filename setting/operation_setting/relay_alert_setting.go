package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

const (
	RelayAlertTargetUser    = "user"
	RelayAlertTargetToken   = "token"
	RelayAlertTargetChannel = "channel"

	RelayAlertLevelWarning = "warning"
	RelayAlertLevelUrgent  = "urgent"

	RelayAlertCategoryHTTP429           = "http_429"
	RelayAlertCategoryHTTP5xx           = "http_5xx"
	RelayAlertCategoryNetworkError      = "network_error"
	RelayAlertCategoryResponseParse     = "response_parse_error"
	RelayAlertCategoryEmptyResponse     = "empty_response"
	RelayAlertCategoryStreamError       = "stream_error"
	RelayAlertCategoryInsufficientQuota = "insufficient_quota"
	RelayAlertCategoryTokenExpired      = "token_expired"
	RelayAlertCategoryTokenExhausted    = "token_exhausted"
)

var relayAlertCategories = map[string]struct{}{
	RelayAlertCategoryHTTP429:           {},
	RelayAlertCategoryHTTP5xx:           {},
	RelayAlertCategoryNetworkError:      {},
	RelayAlertCategoryResponseParse:     {},
	RelayAlertCategoryEmptyResponse:     {},
	RelayAlertCategoryStreamError:       {},
	RelayAlertCategoryInsufficientQuota: {},
	RelayAlertCategoryTokenExpired:      {},
	RelayAlertCategoryTokenExhausted:    {},
}

type RelayAlertRule struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Enabled    bool     `json:"enabled"`
	TargetType string   `json:"target_type"`
	TargetIDs  []int    `json:"target_ids"`
	Categories []string `json:"categories"`
	Level      string   `json:"level"`
}

type RelayAlertSetting struct {
	Enabled                  bool             `json:"enabled"`
	FeishuWebhookURL         string           `json:"feishu_webhook_url"`
	FeishuSecret             string           `json:"feishu_secret"`
	AggregationWindowMinutes int              `json:"aggregation_window_minutes"`
	Rules                    []RelayAlertRule `json:"rules"`
}

var relayAlertSetting = RelayAlertSetting{
	Enabled:                  false,
	AggregationWindowMinutes: 5,
	Rules:                    []RelayAlertRule{},
}

func init() {
	config.GlobalConfig.Register("relay_alert_setting", &relayAlertSetting)
}

func GetRelayAlertSetting() RelayAlertSetting {
	setting := relayAlertSetting
	setting.Rules = make([]RelayAlertRule, len(relayAlertSetting.Rules))
	copy(setting.Rules, relayAlertSetting.Rules)
	for i := range setting.Rules {
		targetIDs := make([]int, len(setting.Rules[i].TargetIDs))
		copy(targetIDs, setting.Rules[i].TargetIDs)
		setting.Rules[i].TargetIDs = targetIDs

		categories := make([]string, len(setting.Rules[i].Categories))
		copy(categories, setting.Rules[i].Categories)
		setting.Rules[i].Categories = categories
	}
	if setting.AggregationWindowMinutes < 1 || setting.AggregationWindowMinutes > 60 {
		setting.AggregationWindowMinutes = 5
	}
	return setting
}

func IsRelayAlertCategory(category string) bool {
	_, ok := relayAlertCategories[category]
	return ok
}
