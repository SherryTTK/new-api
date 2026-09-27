package relay

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/relayconvert"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

func FixedResponseHelper(c *gin.Context, info *relaycommon.RelayInfo, config types.FixedResponseConfig) *types.NewAPIError {
	if err := config.Validate(); err != nil {
		return types.NewError(err, types.ErrorCodeInvalidRequest)
	}
	info.FixedResponse = true
	info.ForcePreConsume = true
	info.InitChannelMeta(c)
	meta := info.Request.GetTokenCountMeta()
	if setting.ShouldCheckPromptSensitive() {
		if contains, _ := service.CheckSensitiveText(meta.CombineText); contains {
			return types.NewError(fmt.Errorf("prompt contains sensitive words"), types.ErrorCodeSensitiveWordsDetected)
		}
	}
	promptTokens, err := service.EstimateRequestToken(c, meta, info)
	if err != nil {
		return types.NewError(err, types.ErrorCodeCountTokenFailed)
	}
	outputTokens := service.CountTextToken(config.Content, info.OriginModelName)
	usage := &dto.Usage{PromptTokens: promptTokens, CompletionTokens: outputTokens, TotalTokens: promptTokens + outputTokens}
	info.SetEstimatePromptTokens(promptTokens)
	common.SetContextKey(c, constant.ContextKeyLocalCountTokens, true)
	// The entire output is known. Reserve its actual cost, irrespective of client max_tokens.
	meta.MaxTokens = outputTokens
	if _, err := helper.ModelPriceHelper(c, info, promptTokens, meta); err != nil {
		return types.NewError(err, types.ErrorCodeModelPriceError, types.ErrOptionWithStatusCode(http.StatusBadRequest))
	}
	quota := service.FixedResponseQuota(c, info, usage)
	payload, contentType, err := fixedResponsePayload(c, info, config.Content, usage)
	if err != nil {
		return types.NewError(err, types.ErrorCodeInvalidRequest)
	}
	if !info.PriceData.FreeModel {
		if apiErr := service.PreConsumeBilling(c, quota, info); apiErr != nil {
			return apiErr
		}
	}
	settled := false
	defer func() {
		if !settled && info.Billing != nil {
			info.Billing.Refund(c)
		}
	}()
	delay := config.MinDelayMS
	if config.MaxDelayMS > delay {
		delay += rand.IntN(config.MaxDelayMS - delay + 1)
	}
	timer := time.NewTimer(time.Duration(delay) * time.Millisecond)
	defer timer.Stop()
	select {
	case <-c.Request.Context().Done():
		return nil
	case <-timer.C:
	}
	if c.Request.Context().Err() != nil {
		return nil
	}
	info.SetFirstResponseTime()
	if info.IsStream && contentType == "text/event-stream" {
		helper.SetEventStreamHeaders(c)
	}
	c.Header("Content-Type", contentType)
	n, err := c.Writer.Write(payload)
	if err != nil || n != len(payload) {
		logger.LogWarn(c, fmt.Sprintf("fixed response write failed: %v (%d/%d bytes)", err, n, len(payload)))
		return nil
	}
	if info.IsStream {
		c.Writer.Flush()
	}
	service.PostTextConsumeQuota(c, info, usage, []string{"固定回复（本地 Token 统计）"})
	settled = true
	return nil
}

