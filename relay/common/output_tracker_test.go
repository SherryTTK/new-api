package common

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHasValidRelayOutput(t *testing.T) {
	tests := []struct {
		name  string
		body  string
		valid bool
	}{
		{
			name:  "chat reasoning only",
			body:  `{"choices":[{"message":{"role":"assistant","reasoning_content":"thinking"}}],"usage":{"completion_tokens":4}}`,
			valid: true,
		},
		{
			name:  "chat tool call only",
			body:  `{"choices":[{"message":{"role":"assistant","tool_calls":[{"type":"function","function":{"name":"lookup","arguments":"{}"}}]}}]}`,
			valid: true,
		},
		{
			name:  "chat refusal only",
			body:  `{"choices":[{"message":{"role":"assistant","content":null,"refusal":"I cannot help with that."}}]}`,
			valid: true,
		},
		{
			name:  "responses reasoning item",
			body:  `{"output":[{"type":"reasoning","id":"r_1","summary":[]}]}`,
			valid: true,
		},
		{
			name:  "responses compaction item",
			body:  `{"output":[{"type":"compaction","encrypted_content":"opaque"}]}`,
			valid: true,
		},
		{
			name:  "responses stream function item",
			body:  `{"type":"response.output_item.added","item":{"type":"function_call","name":"lookup","arguments":"{}"}}`,
			valid: true,
		},
		{
			name:  "responses stream refusal",
			body:  `{"type":"response.refusal.delta","delta":"I cannot help with that."}`,
			valid: true,
		},
		{
			name:  "claude tool use",
			body:  `{"type":"message","content":[{"type":"tool_use","id":"tool_1","name":"lookup","input":{}}]}`,
			valid: true,
		},
		{
			name:  "claude redacted thinking",
			body:  `{"type":"message","content":[{"type":"redacted_thinking","data":"opaque"}]}`,
			valid: true,
		},
		{
			name:  "gemini thought text",
			body:  `{"candidates":[{"content":{"parts":[{"text":"thinking","thought":true}]}}]}`,
			valid: true,
		},
		{
			name:  "gemini thought signature only",
			body:  `{"candidates":[{"content":{"parts":[{"thought":true,"thoughtSignature":"opaque"}]}}]}`,
			valid: true,
		},
		{
			name:  "role only",
			body:  `{"choices":[{"delta":{"role":"assistant"}}]}`,
			valid: false,
		},
		{
			name:  "usage only",
			body:  `{"choices":[],"usage":{"prompt_tokens":10,"completion_tokens":3,"total_tokens":13}}`,
			valid: false,
		},
		{
			name:  "empty gemini candidates",
			body:  `{"candidates":[],"usageMetadata":{"totalTokenCount":10}}`,
			valid: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.valid, HasValidRelayOutput([]byte(test.body)))
		})
	}
}

func TestWrapOutputTrackingWriterClearsPartialRetryOutput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &RelayInfo{}
	WrapOutputTrackingWriter(context, info)

	_, err := context.Writer.Write([]byte(`{"choices":[{"message":{"content":`))
	require.NoError(t, err)
	assert.False(t, info.HasValidOutput())

	info.ResetValidOutput()
	WrapOutputTrackingWriter(context, info)
	_, err = context.Writer.Write([]byte(`"stale"}}]}`))
	require.NoError(t, err)
	assert.False(t, info.HasValidOutput())
}
