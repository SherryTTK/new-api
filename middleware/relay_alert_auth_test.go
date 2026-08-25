package middleware

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/stretchr/testify/assert"
)

func TestTokenInvalidRelayAlertCategory(t *testing.T) {
	now := int64(1000)
	tests := []struct {
		name     string
		token    *model.Token
		expected string
	}{
		{
			name:     "explicit expired status",
			token:    &model.Token{Status: common.TokenStatusExpired},
			expected: operation_setting.RelayAlertCategoryTokenExpired,
		},
		{
			name:     "enabled token expired by timestamp",
			token:    &model.Token{Status: common.TokenStatusEnabled, ExpiredTime: now - 1, RemainQuota: 10},
			expected: operation_setting.RelayAlertCategoryTokenExpired,
		},
		{
			name:     "explicit exhausted status",
			token:    &model.Token{Status: common.TokenStatusExhausted},
			expected: operation_setting.RelayAlertCategoryTokenExhausted,
		},
		{
			name:     "enabled token quota exhausted",
			token:    &model.Token{Status: common.TokenStatusEnabled, ExpiredTime: -1, RemainQuota: 0},
			expected: operation_setting.RelayAlertCategoryTokenExhausted,
		},
		{
			name:     "disabled token is excluded even with stale expiry",
			token:    &model.Token{Status: common.TokenStatusDisabled, ExpiredTime: now - 1, RemainQuota: 0},
			expected: "",
		},
		{
			name:     "missing token is excluded",
			token:    nil,
			expected: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, tokenInvalidRelayAlertCategory(test.token, now))
		})
	}
}
