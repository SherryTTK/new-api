package common

import (
	"bytes"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const maxOutputTrackingBuffer = 1024 * 1024

type OutputTrackingWriter struct {
	gin.ResponseWriter
	info    *RelayInfo
	mu      sync.Mutex
	pending []byte
}

func WrapOutputTrackingWriter(c *gin.Context, info *RelayInfo) {
	if c == nil || info == nil {
		return
	}
	if writer, ok := c.Writer.(*OutputTrackingWriter); ok {
		writer.mu.Lock()
		writer.info = info
		writer.pending = writer.pending[:0]
		writer.mu.Unlock()
		return
	}
	c.Writer = &OutputTrackingWriter{
		ResponseWriter: c.Writer,
		info:           info,
	}
}

func (w *OutputTrackingWriter) Write(data []byte) (int, error) {
	w.observe(data)
	return w.ResponseWriter.Write(data)
}

func (w *OutputTrackingWriter) WriteString(data string) (int, error) {
	w.observe([]byte(data))
	return w.ResponseWriter.WriteString(data)
}

func (w *OutputTrackingWriter) observe(data []byte) {
	if w == nil || w.info == nil || len(data) == 0 || w.info.HasValidOutput() {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()

	w.pending = append(w.pending, data...)
	if len(w.pending) > maxOutputTrackingBuffer {
		w.pending = append([]byte(nil), data...)
	}

	if gjson.ValidBytes(w.pending) {
		if hasSemanticOutput(w.pending) {
			w.info.MarkValidOutput()
		}
		w.pending = w.pending[:0]
		return
	}

	lastNewline := bytes.LastIndexByte(w.pending, '\n')
	if lastNewline < 0 {
		return
	}
	for _, line := range bytes.Split(w.pending[:lastNewline+1], []byte{'\n'}) {
		line = bytes.TrimSpace(line)
		line = bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
		if len(line) == 0 || bytes.Equal(line, []byte("[DONE]")) || !gjson.ValidBytes(line) {
			continue
		}
		if hasSemanticOutput(line) {
			w.info.MarkValidOutput()
			break
		}
	}
	w.pending = append(w.pending[:0], w.pending[lastNewline+1:]...)
}

func hasSemanticOutput(data []byte) bool {
	root := gjson.ParseBytes(data)
	eventType := root.Get("type").String()
	if isSemanticStreamEvent(eventType, root) {
		return true
	}

	for _, choice := range root.Get("choices").Array() {
		if resultHasNonEmptyString(choice, "text") ||
			messageHasSemanticOutput(choice.Get("message")) ||
			messageHasSemanticOutput(choice.Get("delta")) {
			return true
		}
	}

	for _, output := range root.Get("output").Array() {
		if responsesItemHasSemanticOutput(output) {
			return true
		}
	}
	if responsesItemHasSemanticOutput(root.Get("item")) {
		return true
	}

	if resultHasNonEmptyString(root, "completion") {
		return true
	}
	for _, content := range root.Get("content").Array() {
		if claudeContentHasSemanticOutput(content) {
			return true
		}
	}
	if claudeContentHasSemanticOutput(root.Get("content_block")) ||
		claudeContentHasSemanticOutput(root.Get("delta")) {
		return true
	}

	for _, candidate := range root.Get("candidates").Array() {
		for _, part := range candidate.Get("content.parts").Array() {
			if geminiPartHasSemanticOutput(part) {
				return true
			}
		}
		for _, part := range candidate.Get("parts").Array() {
			if geminiPartHasSemanticOutput(part) {
				return true
			}
		}
	}
	return false
}

func HasValidRelayOutput(data []byte) bool {
	return gjson.ValidBytes(data) && hasSemanticOutput(data)
}

func isSemanticStreamEvent(eventType string, root gjson.Result) bool {
	if eventType == "" {
		return false
	}
	if strings.Contains(eventType, "output_text") ||
		strings.Contains(eventType, "reasoning") ||
		strings.Contains(eventType, "thinking") ||
		strings.Contains(eventType, "refusal") ||
		strings.Contains(eventType, "audio") ||
		strings.Contains(eventType, "function_call") ||
		strings.Contains(eventType, "tool_call") {
		if resultHasNonEmptyString(root, "delta") || resultHasNonEmptyString(root, "text") ||
			root.Get("item").Exists() || root.Get("part").Exists() {
			return true
		}
	}
	if eventType == "content_block_start" {
		return claudeContentHasSemanticOutput(root.Get("content_block"))
	}
	if eventType == "content_block_delta" {
		return claudeContentHasSemanticOutput(root.Get("delta"))
	}
	return false
}

func messageHasSemanticOutput(message gjson.Result) bool {
	if !message.Exists() {
		return false
	}
	if resultHasNonEmptyString(message, "content") ||
		resultHasNonEmptyString(message, "reasoning_content") ||
		resultHasNonEmptyString(message, "reasoning") ||
		resultHasNonEmptyString(message, "thinking") ||
		resultHasNonEmptyString(message, "refusal") ||
		resultHasNonEmptyString(message, "audio.data") ||
		resultHasNonEmptyString(message, "audio.transcript") {
		return true
	}
	for _, part := range message.Get("content").Array() {
		if resultHasNonEmptyString(part, "text") ||
			resultHasNonEmptyString(part, "thinking") ||
			resultHasNonEmptyString(part, "refusal") {
			return true
		}
	}
	return len(message.Get("tool_calls").Array()) > 0 || message.Get("function_call").Exists()
}

func responsesItemHasSemanticOutput(item gjson.Result) bool {
	if !item.Exists() {
		return false
	}
	itemType := item.Get("type").String()
	if itemType != "" && itemType != "message" {
		return true
	}
	for _, content := range item.Get("content").Array() {
		if resultHasNonEmptyString(content, "text") ||
			resultHasNonEmptyString(content, "refusal") {
			return true
		}
	}
	for _, summary := range item.Get("summary").Array() {
		if resultHasNonEmptyString(summary, "text") {
			return true
		}
	}
	return false
}

func claudeContentHasSemanticOutput(content gjson.Result) bool {
	if !content.Exists() {
		return false
	}
	contentType := content.Get("type").String()
	if contentType == "tool_use" || contentType == "server_tool_use" {
		return true
	}
	return resultHasNonEmptyString(content, "text") ||
		resultHasNonEmptyString(content, "thinking") ||
		resultHasNonEmptyString(content, "data") ||
		resultHasNonEmptyString(content, "partial_json") ||
		resultHasNonEmptyString(content, "delta")
}

func geminiPartHasSemanticOutput(part gjson.Result) bool {
	return resultHasNonEmptyString(part, "text") ||
		resultHasNonEmptyString(part, "thoughtSignature") ||
		resultHasNonEmptyString(part, "thought_signature") ||
		part.Get("functionCall").Exists() ||
		part.Get("function_call").Exists() ||
		part.Get("executableCode").Exists() ||
		part.Get("codeExecutionResult").Exists()
}

func resultHasNonEmptyString(result gjson.Result, path string) bool {
	value := result.Get(path)
	return value.Type == gjson.String && strings.TrimSpace(value.String()) != ""
}
