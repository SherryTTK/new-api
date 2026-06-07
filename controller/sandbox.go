package controller

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	sandboxUserName   = "sandbox"
	sandboxTokenGroup = "sandbox"
	maxAmountUSD      = 1000000000
	maxTokenNameLen   = 50
)

type sandboxOrganizationCreateRequest struct {
	OrganizationID string `json:"organization_id"`
}

type sandboxTokenCreateRequest struct {
	RemainAmountUSD *float64 `json:"remain_amount_usd"`
	ExpiredTime     *int64   `json:"expired_time"`
}

type sandboxTokenUpdateRequest struct {
	RemainAmountUSD *float64 `json:"remain_amount_usd"`
	ExpiredTime     *int64   `json:"expired_time"`
}

func bindSandboxJSON(c *gin.Context, target any) error {
	if c.Request == nil || c.Request.Body == nil {
		return nil
	}
	if err := c.ShouldBindJSON(target); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	return nil
}

func parseTimestampQuery(c *gin.Context, name string) (int64, error) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s 格式错误", name)
	}
	if value < 0 {
		return 0, fmt.Errorf("%s 不能小于 0", name)
	}
	return value, nil
}

func validateAmountUSD(amount float64) error {
	if amount < 0 {
		return errors.New("remain_amount_usd 不能小于 0")
	}
	if amount > maxAmountUSD {
		return fmt.Errorf("remain_amount_usd 不能超过 %d", maxAmountUSD)
	}
	return nil
}

func validateSandboxExpiredTime(expiredTime int64) error {
	if expiredTime == -1 {
		return nil
	}
	if expiredTime <= common.GetTimestamp() {
		return errors.New("expired_time 必须大于当前时间")
	}
	return nil
}

func sandboxTokenName(organizationID string, now time.Time) string {
	timestamp := now.In(time.Local).Format("20060102150405")
	name := organizationID + "-" + timestamp
	if len(name) <= maxTokenNameLen {
		return name
	}
	maxOrgLen := maxTokenNameLen - len(timestamp) - 1
	if maxOrgLen < 1 {
		maxOrgLen = 1
	}
	return organizationID[:maxOrgLen] + "-" + timestamp
}

func getSandboxUser() (*model.User, error) {
	user, err := model.GetUserByUsername(sandboxUserName, true)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("sandbox 用户不存在")
		}
		return nil, err
	}
	if user.Status != common.UserStatusEnabled {
		return nil, errors.New("sandbox 用户已被禁用")
	}
	if !service.GroupInUserUsableGroups(user.Group, sandboxTokenGroup) {
		return nil, fmt.Errorf("sandbox 用户无权使用 %s 分组", sandboxTokenGroup)
	}
	if !ratio_setting.ContainsGroupRatio(sandboxTokenGroup) {
		return nil, fmt.Errorf("分组 %s 不存在", sandboxTokenGroup)
	}
	return user, nil
}

func getCurrentSandboxToken(c *gin.Context) (*model.Token, error) {
	tokenID := c.GetInt("token_id")
	if tokenID == 0 {
		return nil, errors.New("无效的令牌")
	}
	isSandboxToken, err := model.IsSandboxToken(tokenID)
	if err != nil {
		return nil, err
	}
	if !isSandboxToken {
		return nil, errors.New("当前 key 不是沙盒 key")
	}
	token, err := model.GetTokenById(tokenID)
	if err != nil {
		return nil, err
	}
	return token, nil
}

func buildSandboxLogResponse(log *model.Log) gin.H {
	return gin.H{
		"request_id":            log.RequestId,
		"created_at":            time.Unix(log.CreatedAt, 0).In(time.Local).Format("2006-01-02 15:04:05"),
		"username":              log.Username,
		"token_name":            log.TokenName,
		"token_id":              log.TokenId,
		"model_name":            log.ModelName,
		"prompt_tokens":         log.PromptTokens,
		"completion_tokens":     log.CompletionTokens,
		"cache_creation_tokens": 0,
		"cache_read_tokens":     0,
		"amount_usd":            common.QuotaToUSD(int64(log.Quota)),
		"use_time":              log.UseTime,
		"is_stream":             log.IsStream,
		"group":                 log.Group,
		"content":               log.Content,
	}
}

