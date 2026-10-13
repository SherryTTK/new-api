package relay

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestFixedResponseProtocol(t *testing.T) {
	const content = "固定回复\nhello \"world\""
	usage := &dto.Usage{PromptTokens: 17, CompletionTokens: 9, TotalTokens: 26}
	for _, tc := range []struct {
		name                            string
		format                          types.RelayFormat
		mode                            int
		textPath, inputPath, outputPath string
	}{
		{"chat", types.RelayFormatOpenAI, relayconstant.RelayModeChatCompletions, "choices.0.message.content", "usage.prompt_tokens", "usage.completion_tokens"},
		{"completion", types.RelayFormatOpenAI, relayconstant.RelayModeCompletions, "choices.0.text", "usage.prompt_tokens", "usage.completion_tokens"},
		{"responses", types.RelayFormatOpenAIResponses, relayconstant.RelayModeResponses, "output.0.content.0.text", "usage.input_tokens", "usage.output_tokens"},
		{"claude", types.RelayFormatClaude, 0, "content.0.text", "usage.input_tokens", "usage.output_tokens"},
		{"gemini", types.RelayFormatGemini, 0, "candidates.0.content.parts.0.text", "usageMetadata.promptTokenCount", "usageMetadata.candidatesTokenCount"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("POST", "/?alt=sse", nil)
			info := &relaycommon.RelayInfo{RelayFormat: tc.format, RelayMode: tc.mode, OriginModelName: "test-model", Request: &dto.GeneralOpenAIRequest{StreamOptions: &dto.StreamOptions{IncludeUsage: true}}}
			info.SetEstimatePromptTokens(17)
			data, kind, err := fixedResponsePayload(c, info, content, usage)
			require.NoError(t, err)
			assert.Equal(t, "application/json", kind)
			require.True(t, gjson.ValidBytes(data))
			assert.Equal(t, content, gjson.GetBytes(data, tc.textPath).String())
			assert.EqualValues(t, 17, gjson.GetBytes(data, tc.inputPath).Int())
			assert.EqualValues(t, 9, gjson.GetBytes(data, tc.outputPath).Int())
			info.IsStream = true
			data, kind, err = fixedResponsePayload(c, info, content, usage)
			require.NoError(t, err)
			assert.Equal(t, "text/event-stream", kind)
			var events []gjson.Result
			for _, line := range strings.Split(string(data), "\n") {
				if !strings.HasPrefix(line, "data: ") || line == "data: [DONE]" {
					continue
				}
				raw := strings.TrimPrefix(line, "data: ")
				require.True(t, gjson.Valid(raw), raw)
				events = append(events, gjson.Parse(raw))
			}
			require.NotEmpty(t, events)
			last := events[len(events)-1]
			switch tc.format {
			case types.RelayFormatOpenAI:
				assert.EqualValues(t, 17, last.Get("usage.prompt_tokens").Int())
				assert.EqualValues(t, 9, last.Get("usage.completion_tokens").Int())
				assert.True(t, strings.HasSuffix(string(data), "data: [DONE]\n\n"))
				path := "choices.0.delta.content"
				if tc.mode == relayconstant.RelayModeCompletions {
					path = "choices.0.text"
				}
				assert.Equal(t, content, events[0].Get(path).String())
			case types.RelayFormatOpenAIResponses:
				assert.Equal(t, "response.completed", last.Get("type").String())
				assert.EqualValues(t, 17, last.Get("response.usage.input_tokens").Int())
				assert.EqualValues(t, 9, last.Get("response.usage.output_tokens").Int())
				assert.Equal(t, content, last.Get("response.output.0.content.0.text").String())
			case types.RelayFormatClaude:
				assert.Equal(t, "message_start", events[0].Get("type").String())
				assert.EqualValues(t, 17, events[0].Get("message.usage.input_tokens").Int())
				assert.Equal(t, "message_stop", last.Get("type").String())
				var text string
				var output int64
				for _, event := range events {
					text += event.Get("delta.text").String()
					if event.Get("type").String() == "message_delta" {
						output = event.Get("usage.output_tokens").Int()
					}
				}
				assert.Equal(t, content, text)
				assert.EqualValues(t, 9, output)
			case types.RelayFormatGemini:
				assert.Equal(t, content, last.Get(tc.textPath).String())
				assert.EqualValues(t, 17, last.Get(tc.inputPath).Int())
				assert.EqualValues(t, 9, last.Get(tc.outputPath).Int())
			}
		})
	}
}