// Build the complete protocol envelope before charging or committing HTTP headers.
func fixedResponsePayload(c *gin.Context, info *relaycommon.RelayInfo, content string, usage *dto.Usage) ([]byte, string, error) {
	id := "chatcmpl-" + common.GetUUID()
	created := time.Now().Unix()
	response := &dto.OpenAITextResponse{
		Id: id, Object: "chat.completion", Created: created, Model: info.OriginModelName, Usage: *usage,
		Choices: []dto.OpenAITextResponseChoice{{Index: 0, Message: dto.Message{Role: "assistant", Content: content}, FinishReason: "stop"}},
	}
	legacy := info.RelayMode == relayconstant.RelayModeCompletions
	if !info.IsStream {
		var value any
		if legacy {
			value = gin.H{"id": id, "object": "text_completion", "created": created, "model": info.OriginModelName, "choices": []gin.H{{"index": 0, "text": content, "finish_reason": "stop", "logprobs": nil}}, "usage": usage}
		} else {
			result, err := relayconvert.ConvertResponse(c, info, info.RelayFormat, response)
			if err != nil {
				return nil, "", err
			}
			value = result.Value
		}
		data, err := common.Marshal(value)
		return data, "application/json", err
	}
	// Gemini also supports a streamed JSON array when alt=sse is absent.
	if info.RelayFormat == types.RelayFormatGemini {
		result, err := relayconvert.ConvertResponse(c, info, info.RelayFormat, response)
		if err != nil {
			return nil, "", err
		}
		if c.Query("alt") != "sse" {
			data, err := common.Marshal([]any{result.Value})
			return data, "application/json", err
		}
		data, err := common.Marshal(result.Value)
		return append(append([]byte("data: "), data...), '\n', '\n'), "text/event-stream", err
	}
	includeUsage := true
	if req, ok := info.Request.(*dto.GeneralOpenAIRequest); ok {
		includeUsage = req.StreamOptions != nil && req.StreamOptions.IncludeUsage
	}
	chunks := []*dto.ChatCompletionsStreamResponse{
		{Id: id, Object: "chat.completion.chunk", Created: created, Model: info.OriginModelName, Choices: []dto.ChatCompletionsStreamResponseChoice{{Index: 0, Delta: dto.ChatCompletionsStreamResponseChoiceDelta{Role: "assistant", Content: &content}}}},
		{Id: id, Object: "chat.completion.chunk", Created: created, Model: info.OriginModelName, Choices: []dto.ChatCompletionsStreamResponseChoice{{Index: 0, FinishReason: common.GetPointer("stop")}}, Usage: usage},
	}
	state, err := relayconvert.NewResponseStreamState(types.RelayFormatOpenAI, info.RelayFormat, relayconvert.ResponseStreamOptions{ID: id, Model: info.OriginModelName, Created: created, IncludeUsage: includeUsage})
	if err != nil {
		return nil, "", err
	}
	var values []any
	for _, chunk := range chunks {
		if info.RelayFormat == types.RelayFormatOpenAI {
			chunk.Usage = nil
			if legacy {
				text := chunk.Choices[0].Delta.GetContentString()
				values = append(values, gin.H{"id": id, "object": "text_completion", "created": created, "model": info.OriginModelName, "choices": []gin.H{{"index": 0, "text": text, "finish_reason": chunk.Choices[0].FinishReason, "logprobs": nil}}})
			} else {
				values = append(values, chunk)
			}
			continue
		}
		results, err := relayconvert.ConvertStreamResponseChunk(c, info, state, chunk)
		if err != nil {
			return nil, "", err
		}
		for _, result := range results {
			values = append(values, result.Value)
		}
	}
	results, err := relayconvert.FinalizeStreamResponse(c, info, state)
	if err != nil {
		return nil, "", err
	}
	for _, result := range results {
		values = append(values, result.Value)
	}
	if info.RelayFormat == types.RelayFormatOpenAI && includeUsage {
		object := "chat.completion.chunk"
		if legacy {
			object = "text_completion"
		}
		values = append(values, gin.H{"id": id, "object": object, "created": created, "model": info.OriginModelName, "choices": []any{}, "usage": usage})
	}
	var payload bytes.Buffer
	for _, value := range values {
		if event, ok := value.(relayconvert.ChatToResponsesStreamEvent); ok {
			value = event.Payload
		}
		data, err := common.Marshal(value)
		if err != nil {
			return nil, "", err
		}
		if info.RelayFormat == types.RelayFormatClaude || info.RelayFormat == types.RelayFormatOpenAIResponses {
			fmt.Fprintf(&payload, "event: %s\n", gjson.GetBytes(data, "type").String())
		}
		payload.WriteString("data: ")
		payload.Write(data)
		payload.WriteString("\n\n")
	}
	if info.RelayFormat == types.RelayFormatOpenAI {
		payload.WriteString("data: [DONE]\n\n")
	}
	return payload.Bytes(), "text/event-stream", nil
}
