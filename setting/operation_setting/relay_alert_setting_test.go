package operation_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRelayAlertSettingKeepsEmptySlicesNonNil(t *testing.T) {
	original := relayAlertSetting
	t.Cleanup(func() { relayAlertSetting = original })

	relayAlertSetting = RelayAlertSetting{
		AggregationWindowMinutes: 5,
		Rules: []RelayAlertRule{
			{
				ID:         "rule-1",
				TargetIDs:  nil,
				Categories: nil,
			},
		},
	}

	setting := GetRelayAlertSetting()
	require.NotNil(t, setting.Rules)
	require.Len(t, setting.Rules, 1)
	assert.NotNil(t, setting.Rules[0].TargetIDs)
	assert.NotNil(t, setting.Rules[0].Categories)
}

func TestGetRelayAlertSettingReturnsEmptyRulesArray(t *testing.T) {
	original := relayAlertSetting
	t.Cleanup(func() { relayAlertSetting = original })

	relayAlertSetting = RelayAlertSetting{
		AggregationWindowMinutes: 5,
		Rules:                    nil,
	}

	setting := GetRelayAlertSetting()
	assert.NotNil(t, setting.Rules)
	assert.Empty(t, setting.Rules)
}
