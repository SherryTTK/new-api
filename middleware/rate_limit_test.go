package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newRateLimitTestRequest(method string, target string, remoteAddr string) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	req.RemoteAddr = remoteAddr
	return req
}

func TestSandboxPathSkipsGlobalAPIRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	originalRedisEnabled := common.RedisEnabled
	originalGlobalEnabled := common.GlobalApiRateLimitEnable
	originalGlobalNum := common.GlobalApiRateLimitNum
	originalGlobalDuration := common.GlobalApiRateLimitDuration
	originalExpiration := common.RateLimitKeyExpirationDuration
	originalLimiter := inMemoryRateLimiter

	common.RedisEnabled = false
	common.GlobalApiRateLimitEnable = true
	common.GlobalApiRateLimitNum = 1
	common.GlobalApiRateLimitDuration = 3600
	common.RateLimitKeyExpirationDuration = time.Hour
	inMemoryRateLimiter = common.InMemoryRateLimiter{}

	t.Cleanup(func() {
		common.RedisEnabled = originalRedisEnabled
		common.GlobalApiRateLimitEnable = originalGlobalEnabled
		common.GlobalApiRateLimitNum = originalGlobalNum
		common.GlobalApiRateLimitDuration = originalGlobalDuration
		common.RateLimitKeyExpirationDuration = originalExpiration
		inMemoryRateLimiter = originalLimiter
	})

	router := gin.New()
	router.Use(GlobalAPIRateLimit())
	router.GET("/api/sandbox/log/self", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	router.GET("/api/non-sandbox", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	for i := 0; i < 3; i++ {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, newRateLimitTestRequest(http.MethodGet, "/api/sandbox/log/self", "198.51.100.10:12345"))
		require.Equal(t, http.StatusOK, recorder.Code)
	}

	first := httptest.NewRecorder()
	router.ServeHTTP(first, newRateLimitTestRequest(http.MethodGet, "/api/non-sandbox", "198.51.100.11:12345"))
	require.Equal(t, http.StatusOK, first.Code)

	second := httptest.NewRecorder()
	router.ServeHTTP(second, newRateLimitTestRequest(http.MethodGet, "/api/non-sandbox", "198.51.100.11:12345"))
	require.Equal(t, http.StatusTooManyRequests, second.Code)
}

func TestSandboxPathSkipsCriticalRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	originalRedisEnabled := common.RedisEnabled
	originalCriticalEnabled := common.CriticalRateLimitEnable
	originalCriticalNum := common.CriticalRateLimitNum
	originalCriticalDuration := common.CriticalRateLimitDuration
	originalExpiration := common.RateLimitKeyExpirationDuration
	originalLimiter := inMemoryRateLimiter

	common.RedisEnabled = false
	common.CriticalRateLimitEnable = true
	common.CriticalRateLimitNum = 1
	common.CriticalRateLimitDuration = 3600
	common.RateLimitKeyExpirationDuration = time.Hour
	inMemoryRateLimiter = common.InMemoryRateLimiter{}

	t.Cleanup(func() {
		common.RedisEnabled = originalRedisEnabled
		common.CriticalRateLimitEnable = originalCriticalEnabled
		common.CriticalRateLimitNum = originalCriticalNum
		common.CriticalRateLimitDuration = originalCriticalDuration
		common.RateLimitKeyExpirationDuration = originalExpiration
		inMemoryRateLimiter = originalLimiter
	})

	router := gin.New()
	router.Use(CriticalRateLimit())
	router.GET("/api/sandbox/organizations", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	for i := 0; i < 3; i++ {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, newRateLimitTestRequest(http.MethodGet, "/api/sandbox/organizations", "198.51.100.12:12345"))
		require.Equal(t, http.StatusOK, recorder.Code)
	}
}
