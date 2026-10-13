package service

import (
	"bytes"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const maxRecordedTextBytes = 60000 // Fits MySQL TEXT even for multibyte UTF-8.
const maxCaptureFrameBytes = 1024 * 1024

// TokenRequestCapture observes the client-facing protocol after all provider
// conversions, without buffering or changing the response sent to the client.
type TokenRequestCapture struct {
	gin.ResponseWriter
	mu              sync.Mutex
	record          model.TokenLastRequest
	answer          strings.Builder
	pending         []byte
	event           []byte
	discardLine     bool
	wireTruncated   bool
	answerTruncated bool
	failed          bool
}

func BeginTokenRequestCapture(c *gin.Context, isStream bool) *TokenRequestCapture {
	if !c.GetBool("token_record_last_request") || !types.IsFixedResponsePath(c.Request.URL.Path) {
		return nil
	}
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		logger.LogWarn(c, "cannot read latest request body: "+err.Error())
		return nil
	}
	body, err := storage.Bytes()
	if err != nil {
		logger.LogWarn(c, "cannot read latest request body: "+err.Error())
		return nil
	}
	question, truncated := latestUserQuestion(body)
	modelName := common.GetContextKeyString(c, constant.ContextKeyOriginalModel)
	if len(modelName) > 255 {
		modelName = modelName[:255]
		for !utf8.ValidString(modelName) {
			modelName = modelName[:len(modelName)-1]
		}
	}
	started := common.GetContextKeyTime(c, constant.ContextKeyRequestStartTime)
	if started.IsZero() {
		started = time.Now()
	}
	capture := &TokenRequestCapture{ResponseWriter: c.Writer, record: model.TokenLastRequest{
		TokenId: c.GetInt("token_id"), UserId: c.GetInt("id"), RequestId: common.GetUUID(), StartedAt: started.UnixNano(), CreatedAt: started.UnixMilli(),
		Model: modelName, Question: question, Status: "pending", IsStream: isStream, Truncated: truncated,
	}}
	enabled, err := model.BeginTokenLastRequest(&capture.record)
	if err != nil {
		logger.LogWarn(c, "cannot store latest request: "+err.Error())
		return nil
	}
	if !enabled {
		return nil
	}
	c.Writer = capture
	return capture
}

func (w *TokenRequestCapture) Write(data []byte) (int, error) {
	n, err := w.ResponseWriter.Write(data)
	w.mu.Lock()
	defer w.mu.Unlock()
	if err != nil || n != len(data) {
		w.failed = true
	}
	if n > 0 {
		w.observe(data[:n])
	}
	return n, err
}

func (w *TokenRequestCapture) WriteString(data string) (int, error) { return w.Write([]byte(data)) }

func (w *TokenRequestCapture) observe(data []byte) {
	if !strings.HasPrefix(w.Header().Get("Content-Type"), "text/event-stream") {
		remaining := maxCaptureFrameBytes - len(w.pending)
		if len(data) > remaining {
			data = data[:remaining]
			w.wireTruncated = true
		}
		w.pending = append(w.pending, data...)
		return
	}
	for len(data) > 0 {
		end := bytes.IndexByte(data, '\n')
		part := data
		if end >= 0 {
			part = data[:end]
		}
		if !w.discardLine {
			if len(w.pending)+len(part) > maxCaptureFrameBytes {
				w.pending = nil
				w.event = nil
				w.discardLine = true
				w.wireTruncated = true
			} else {
				w.pending = append(w.pending, part...)
			}
		}
		if end < 0 {
			return
		}
		if !w.discardLine {
			w.consumeLine(bytes.TrimSuffix(w.pending, []byte{'\r'}))
		}
		w.pending = w.pending[:0]
		w.discardLine = false
		data = data[end+1:]
	}
}

func (w *TokenRequestCapture) consumeLine(line []byte) {
	if len(line) == 0 {
		if len(w.event) > 0 {
			w.consumeJSON(w.event)
			w.event = w.event[:0]
		}
		return
	}
	if !bytes.HasPrefix(line, []byte("data:")) {
		return
	}
	data := bytes.TrimPrefix(line, []byte("data:"))
	data = bytes.TrimPrefix(data, []byte(" "))
	if len(w.event)+len(data)+1 > maxCaptureFrameBytes {
		w.event = nil
		w.wireTruncated = true
		return
	}
	w.event = append(w.event, data...)
	w.event = append(w.event, '\n')
}

func (w *TokenRequestCapture) appendText(text string) {
	if text == "" || w.answerTruncated {
		return
	}
	remaining := maxRecordedTextBytes - w.answer.Len()
	if len(text) > remaining {
		text = text[:remaining]
		for !utf8.ValidString(text) {
			text = text[:len(text)-1]
		}
		w.record.Truncated = true
		w.answerTruncated = true
	}
	w.answer.WriteString(text)
}

