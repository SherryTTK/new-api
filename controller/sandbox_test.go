package controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type sandboxAPIResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type sandboxOrganizationResponse struct {
	ID             int    `json:"id"`
	OrganizationID string `json:"organization_id"`
	Created        bool   `json:"created"`
	CreatedTime    int64  `json:"created_time"`
	UpdatedTime    int64  `json:"updated_time"`
}

type sandboxTokenResponse struct {
	OrganizationID        string  `json:"organization_id"`
	SandboxOrganizationID int     `json:"sandbox_organization_id"`
	RelationID            int     `json:"relation_id"`
	TokenID               int     `json:"token_id"`
	Name                  string  `json:"name"`
	Key                   string  `json:"key"`
	Group                 string  `json:"group"`
	RemainAmountUSD       float64 `json:"remain_amount_usd"`
	ExpiredTime           int64   `json:"expired_time"`
	CreatedTime           int64   `json:"created_time"`
}

type sandboxTokenSelfResponse struct {
	TokenID         int     `json:"token_id"`
	Status          int     `json:"status"`
	RemainAmountUSD float64 `json:"remain_amount_usd"`
	ExpiredTime     int64   `json:"expired_time"`
	AutoRecovered   bool    `json:"auto_recovered"`
}

type sandboxDeleteResponse struct {
	TokenID int  `json:"token_id"`
	Deleted bool `json:"deleted"`
}

type sandboxLogListResponse struct {
	Page         int                  `json:"page"`
	PageSize     int                  `json:"page_size"`
	Total        int64                `json:"total"`
	Items        []sandboxLogItem     `json:"items"`
	UsageSummary *sandboxUsageSummary `json:"usage_summary"`
}

