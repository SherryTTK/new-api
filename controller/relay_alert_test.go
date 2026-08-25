package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeRelayAlertSettingRequest(t *testing.T) {
	t.Run("normalizes duplicate targets and categories", func(t *testing.T) {
		request := updateRelayAlertSettingRequest{
			AggregationWindowMinutes: 5,
			Rules: []operation_setting.RelayAlertRule{
				{
					Name:       " Production ",
					Enabled:    true,
					TargetType: operation_setting.RelayAlertTargetUser,
					TargetIDs:  []int{7, 3, 7, -1},
					Categories: []string{
						operation_setting.RelayAlertCategoryHTTP5xx,
						operation_setting.RelayAlertCategoryHTTP429,
						operation_setting.RelayAlertCategoryHTTP5xx,
					},
					Level: operation_setting.RelayAlertLevelWarning,
				},
			},
		}

		require.NoError(t, normalizeRelayAlertSettingRequest(&request))
		require.Len(t, request.Rules, 1)
		rule := request.Rules[0]
		assert.NotEmpty(t, rule.ID)
		assert.Equal(t, "Production", rule.Name)
		assert.Equal(t, []int{3, 7}, rule.TargetIDs)
		assert.Equal(t, []string{
			operation_setting.RelayAlertCategoryHTTP429,
			operation_setting.RelayAlertCategoryHTTP5xx,
		}, rule.Categories)
	})

	t.Run("rejects invalid aggregation window", func(t *testing.T) {
		request := updateRelayAlertSettingRequest{AggregationWindowMinutes: 0}
		assert.Error(t, normalizeRelayAlertSettingRequest(&request))
	})

	t.Run("rejects unknown category", func(t *testing.T) {
		request := updateRelayAlertSettingRequest{
			AggregationWindowMinutes: 5,
			Rules: []operation_setting.RelayAlertRule{
				{
					Name:       "Production",
					TargetType: operation_setting.RelayAlertTargetChannel,
					TargetIDs:  []int{1},
					Categories: []string{"unknown"},
					Level:      operation_setting.RelayAlertLevelUrgent,
				},
			},
		}
		assert.Error(t, normalizeRelayAlertSettingRequest(&request))
	})
}