func (w *TokenRequestCapture) consumeJSON(data []byte) {
	if !gjson.ValidBytes(data) {
		return
	}
	root := gjson.ParseBytes(data)
	if root.IsArray() {
		for _, item := range root.Array() {
			w.consumeJSON([]byte(item.Raw))
		}
		return
	}
	eventType := root.Get("type").String()
	if root.Get("error").Exists() && root.Get("error").Type != gjson.Null || eventType == "response.failed" || eventType == "error" {
		w.failed = true
	}
	switch eventType {
	case "response.output_text.delta", "response.refusal.delta":
		w.appendText(root.Get("delta").String())
		return
	case "response.completed", "response.incomplete":
		if eventType == "response.incomplete" {
			w.failed = true
		}
		if w.answer.Len() == 0 {
			w.consumeJSON([]byte(root.Get("response").Raw))
		}
		return
	case "content_block_start":
		w.appendText(root.Get("content_block.text").String())
		return
	case "content_block_delta":
		if root.Get("delta.type").String() == "text_delta" {
			w.appendText(root.Get("delta.text").String())
		}
		return
	}
	if strings.HasPrefix(eventType, "response.") {
		return
	} // Ignore snapshots duplicating deltas.
	choice := root.Get("choices.0")
	if choice.Exists() {
		if delta := choice.Get("delta"); delta.Exists() {
			w.appendText(delta.Get("content").String())
			w.appendText(delta.Get("refusal").String())
		} else if message := choice.Get("message"); message.Exists() {
			w.appendText(recordedContentText(message.Get("content")))
			w.appendText(message.Get("refusal").String())
		} else {
			w.appendText(choice.Get("text").String())
		}
		return
	}
	if output := root.Get("output"); output.IsArray() {
		for _, item := range output.Array() {
			if item.Get("type").String() == "message" {
				w.appendText(recordedContentText(item.Get("content")))
			}
		}
		return
	}
	if content := root.Get("content"); content.IsArray() {
		w.appendText(recordedContentText(content))
		return
	}
	for _, part := range root.Get("candidates.0.content.parts").Array() {
		if !part.Get("thought").Bool() {
			w.appendText(part.Get("text").String())
		}
	}
}

func (w *TokenRequestCapture) Finish(c *gin.Context, failed bool) {
	w.mu.Lock()
	if strings.HasPrefix(w.Header().Get("Content-Type"), "text/event-stream") {
		if len(w.pending) > 0 && !w.discardLine {
			w.consumeLine(w.pending)
		}
		w.consumeJSON(w.event)
	} else if !w.wireTruncated {
		w.consumeJSON(w.pending)
	}
	w.record.Answer = w.answer.String()
	w.record.Truncated = w.record.Truncated || w.wireTruncated
	w.record.Status = "completed"
	if failed || w.failed || w.Status() >= 400 {
		w.record.Status = "failed"
	}
	if c.Request.Context().Err() != nil {
		w.record.Status = "cancelled"
	}
	record := w.record
	w.mu.Unlock()
	if err := model.CompleteTokenLastRequest(&record); err != nil {
		logger.LogWarn(c, "cannot save latest response: "+err.Error())
	}
}

// Record only the most recent user turn, not hidden instructions or chat history.
func latestUserQuestion(body []byte) (string, bool) {
	root := gjson.ParseBytes(body)
	var text string
	switch {
	case root.Get("messages").IsArray():
		messages := root.Get("messages").Array()
		for i := len(messages) - 1; i >= 0; i-- {
			if messages[i].Get("role").String() == "user" {
				text = recordedContentText(messages[i].Get("content"))
				break
			}
		}
	case root.Get("input").Exists():
		input := root.Get("input")
		if input.Type == gjson.String {
			text = input.String()
		} else {
			messages := input.Array()
			for i := len(messages) - 1; i >= 0; i-- {
				role := messages[i].Get("role").String()
				if role == "user" || role == "" && messages[i].Get("content").Exists() {
					text = recordedContentText(messages[i].Get("content"))
					break
				}
			}
		}
	case root.Get("contents").IsArray():
		messages := root.Get("contents").Array()
		for i := len(messages) - 1; i >= 0; i-- {
			role := messages[i].Get("role").String()
			if role == "user" || role == "" {
				text = recordedContentText(messages[i].Get("parts"))
				break
			}
		}
	default:
		prompt := root.Get("prompt")
		if prompt.Type == gjson.String {
			text = prompt.String()
		} else {
			var parts []string
			for _, part := range prompt.Array() {
				if part.Type == gjson.String {
					parts = append(parts, part.String())
				}
			}
			text = strings.Join(parts, "\n")
		}
	}
	if len(text) <= maxRecordedTextBytes {
		return text, false
	}
	text = text[:maxRecordedTextBytes]
	for !utf8.ValidString(text) {
		text = text[:len(text)-1]
	}
	return text, true
}

func recordedContentText(content gjson.Result) string {
	if content.Type == gjson.String {
		return content.String()
	}
	var parts []string
	for _, part := range content.Array() {
		kind := part.Get("type").String()
		if kind == "refusal" {
			parts = append(parts, part.Get("refusal").String())
			continue
		}
		if kind != "" && kind != "text" && kind != "input_text" && kind != "output_text" {
			continue
		}
		if text := part.Get("text"); text.Type == gjson.String && !part.Get("thought").Bool() {
			parts = append(parts, text.String())
		}
	}
	return strings.Join(parts, "\n")
}
