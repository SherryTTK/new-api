package service

import (
	"bytes"
	"fmt"
	"html"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const exportCNYExchangeRate = 7.2

func parseCacheTokensFromOther(other string) (cacheCreation int, cacheRead int) {
	if other == "" {
		return 0, 0
	}
	var data map[string]interface{}
	if err := common.UnmarshalJsonStr(other, &data); err != nil {
		return 0, 0
	}
	if v, ok := data["cache_creation_tokens"]; ok {
		if n, ok := v.(float64); ok {
			cacheCreation = int(n)
		}
	}
	if v, ok := data["cache_tokens"]; ok {
		if n, ok := v.(float64); ok {
			cacheRead = int(n)
		}
	}
	return
}

func quotaToAmount(quota int64) float64 {
	if common.QuotaPerUnit == 0 {
		return 0
	}
	return float64(quota) / common.QuotaPerUnit
}

func quotaToCNYAmount(quota int64) float64 {
	return quotaToAmount(quota) * exportCNYExchangeRate
}

func formatAmount(quota int64) string {
	return strconv.FormatFloat(quotaToAmount(quota), 'f', 6, 64)
}

func formatCNYAmount(quota int64) string {
	return strconv.FormatFloat(quotaToCNYAmount(quota), 'f', 6, 64)
}

func formatExportTimestamp(timestamp int64) string {
	if timestamp <= 0 {
		return ""
	}
	return time.Unix(timestamp, 0).In(time.Local).Format("2006-01-02 15:04:05")
}

func buildExportFileName(prefix string, startTimestamp int64, endTimestamp int64) string {
	timePart := time.Now().In(time.Local).Format("20060102_150405")
	if startTimestamp > 0 && endTimestamp > 0 {
		start := time.Unix(startTimestamp, 0).In(time.Local).Format("20060102_150405")
		end := time.Unix(endTimestamp, 0).In(time.Local).Format("20060102_150405")
		timePart = fmt.Sprintf("%s_%s", start, end)
	}
	return fmt.Sprintf("%s-%s.xls", prefix, timePart)
}

func buildExcelHTML(headers []string, rows [][]string) []byte {
	var buffer bytes.Buffer
	buffer.WriteString("<html><head><meta charset=\"utf-8\"></head><body>")
	buffer.WriteString("<table border=\"1\" cellspacing=\"0\" cellpadding=\"6\">")
	buffer.WriteString("<tr style=\"font-weight:bold;background:#E8F3FF;\">")
	for _, header := range headers {
		buffer.WriteString("<th>")
		buffer.WriteString(html.EscapeString(header))
		buffer.WriteString("</th>")
	}
	buffer.WriteString("</tr>")
	for _, row := range rows {
		buffer.WriteString("<tr>")
		for _, value := range row {
			buffer.WriteString("<td>")
			buffer.WriteString(html.EscapeString(value))
			buffer.WriteString("</td>")
		}
		buffer.WriteString("</tr>")
	}
	buffer.WriteString("</table></body></html>")
	return buffer.Bytes()
}

func BuildLogSummaryExport(startTimestamp int64, endTimestamp int64, userId int, username string) ([]byte, string, error) {
	rows, err := model.GetLogSummaryExportRows(startTimestamp, endTimestamp, userId, username)
	if err != nil {
		return nil, "", err
	}

	headers := []string{
		"Key ID",
		"Key名称",
		"Key",
		"令牌剩余额度",
		"令牌总额度(USD)",
		"令牌剩余额度(USD)",
		"总调用次数",
		"总消耗Token数",
		"总消耗额度",
		"总花费金额(USD)",
	}
	tableRows := make([][]string, 0, len(rows))
	for _, row := range rows {
		tokenRemainStr := "无限"
		tokenTotalUSDStr := "无限"
		tokenRemainUSDStr := "无限"
		if !row.UnlimitedQuota {
			tokenRemainStr = strconv.Itoa(row.RemainQuota)
			totalQuota := row.RemainQuota + row.UsedQuota
			tokenTotalUSDStr = strconv.FormatFloat(float64(totalQuota)/500000, 'f', 6, 64)
			tokenRemainUSDStr = strconv.FormatFloat(float64(row.RemainQuota)/500000, 'f', 6, 64)
		}
		tableRows = append(tableRows, []string{
			strconv.Itoa(row.TokenID),
			row.TokenName,
			row.MaskedKey,
			tokenRemainStr,
			tokenTotalUSDStr,
			tokenRemainUSDStr,
			strconv.FormatInt(row.TotalCalls, 10),
			strconv.FormatInt(row.TotalTokens, 10),
			strconv.FormatInt(row.TotalQuota, 10),
			formatAmount(row.TotalQuota),
		})
	}

	return buildExcelHTML(headers, tableRows), buildExportFileName("usage-log-summary", startTimestamp, endTimestamp), nil
}

func BuildLogDetailExport(startTimestamp int64, endTimestamp int64, tokenName string, userId int, username string) ([]byte, string, error) {
	logs, err := model.GetLogsForDetailExport(startTimestamp, endTimestamp, tokenName, userId, username)
	if err != nil {
		return nil, "", err
	}

	headers := []string{
		"时间",
		"用户名称",
		"Key名称",
		"Key ID",
		"模型名称",
		"Prompt Tokens",
		"Completion Tokens",
		"总 Tokens",
		"缓存创建Tokens",
		"缓存读取Tokens",
		"消耗额度",
		"花费金额(USD)",
		"分组",
		"渠道 ID",
		"渠道名称",
		"Request ID",
		"内容",
	}
	tableRows := make([][]string, 0, len(logs))
	for _, log := range logs {
		totalTokens := log.PromptTokens + log.CompletionTokens
		cacheCreationTokens, cacheReadTokens := parseCacheTokensFromOther(log.Other)
		tableRows = append(tableRows, []string{
			formatExportTimestamp(log.CreatedAt),
			log.Username,
			log.TokenName,
			strconv.Itoa(log.TokenId),
			log.ModelName,
			strconv.Itoa(log.PromptTokens),
			strconv.Itoa(log.CompletionTokens),
			strconv.Itoa(totalTokens),
			strconv.Itoa(cacheCreationTokens),
			strconv.Itoa(cacheReadTokens),
			strconv.Itoa(log.Quota),
			formatAmount(int64(log.Quota)),
			log.Group,
			strconv.Itoa(log.ChannelId),
			log.ChannelName,
			log.RequestId,
			log.Content,
		})
	}

	return buildExcelHTML(headers, tableRows), buildExportFileName("usage-log-detail", startTimestamp, endTimestamp), nil
}

func BuildLogRequestIdResponse(log *model.Log) map[string]interface{} {
	cacheCreationTokens, cacheReadTokens := parseCacheTokensFromOther(log.Other)
	return map[string]interface{}{
		"request_id":            log.RequestId,
		"created_at":            log.CreatedAt,
		"username":              log.Username,
		"token_name":            log.TokenName,
		"token_id":              log.TokenId,
		"model_name":            log.ModelName,
		"prompt_tokens":         log.PromptTokens,
		"completion_tokens":     log.CompletionTokens,
		"cache_creation_tokens": cacheCreationTokens,
		"cache_read_tokens":     cacheReadTokens,
		"quota":                 log.Quota,
		"use_time":              log.UseTime,
		"is_stream":             log.IsStream,
		"group":                 log.Group,
		"channel_id":            log.ChannelId,
		"content":               log.Content,
	}
}