func CreateSandboxOrganization(c *gin.Context) {
	req := sandboxOrganizationCreateRequest{}
	if err := bindSandboxJSON(c, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	req.OrganizationID = strings.TrimSpace(req.OrganizationID)
	if req.OrganizationID == "" {
		common.ApiErrorMsg(c, "organization_id 不能为空")
		return
	}
	organization, created, err := model.CreateOrGetSandboxOrganization(req.OrganizationID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"id":              organization.Id,
		"organization_id": organization.OrganizationID,
		"created":         created,
		"created_time":    organization.CreatedTime,
		"updated_time":    organization.UpdatedTime,
	})
}

func CreateSandboxToken(c *gin.Context) {
	organizationID := strings.TrimSpace(c.Param("organization_id"))
	if organizationID == "" {
		common.ApiErrorMsg(c, "organization_id 不能为空")
		return
	}

	req := sandboxTokenCreateRequest{}
	if err := bindSandboxJSON(c, &req); err != nil {
		common.ApiError(c, err)
		return
	}

	remainAmountUSD := 5.0
	if req.RemainAmountUSD != nil {
		remainAmountUSD = *req.RemainAmountUSD
	}
	if err := validateAmountUSD(remainAmountUSD); err != nil {
		common.ApiError(c, err)
		return
	}

	expiredTime := int64(-1)
	if req.ExpiredTime != nil {
		expiredTime = *req.ExpiredTime
	}
	if err := validateSandboxExpiredTime(expiredTime); err != nil {
		common.ApiError(c, err)
		return
	}

	organization, err := model.GetSandboxOrganizationByOrganizationID(organizationID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.ApiErrorMsg(c, "organization_id 不存在")
			return
		}
		common.ApiError(c, err)
		return
	}

	sandboxUser, err := getSandboxUser()
	if err != nil {
		common.ApiError(c, err)
		return
	}

	count, err := model.CountUserTokens(sandboxUser.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	maxTokens := operation_setting.GetMaxUserTokens()
	if int(count) >= maxTokens {
		common.ApiErrorMsg(c, fmt.Sprintf("sandbox 用户令牌数量已达到上限 (%d)", maxTokens))
		return
	}

	internalQuota := common.USDToQuota(remainAmountUSD)
	if sandboxUser.Quota < internalQuota {
		common.ApiErrorMsg(c, "sandbox 用户余额不足")
		return
	}

	key, err := common.GenerateKey()
	if err != nil {
		common.ApiError(c, err)
		return
	}

	now := common.GetTimestamp()
	token := model.Token{
		UserId:             sandboxUser.Id,
		Key:                key,
		Status:             common.TokenStatusEnabled,
		Name:               sandboxTokenName(organizationID, time.Unix(now, 0)),
		CreatedTime:        now,
		AccessedTime:       now,
		ExpiredTime:        expiredTime,
		RemainQuota:        internalQuota,
		UnlimitedQuota:     false,
		ModelLimitsEnabled: false,
		ModelLimits:        "",
		AllowIps:           nil,
		UsedQuota:          0,
		Group:              sandboxTokenGroup,
		CrossGroupRetry:    false,
	}
	relation := model.SandboxOrgToken{
		SandboxOrganizationId: organization.Id,
	}

	tx := model.DB.Begin()
	if tx.Error != nil {
		common.ApiError(c, tx.Error)
		return
	}
	if err := tx.Create(&token).Error; err != nil {
		tx.Rollback()
		common.ApiError(c, err)
		return
	}
	relation.TokenId = token.Id
	relation.CreatedTime = now
	if err := tx.Create(&relation).Error; err != nil {
		tx.Rollback()
		common.ApiError(c, err)
		return
	}
	if err := tx.Commit().Error; err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, gin.H{
		"organization_id":         organization.OrganizationID,
		"sandbox_organization_id": organization.Id,
		"relation_id":             relation.Id,
		"token_id":                token.Id,
		"name":                    token.Name,
		"key":                     "sk-" + token.Key,
		"group":                   token.Group,
		"remain_amount_usd":       common.QuotaToUSD(int64(token.RemainQuota)),
		"expired_time":            token.ExpiredTime,
		"created_time":            token.CreatedTime,
	})
}

