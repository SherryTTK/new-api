package service

import (
	"math"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFixedResponseQuotaPricing(t *testing.T) {
	for _, tc := range []struct {
		name       string
		perCall    bool
		expr       string
		modelRatio float64
		expected   int
		clamped    bool
		cache      bool
	}{
		{name: "token and group multipliers", modelRatio: 2, expected: 1200},
		{name: "per call", perCall: true, expected: 7500},
		{name: "cache multipliers", cache: true, modelRatio: 2, expected: 9000},
		{name: "cache per call", cache: true, perCall: true, expected: 7500},
		{name: "cache expression", cache: true, expr: `p * 2 + c * 10 + cr * 0.2 + cc * 2.5`, expected: 3200},
		{name: "cache expression folded into input", cache: true, expr: `p * 2 + c * 10`, expected: 3600},
		{name: "expression", expr: `tier("local", p * 2 + c * 10)`, expected: 600},
		{name: "saturation", modelRatio: math.MaxFloat64, expected: common.MaxQuota, clamped: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			info := &relaycommon.RelayInfo{FixedResponse: true, OriginModelName: "test-search-preview", StartTime: time.Now(), PriceData: types.PriceData{ModelRatio: tc.modelRatio, CompletionRatio: 3, GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1.5}, UsePrice: tc.perCall, ModelPrice: 0.01}}
			usage := &dto.Usage{PromptTokens: 100, CompletionTokens: 100, TotalTokens: 200}
			if tc.cache {
				usage.PromptTokens += 3000
				usage.TotalTokens += 3000
				usage.UsageSemantic = dto.BillingUsageSemanticOpenAI
				usage.PromptTokensDetails = dto.InputTokenDetails{CachedTokens: 1000, CacheWriteTokens: 2000}
				info.PriceData.CacheRatio = 0.1
				info.PriceData.CacheCreationRatio = 1.25
			}
			if tc.expr != "" {
				info.TieredBillingSnapshot = &billingexpr.BillingSnapshot{BillingMode: "tiered_expr", ExprString: tc.expr, ExprHash: billingexpr.ExprHashString(tc.expr), GroupRatio: 1, QuotaPerUnit: 500000}
			}
			quota := FixedResponseQuota(c, info, usage)
			assert.Equal(t, tc.expected, quota)
			if tc.clamped {
				require.NotNil(t, info.QuotaClamp)
			} else {
				assert.Nil(t, info.QuotaClamp)
			}
			// Merely naming a search model must not charge for an unexecuted tool.
			summary := calculateTextQuotaSummary(c, info, usage)
			assert.Empty(t, summary.ToolSurchargeItems)
		})
	}
}
