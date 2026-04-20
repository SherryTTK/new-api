package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

const (
	tokenRpmDuration int64 = 60 // 1 minute in seconds
)

var tokenRateLimiter common.InMemoryRateLimiter

func TokenRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		rpmLimit, exists := c.Get("token_rpm_limit")
		if !exists {
			c.Next()
			return
		}
		limit, ok := rpmLimit.(int)
		if !ok || limit <= 0 {
			c.Next()
			return
		}

		tokenId := c.GetInt("token_id")
		key := fmt.Sprintf("token_rpm:%d", tokenId)

		var allowed bool
		if common.RedisEnabled {
			allowed = redisTokenRateLimit(c, key, limit)
		} else {
			tokenRateLimiter.Init(time.Minute)
			allowed = tokenRateLimiter.Request(key, limit, tokenRpmDuration)
		}

		if !allowed {
			msg := fmt.Sprintf("该令牌已达到调用频率限制：每分钟最多%d次", limit)
			userId := c.GetInt("id")
			tokenName := c.GetString("token_name")
			group := c.GetString("group")
			modelName := getRequestModelName(c)
			model.RecordErrorLog(c, userId, 0, modelName, tokenName, msg, tokenId, 0, false, group, nil)
			abortWithOpenAiMessage(c, http.StatusTooManyRequests, msg)
			return
		}

		c.Next()
	}
}

func redisTokenRateLimit(c *gin.Context, key string, maxCount int) bool {
	ctx := context.Background()
	rdb := common.RDB

	allowed, err := checkRedisRateLimit(ctx, rdb, key, maxCount, tokenRpmDuration)
	if err != nil {
		return true
	}
	if !allowed {
		return false
	}

	recordRedisRequest(ctx, rdb, key, maxCount)
	return true
}

// getRequestModelName tries to extract the model name from the request for error logging.
// Best-effort: returns empty string if extraction fails.
func getRequestModelName(c *gin.Context) string {
	path := c.Request.URL.Path
	if strings.HasPrefix(path, "/v1beta/models/") || strings.HasPrefix(path, "/v1/models/") {
		return extractModelNameFromGeminiPath(path)
	}
	if model := c.Query("model"); model != "" {
		return model
	}
	var req struct {
		Model string `json:"model"`
	}
	if err := common.UnmarshalBodyReusable(c, &req); err == nil && req.Model != "" {
		return req.Model
	}
	return ""
}
