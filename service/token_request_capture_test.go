package service

import (
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLatestUserQuestion(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"chat", `{"messages":[{"role":"system","content":"secret instructions"},{"role":"user","content":"old"},{"role":"assistant","content":"old answer"},{"role":"user","content":[{"type":"text","text":"new question"},{"type":"image_url","image_url":{"url":"secret"}}]}]}`, "new question"},
		{"claude", `{"messages":[{"role":"user","content":[{"type":"text","text":"你好"},{"type":"text","text":"世界"}]}]}`, "你好\n世界"},
		{"responses string", `{"input":"question"}`, "question"},
		{"responses array", `{"input":[{"role":"user","content":[{"type":"input_text","text":"question"}]}]}`, "question"},
		{"gemini", `{"contents":[{"role":"model","parts":[{"text":"history"}]},{"role":"user","parts":[{"text":"question"}]}]}`, "question"},
		{"completion", `{"prompt":"question"}`, "question"},
		{"image only", `{"messages":[{"role":"user","content":"old"},{"role":"user","content":[{"type":"image_url","image_url":{"url":"private"}}]}]}`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, truncated := latestUserQuestion([]byte(tc.body))
			assert.Equal(t, tc.want, got)
			assert.False(t, truncated)
		})
	}
	question, truncated := latestUserQuestion([]byte(`{"input":"` + strings.Repeat("😀", 20000) + `"}`))
	assert.True(t, truncated)
	assert.LessOrEqual(t, len(question), maxRecordedTextBytes)
	assert.True(t, utf8.ValidString(question))
}

func TestTokenRequestCaptureResponses(t *testing.T) {
	for _, tc := range []struct {
		name, body, want string
		failed           bool
	}{
		{"chat", `{"choices":[{"message":{"role":"assistant","content":"answer","reasoning_content":"private reasoning"}}]}`, "answer", false},
		{"completion", `{"choices":[{"text":"answer"}]}`, "answer", false},
		{"responses", `{"output":[{"type":"reasoning","summary":[]},{"type":"message","content":[{"type":"output_text","text":"answer"}]}]}`, "answer", false},
		{"claude", `{"content":[{"type":"thinking","thinking":"private"},{"type":"text","text":"answer"}]}`, "answer", false},
		{"gemini", `{"candidates":[{"content":{"parts":[{"thought":true,"text":"private"},{"text":"answer"}]}}]}`, "answer", false},
		{"gemini array", `[{"candidates":[{"content":{"parts":[{"text":"a"}]}}]},{"candidates":[{"content":{"parts":[{"text":"b"}]}}]}]`, "ab", false},
		{"failure", `{"error":{"message":"upstream error"}}`, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			capture := &TokenRequestCapture{ResponseWriter: c.Writer}
			n, err := capture.WriteString(tc.body)
			require.NoError(t, err)
			assert.Equal(t, len(tc.body), n)
			capture.consumeJSON(capture.pending)
			assert.Equal(t, tc.want, capture.answer.String())
			assert.Equal(t, tc.failed, capture.failed)
			assert.Equal(t, tc.body, recorder.Body.String())
		})
	}
}

func TestTokenRequestCaptureStreams(t *testing.T) {
	for _, tc := range []struct {
		name, body, want string
		failed           bool
	}{
		{"chat", "data: {\"choices\":[{\"delta\":{\"content\":\"你好\"}}]}\r\n\r\ndata: {\"choices\":[{\"delta\":{\"content\":\"世界\"}}]}\n\ndata: [DONE]\n\n", "你好世界", false},
		{"responses", "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"你好\"}\n\ndata: {\"type\":\"response.output_text.done\",\"text\":\"你好\"}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"output\":[{\"type\":\"message\",\"content\":[{\"type\":\"output_text\",\"text\":\"你好\"}]}]}}\n\n", "你好", false},
		{"claude", "data: {\"type\":\"content_block_start\",\"content_block\":{\"type\":\"text\",\"text\":\"你\"}}\n\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"好\"}}\n\ndata: {\"type\":\"message_stop\"}\n\n", "你好", false},
		{"gemini", "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"你好\"}]}}]}\n\n", "你好", false},
		{"multiline and error", "data: {\"type\":\"response.output_text.delta\",\ndata: \"delta\":\"你好\"}\n\ndata: {\"type\":\"error\",\"error\":{\"message\":\"failed\"}}\n\n", "你好", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Header("Content-Type", "text/event-stream; charset=utf-8")
			capture := &TokenRequestCapture{ResponseWriter: c.Writer}
			split := strings.Index(tc.body, "你") + 1 // Split inside a UTF-8 rune and JSON frame.
			_, err := capture.Write([]byte(tc.body[:split]))
			require.NoError(t, err)
			_, err = capture.WriteString(tc.body[split:])
			require.NoError(t, err)
			assert.Equal(t, tc.want, capture.answer.String())
			assert.Equal(t, tc.failed, capture.failed)
			assert.Equal(t, tc.body, recorder.Body.String())
		})
	}
}

func TestTokenRequestCaptureBounds(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Header("Content-Type", "text/event-stream")
	capture := &TokenRequestCapture{ResponseWriter: c.Writer}
	oversized := "data: " + strings.Repeat("x", maxCaptureFrameBytes+1) + "\n\n"
	_, err := capture.WriteString(oversized)
	require.NoError(t, err)
	assert.True(t, capture.wireTruncated)
	assert.LessOrEqual(t, len(capture.pending), maxCaptureFrameBytes)
	_, err = capture.WriteString("data: {\"choices\":[{\"delta\":{\"content\":\"recovered\"}}]}\n\n")
	require.NoError(t, err)
	assert.Equal(t, "recovered", capture.answer.String())
	capture.appendText(strings.Repeat("界", maxRecordedTextBytes))
	assert.True(t, capture.record.Truncated)
	assert.LessOrEqual(t, capture.answer.Len(), maxRecordedTextBytes)
	assert.True(t, utf8.ValidString(capture.answer.String()))
}

func TestTokenRequestCaptureTruncationPreservesPrefix(t *testing.T) {
	capture := &TokenRequestCapture{}
	capture.appendText(strings.Repeat("a", maxRecordedTextBytes-1))
	capture.appendText("你好")
	prefix := capture.answer.String()
	capture.appendText("b")
	assert.Equal(t, prefix, capture.answer.String())
	assert.True(t, capture.record.Truncated)
}