func TestFixedResponseStreamUsageOptOut(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	info := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatOpenAI, IsStream: true, Request: &dto.GeneralOpenAIRequest{}}
	payload, _, err := fixedResponsePayload(c, info, "hello", &dto.Usage{PromptTokens: 3, CompletionTokens: 1, TotalTokens: 4})
	require.NoError(t, err)
	assert.NotContains(t, string(payload), "prompt_tokens")
	assert.Contains(t, string(payload), "data: [DONE]")
	// Also verify that the native Gemini JSON stream is an array, not SSE.
	info.RelayFormat = types.RelayFormatGemini
	payload, kind, err := fixedResponsePayload(c, info, "hello", &dto.Usage{PromptTokens: 3, CompletionTokens: 1, TotalTokens: 4})
	require.NoError(t, err)
	assert.Equal(t, "application/json", kind)
	var responses []dto.GeminiChatResponse
	require.NoError(t, common.Unmarshal(payload, &responses))
	require.Len(t, responses, 1)
	assert.Equal(t, 4, responses[0].UsageMetadata.TotalTokenCount)
}

func TestFixedResponseCacheUsageProtocols(t *testing.T) {
	for _, writeTokens := range []int{0, 200} {
		for _, format := range []types.RelayFormat{types.RelayFormatOpenAI, types.RelayFormatOpenAIResponses, types.RelayFormatClaude, types.RelayFormatGemini} {
			for _, stream := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/write=%d/stream=%t", format, writeTokens, stream), func(t *testing.T) {
					usage := &dto.Usage{PromptTokens: 117 + writeTokens, CompletionTokens: 9, TotalTokens: 126 + writeTokens, UsageSemantic: dto.BillingUsageSemanticOpenAI, PromptTokensDetails: dto.InputTokenDetails{CachedTokens: 100, CacheWriteTokens: writeTokens}}
					c, _ := gin.CreateTestContext(httptest.NewRecorder())
					c.Request = httptest.NewRequest("POST", "/?alt=sse", nil)
					info := &relaycommon.RelayInfo{RelayFormat: format, IsStream: stream, OriginModelName: "test-model", Request: &dto.GeneralOpenAIRequest{StreamOptions: &dto.StreamOptions{IncludeUsage: true}}}
					info.SetEstimatePromptTokens(17)
					data, _, err := fixedResponsePayload(c, info, "hello", usage)
					require.NoError(t, err)
					result := gjson.ParseBytes(data)
					if stream {
						for _, line := range strings.Split(string(data), "\n") {
							if !strings.HasPrefix(line, "data: ") || line == "data: [DONE]" {
								continue
							}
							event := gjson.Parse(strings.TrimPrefix(line, "data: "))
							if format == types.RelayFormatClaude && event.Get("type").String() != "message_delta" {
								continue
							}
							result = event
						}
					}
					switch format {
					case types.RelayFormatOpenAI:
						assert.EqualValues(t, 100, result.Get("usage.prompt_tokens_details.cached_tokens").Int())
						assert.EqualValues(t, writeTokens, result.Get("usage.prompt_tokens_details.cache_write_tokens").Int())
						assert.EqualValues(t, 117+writeTokens, result.Get("usage.prompt_tokens").Int())
					case types.RelayFormatOpenAIResponses:
						if stream {
							result = result.Get("response")
						}
						assert.EqualValues(t, 100, result.Get("usage.input_tokens_details.cached_tokens").Int())
						assert.EqualValues(t, writeTokens, result.Get("usage.input_tokens_details.cache_write_tokens").Int())
					case types.RelayFormatClaude:
						assert.EqualValues(t, 100, result.Get("usage.cache_read_input_tokens").Int())
						assert.EqualValues(t, writeTokens, result.Get("usage.cache_creation_input_tokens").Int())
						assert.EqualValues(t, 17, result.Get("usage.input_tokens").Int())
					case types.RelayFormatGemini:
						assert.EqualValues(t, 100, result.Get("usageMetadata.cachedContentTokenCount").Int())
						assert.EqualValues(t, 117+writeTokens, result.Get("usageMetadata.promptTokenCount").Int())
					}
				})
			}
		}
	}
}