type sandboxLogItem struct {
	RequestID        string  `json:"request_id"`
	TokenID          int     `json:"token_id"`
	TokenName        string  `json:"token_name"`
	ModelName        string  `json:"model_name"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	AmountUSD        float64 `json:"amount_usd"`
}

type sandboxUsageSummary struct {
	StartTimestamp        int64   `json:"start_timestamp"`
	EndTimestamp          int64   `json:"end_timestamp"`
	TotalRequests         int64   `json:"total_requests"`
	TotalPromptTokens     int64   `json:"total_prompt_tokens"`
	TotalCompletionTokens int64   `json:"total_completion_tokens"`
	TotalAmountUSD        float64 `json:"total_amount_usd"`
}

func setupSandboxControllerTest(t *testing.T) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	originalSQLitePath := common.SQLitePath
	originalRedisEnabled := common.RedisEnabled
	originalDebugEnabled := common.DebugEnabled
	originalMasterNode := common.IsMasterNode
	originalMainDatabaseType := common.MainDatabaseType()
	originalLogDatabaseType := common.LogDatabaseType()
	originalUserUsableGroups := setting.UserUsableGroups2JSONString()
	originalGroupRatio := ratio_setting.GroupRatio2JSONString()

	common.SQLitePath = filepath.Join(t.TempDir(), "sandbox-controller-test.db") + "?_busy_timeout=30000"
	common.RedisEnabled = false
	common.DebugEnabled = false
	common.IsMasterNode = true

	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"sandbox":1,"sandbox-China":1}`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"default","sandbox":"sandbox","sandbox-China":"sandbox-China"}`))
	require.NoError(t, model.InitDB())
	require.NoError(t, model.InitLogDB())

	t.Cleanup(func() {
		if model.LOG_DB != nil && model.LOG_DB != model.DB {
			require.NoError(t, model.CloseDB())
		} else if model.DB != nil {
			sqlDB, err := model.DB.DB()
			require.NoError(t, err)
			require.NoError(t, sqlDB.Close())
		}
		common.SQLitePath = originalSQLitePath
		common.RedisEnabled = originalRedisEnabled
		common.DebugEnabled = originalDebugEnabled
		common.IsMasterNode = originalMasterNode
		common.SetDatabaseTypes(originalMainDatabaseType, originalLogDatabaseType)
		model.DB = nil
		model.LOG_DB = nil
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUserUsableGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatio))
	})
}

func newSandboxTestRouter() *gin.Engine {
	router := gin.New()

	secretRoute := router.Group("/api/sandbox")
	secretRoute.Use(middleware.SandboxSecretAuth())
	{
		secretRoute.POST("/organizations", CreateSandboxOrganization)
		secretRoute.POST("/organizations/:organization_id/tokens", CreateSandboxToken)
	}

	selfRoute := router.Group("/api/sandbox")
	selfRoute.Use(middleware.SandboxTokenAuthReadOnly())
	{
		selfRoute.PUT("/token/self", UpdateSandboxTokenSelf)
		selfRoute.DELETE("/token/self", DeleteSandboxTokenSelf)
		selfRoute.GET("/log/self", GetSandboxLogSelf)
	}

	return router
}

func seedSandboxUser(t *testing.T) *model.User {
	t.Helper()

	user := &model.User{
		Username: "sandbox",
		Password: "sandbox-password",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "sandbox",
		Quota:    common.USDToQuota(100),
	}
	require.NoError(t, model.DB.Create(user).Error)
	return user
}

func performSandboxRequest(t *testing.T, router *gin.Engine, method string, target string, body any, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	var requestBody *bytes.Reader
	if body != nil {
		payload, err := common.Marshal(body)
		require.NoError(t, err)
		requestBody = bytes.NewReader(payload)
	} else {
		requestBody = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, target, requestBody)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func decodeSandboxResponse(t *testing.T, recorder *httptest.ResponseRecorder) sandboxAPIResponse {
	t.Helper()

	response := sandboxAPIResponse{}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func createSandboxOrganizationForTest(t *testing.T, router *gin.Engine, organizationID string) sandboxOrganizationResponse {
	t.Helper()

	recorder := performSandboxRequest(t, router, http.MethodPost, "/api/sandbox/organizations", gin.H{
		"organization_id": organizationID,
	}, map[string]string{
		"X-Sandbox-Secret": common.SandboxSecret,
	})
	require.Equal(t, http.StatusOK, recorder.Code)

	response := decodeSandboxResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	data := sandboxOrganizationResponse{}
	require.NoError(t, common.Unmarshal(response.Data, &data))
	return data
}

func createSandboxTokenForTest(t *testing.T, router *gin.Engine, organizationID string, body any) sandboxTokenResponse {
	t.Helper()

	recorder := performSandboxRequest(
		t,
		router,
		http.MethodPost,
		fmt.Sprintf("/api/sandbox/organizations/%s/tokens", organizationID),
		body,
		map[string]string{"X-Sandbox-Secret": common.SandboxSecret},
	)
	require.Equal(t, http.StatusOK, recorder.Code)

	response := decodeSandboxResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	data := sandboxTokenResponse{}
	require.NoError(t, common.Unmarshal(response.Data, &data))
	return data
}

func TestSandboxOrganizationAndTokenCreation(t *testing.T) {
	setupSandboxControllerTest(t)
	router := newSandboxTestRouter()
	sandboxUser := seedSandboxUser(t)

	organization := createSandboxOrganizationForTest(t, router, "org-123456")
	require.True(t, organization.Created)
	require.Equal(t, "org-123456", organization.OrganizationID)
	require.NotZero(t, organization.ID)

	organizationAgain := createSandboxOrganizationForTest(t, router, "org-123456")
	require.False(t, organizationAgain.Created)
	require.Equal(t, organization.ID, organizationAgain.ID)

	defaultToken := createSandboxTokenForTest(t, router, "org-123456", gin.H{})
	require.Equal(t, "org-123456", defaultToken.OrganizationID)
	require.Equal(t, "sandbox", defaultToken.Group)
	require.Equal(t, 5.0, defaultToken.RemainAmountUSD)
	require.Equal(t, int64(-1), defaultToken.ExpiredTime)
	require.True(t, strings.HasPrefix(defaultToken.Name, "org-123456-"))
	require.True(t, strings.HasPrefix(defaultToken.Key, "sk-"))

	tokenModel, err := model.GetTokenById(defaultToken.TokenID)
	require.NoError(t, err)
	require.Equal(t, sandboxUser.Id, tokenModel.UserId)
	require.Equal(t, "sandbox", tokenModel.Group)
	require.Equal(t, common.USDToQuota(5.0), tokenModel.RemainQuota)

	relation, err := model.GetSandboxOrgTokenByTokenID(defaultToken.TokenID)
	require.NoError(t, err)
	require.Equal(t, organization.ID, relation.SandboxOrganizationId)

	expiredTime := time.Now().Add(24 * time.Hour).Unix()
	customToken := createSandboxTokenForTest(t, router, "org-123456", gin.H{
		"remain_amount_usd": 8.5,
		"expired_time":      expiredTime,
	})
	require.NotEqual(t, defaultToken.TokenID, customToken.TokenID)
	require.NotEqual(t, defaultToken.Key, customToken.Key)
	require.Equal(t, 8.5, customToken.RemainAmountUSD)
	require.Equal(t, expiredTime, customToken.ExpiredTime)

	chinaToken := createSandboxTokenForTest(t, router, "org-123456", gin.H{
		"group": "sandbox-China",
	})
	require.Equal(t, "sandbox-China", chinaToken.Group)

	chinaTokenModel, err := model.GetTokenById(chinaToken.TokenID)
	require.NoError(t, err)
	require.Equal(t, "sandbox-China", chinaTokenModel.Group)

	invalidGroupRecorder := performSandboxRequest(t, router, http.MethodPost, "/api/sandbox/organizations/org-123456/tokens", gin.H{
		"group": "sandbox-US",
	}, map[string]string{
		"X-Sandbox-Secret": common.SandboxSecret,
	})
	require.Equal(t, http.StatusOK, invalidGroupRecorder.Code)

	invalidGroupResponse := decodeSandboxResponse(t, invalidGroupRecorder)
	require.False(t, invalidGroupResponse.Success)
	require.Contains(t, invalidGroupResponse.Message, "group 仅支持")

	var relationCount int64
	require.NoError(t, model.DB.Model(&model.SandboxOrgToken{}).
		Where("sandbox_organization_id = ?", organization.ID).
		Count(&relationCount).Error)
	require.Equal(t, int64(3), relationCount)
}

func TestSandboxTokenSelfLifecycleAndLogs(t *testing.T) {
	setupSandboxControllerTest(t)
	router := newSandboxTestRouter()
	sandboxUser := seedSandboxUser(t)

	organization := createSandboxOrganizationForTest(t, router, "org-log-1")
	tokenData := createSandboxTokenForTest(t, router, organization.OrganizationID, gin.H{
		"remain_amount_usd": 2.0,
		"expired_time":      time.Now().Add(2 * time.Hour).Unix(),
	})

	require.NoError(t, model.DB.Model(&model.Token{}).
		Where("id = ?", tokenData.TokenID).
		Updates(map[string]any{
			"status":       common.TokenStatusDisabled,
			"remain_quota": 0,
			"expired_time": common.GetTimestamp() - 60,
		}).Error)

	newExpiredTime := time.Now().Add(48 * time.Hour).Unix()
	updateRecorder := performSandboxRequest(t, router, http.MethodPut, "/api/sandbox/token/self", gin.H{
		"remain_amount_usd": 3.25,
		"expired_time":      newExpiredTime,
	}, map[string]string{
		"Authorization": "Bearer " + tokenData.Key,
	})
	require.Equal(t, http.StatusOK, updateRecorder.Code)

	updateResponse := decodeSandboxResponse(t, updateRecorder)
	require.True(t, updateResponse.Success, updateResponse.Message)

	updateData := sandboxTokenSelfResponse{}
	require.NoError(t, common.Unmarshal(updateResponse.Data, &updateData))
	require.Equal(t, tokenData.TokenID, updateData.TokenID)
	require.Equal(t, common.TokenStatusEnabled, updateData.Status)
	require.Equal(t, 3.25, updateData.RemainAmountUSD)
	require.Equal(t, newExpiredTime, updateData.ExpiredTime)
	require.True(t, updateData.AutoRecovered)

	updatedToken, err := model.GetTokenById(tokenData.TokenID)
	require.NoError(t, err)
	require.Equal(t, common.TokenStatusEnabled, updatedToken.Status)
	require.Equal(t, common.USDToQuota(3.25), updatedToken.RemainQuota)
	require.Equal(t, newExpiredTime, updatedToken.ExpiredTime)
	require.Equal(t, sandboxUser.Id, updatedToken.UserId)

	baseTime := common.GetTimestamp() - 1000
	logs := []model.Log{
		{
			UserId:           sandboxUser.Id,
			Username:         sandboxUser.Username,
			CreatedAt:        baseTime + 100,
			Type:             model.LogTypeConsume,
			TokenId:          tokenData.TokenID,
			TokenName:        updatedToken.Name,
			ModelName:        "gpt-4o-mini",
			Quota:            common.USDToQuota(0.5),
			PromptTokens:     10,
			CompletionTokens: 5,
			RequestId:        "req-1",
			Group:            "sandbox",
		},
		{
			UserId:           sandboxUser.Id,
			Username:         sandboxUser.Username,
			CreatedAt:        baseTime + 200,
			Type:             model.LogTypeConsume,
			TokenId:          tokenData.TokenID,
			TokenName:        updatedToken.Name,
			ModelName:        "gpt-4o",
			Quota:            common.USDToQuota(0.75),
			PromptTokens:     20,
			CompletionTokens: 8,
			RequestId:        "req-2",
			Group:            "sandbox",
		},
		{
			UserId:           sandboxUser.Id,
			Username:         sandboxUser.Username,
			CreatedAt:        baseTime + 300,
			Type:             model.LogTypeConsume,
			TokenId:          tokenData.TokenID,
			TokenName:        updatedToken.Name,
			ModelName:        "claude-3-5-sonnet",
			Quota:            common.USDToQuota(1.25),
			PromptTokens:     30,
			CompletionTokens: 13,
			RequestId:        "req-3",
			Group:            "sandbox",
		},
		{
			UserId:    sandboxUser.Id,
			Username:  sandboxUser.Username,
			CreatedAt: baseTime + 220,
			Type:      model.LogTypeTopup,
			TokenId:   tokenData.TokenID,
			TokenName: updatedToken.Name,
			ModelName: "ignored",
			Quota:     common.USDToQuota(9.99),
			RequestId: "req-ignore",
			Group:     "sandbox",
		},
	}
	require.NoError(t, model.LOG_DB.Create(&logs).Error)

	logRecorder := performSandboxRequest(
		t,
		router,
		http.MethodGet,
		fmt.Sprintf(
			"/api/sandbox/log/self?log_start_timestamp=%d&log_end_timestamp=%d&summary_start_timestamp=%d&summary_end_timestamp=%d&p=1&size=10",
			baseTime+150,
			baseTime+350,
			baseTime+50,
			baseTime+250,
		),
		nil,
		map[string]string{"Authorization": "Bearer " + tokenData.Key},
	)
	require.Equal(t, http.StatusOK, logRecorder.Code)

	logResponse := decodeSandboxResponse(t, logRecorder)
	require.True(t, logResponse.Success, logResponse.Message)

	logData := sandboxLogListResponse{}
	require.NoError(t, common.Unmarshal(logResponse.Data, &logData))
	require.Equal(t, 1, logData.Page)
	require.Equal(t, 10, logData.PageSize)
	require.Equal(t, int64(2), logData.Total)
	require.Len(t, logData.Items, 2)
	require.Equal(t, "req-3", logData.Items[0].RequestID)
	require.Equal(t, "req-2", logData.Items[1].RequestID)
	require.InDelta(t, 1.25, logData.Items[0].AmountUSD, 0.000001)
	require.InDelta(t, 0.75, logData.Items[1].AmountUSD, 0.000001)

	require.NotNil(t, logData.UsageSummary)
	require.Equal(t, baseTime+50, logData.UsageSummary.StartTimestamp)
	require.Equal(t, baseTime+250, logData.UsageSummary.EndTimestamp)
	require.Equal(t, int64(2), logData.UsageSummary.TotalRequests)
	require.Equal(t, int64(30), logData.UsageSummary.TotalPromptTokens)
	require.Equal(t, int64(13), logData.UsageSummary.TotalCompletionTokens)
	require.InDelta(t, 1.25, logData.UsageSummary.TotalAmountUSD, 0.000001)

	deleteRecorder := performSandboxRequest(t, router, http.MethodDelete, "/api/sandbox/token/self", nil, map[string]string{
		"Authorization": "Bearer " + tokenData.Key,
	})
	require.Equal(t, http.StatusOK, deleteRecorder.Code)

	deleteResponse := decodeSandboxResponse(t, deleteRecorder)
	require.True(t, deleteResponse.Success, deleteResponse.Message)

	deleteData := sandboxDeleteResponse{}
	require.NoError(t, common.Unmarshal(deleteResponse.Data, &deleteData))
	require.Equal(t, tokenData.TokenID, deleteData.TokenID)
	require.True(t, deleteData.Deleted)

	deletedToken := model.Token{}
	require.NoError(t, model.DB.Unscoped().First(&deletedToken, tokenData.TokenID).Error)
	require.True(t, deletedToken.DeletedAt.Valid)
}