func UpdateSandboxTokenSelf(c *gin.Context) {
	req := sandboxTokenUpdateRequest{}
	if err := bindSandboxJSON(c, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.RemainAmountUSD == nil || req.ExpiredTime == nil {
		common.ApiErrorMsg(c, "remain_amount_usd 和 expired_time 为必填参数")
		return
	}
	if err := validateAmountUSD(*req.RemainAmountUSD); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := validateSandboxExpiredTime(*req.ExpiredTime); err != nil {
		common.ApiError(c, err)
		return
	}

	token, err := getCurrentSandboxToken(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	token.Status = common.TokenStatusEnabled
	token.RemainQuota = common.USDToQuota(*req.RemainAmountUSD)
	token.ExpiredTime = *req.ExpiredTime
	token.UnlimitedQuota = false
	if err := token.Update(); err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, gin.H{
		"token_id":          token.Id,
		"status":            token.Status,
		"remain_amount_usd": common.QuotaToUSD(int64(token.RemainQuota)),
		"expired_time":      token.ExpiredTime,
		"auto_recovered":    true,
	})
}

func DeleteSandboxTokenSelf(c *gin.Context) {
	token, err := getCurrentSandboxToken(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.DeleteTokenById(token.Id, token.UserId); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"token_id": token.Id,
		"deleted":  true,
	})
}

func GetSandboxLogSelf(c *gin.Context) {
	token, err := getCurrentSandboxToken(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	logStartTimestamp, err := parseTimestampQuery(c, "log_start_timestamp")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	logEndTimestamp, err := parseTimestampQuery(c, "log_end_timestamp")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if logStartTimestamp != 0 && logEndTimestamp != 0 && logStartTimestamp > logEndTimestamp {
		common.ApiErrorMsg(c, "log_start_timestamp 不能大于 log_end_timestamp")
		return
	}

	summaryStartTimestamp, err := parseTimestampQuery(c, "summary_start_timestamp")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	summaryEndTimestamp, err := parseTimestampQuery(c, "summary_end_timestamp")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if (summaryStartTimestamp == 0) != (summaryEndTimestamp == 0) {
		common.ApiErrorMsg(c, "summary_start_timestamp 和 summary_end_timestamp 必须成对出现")
		return
	}
	if summaryStartTimestamp != 0 && summaryEndTimestamp != 0 && summaryStartTimestamp > summaryEndTimestamp {
		common.ApiErrorMsg(c, "summary_start_timestamp 不能大于 summary_end_timestamp")
		return
	}

	pageInfo := common.GetPageQuery(c)
	logs, total, err := model.GetSandboxLogsByTokenID(token.Id, logStartTimestamp, logEndTimestamp, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}

	items := make([]gin.H, 0, len(logs))
	for _, log := range logs {
		items = append(items, buildSandboxLogResponse(log))
	}

	data := gin.H{
		"page":      pageInfo.GetPage(),
		"page_size": pageInfo.GetPageSize(),
		"total":     total,
		"items":     items,
	}
	if summaryStartTimestamp != 0 && summaryEndTimestamp != 0 {
		summary, err := model.GetSandboxLogSummaryByTokenID(token.Id, summaryStartTimestamp, summaryEndTimestamp)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		data["usage_summary"] = gin.H{
			"start_timestamp":         summaryStartTimestamp,
			"end_timestamp":           summaryEndTimestamp,
			"total_requests":          summary.TotalRequests,
			"total_prompt_tokens":     summary.TotalPromptTokens,
			"total_completion_tokens": summary.TotalCompletionTokens,
			"total_amount_usd":        common.QuotaToUSD(summary.TotalQuota),
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    data,
	})
}
