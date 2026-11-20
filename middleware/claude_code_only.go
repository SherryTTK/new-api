package middleware

import (
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

var claudeCodeUserAgent = regexp.MustCompile(`(?i)^claude-cli/\d+\.\d+\.\d+(?:$|[\s(+-])`)

// ClaudeCodeOnly runs after token authentication and before channel selection.
// Request fingerprints can be copied; token authentication remains mandatory.
func ClaudeCodeOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		policy := model.CurrentRequestPolicy().ClaudeCodeGate
		group := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
		if !policy.AppliesTo(group) {
			c.Next()
			return
		}
		// GET endpoints list/retrieve resources, except realtime whose model
		// comes from the query. Responses WebSocket events are checked separately.
		modelName := ""
		if c.Request.Method == http.MethodGet {
			if c.Request.URL.Path == "/v1/realtime" {
				modelName = c.Query("model")
			}
		} else {
			request, _, err := getModelRequest(c)
			if err != nil {
				// Parsing errors belong to the normal protocol validator. Restore
				// the body even on errors so observation cannot consume it.
				if value, exists := c.Get(common.KeyBodyStorage); exists {
					if storage, ok := value.(common.BodyStorage); ok {
						_, _ = storage.Seek(0, io.SeekStart)
						c.Request.Body = io.NopCloser(storage)
					}
				}
				c.Next()
				return
			}
			modelName = request.Model
		}
		if checkClaudeCodeRequest(c, group, modelName) {
			return
		}
		c.Next()
	}
}

// checkClaudeCodeRequest also runs after model resolution in Distribute, since
// task endpoints may resolve a model after the initial gate middleware.
func checkClaudeCodeRequest(c *gin.Context, group, modelName string) bool {
	policy := model.CurrentRequestPolicy().ClaudeCodeGate
	if !policy.AppliesToModel(group, modelName) {
		return false
	}
	checked := ModelRequest{Model: modelName, Group: group}
	if previous, exists := c.Get("claude_code_gate_checked_model"); exists && previous == checked {
		return false
	}
	c.Set("claude_code_gate_checked_model", checked)
	common.SetContextKey(c, constant.ContextKeyOriginalModel, modelName)
	messages := c.Request.Method == http.MethodPost && c.Request.URL.Path == "/v1/messages"
	// Official background sessions use cli-bg instead of the foreground cli.
	app := strings.TrimSpace(c.GetHeader("x-app"))
	reason := ""
	switch {
	case !messages:
		reason = "protocol_not_allowed"
	case !claudeCodeUserAgent.MatchString(strings.TrimSpace(c.GetHeader("User-Agent"))):
		reason = "user_agent_mismatch"
	case messages && !strings.EqualFold(app, "cli") && !strings.EqualFold(app, "cli-bg"):
		reason = "x_app_mismatch"
	case messages && strings.TrimSpace(c.GetHeader("x-claude-code-session-id")) == "":
		reason = "session_missing"
	}
	if reason == "" && messages {
		betaFound := false
		for _, value := range c.Request.Header.Values("anthropic-beta") {
			for beta := range strings.SplitSeq(value, ",") {
				beta = strings.ToLower(strings.TrimSpace(beta))
				if suffix, ok := strings.CutPrefix(beta, "claude-code-"); ok && suffix != "" {
					betaFound = true
				}
			}
		}
		if !betaFound {
			reason = "beta_missing"
		}
	}
	if reason != "" && rejectClaudeCodeRequest(c, group, reason, policy.Mode) {
		return true
	}
	if messages && policy.BodyMode != "off" {
		bodyReason := claudeCodeBodyFailure(c)
		if bodyReason != "" {
			mode := policy.BodyMode
			if policy.Mode == "shadow" {
				mode = "shadow"
			}
			if rejectClaudeCodeRequest(c, group, bodyReason, mode) {
				return true
			}
		}
	}
	return false
}

// claudeCodeBodyFailure checks only CC metadata and the system billing marker.
// It deliberately leaves protocol validation to the existing relay validator.
func claudeCodeBodyFailure(c *gin.Context) string {
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return "body_unreadable"
	}
	var request struct {
		Metadata struct {
			UserID string `json:"user_id"`
		} `json:"metadata"`
		System json.RawMessage `json:"system"`
	}
	err = common.DecodeJson(storage, &request)
	_, seekErr := storage.Seek(0, io.SeekStart)
	c.Request.Body = io.NopCloser(storage)
	if err != nil || seekErr != nil {
		return "body_unreadable"
	}
	var identity struct {
		DeviceID  string `json:"device_id"`
		SessionID string `json:"session_id"`
	}
	if err := common.UnmarshalJsonStr(request.Metadata.UserID, &identity); err != nil || strings.TrimSpace(identity.DeviceID) == "" || strings.TrimSpace(identity.SessionID) == "" {
		return "metadata_missing"
	}
	var text string
	if common.Unmarshal(request.System, &text) == nil && strings.Contains(text, "x-anthropic-billing-header:") {
		return ""
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if common.Unmarshal(request.System, &blocks) == nil {
		for _, block := range blocks {
			if block.Type == "text" && strings.Contains(block.Text, "x-anthropic-billing-header:") {
				return ""
			}
		}
	}
	return "billing_marker_missing"
}

func rejectClaudeCodeRequest(c *gin.Context, group, reason, mode string) bool {
	logger.LogWarn(c.Request.Context(), common.LogText(
		"Claude Code gate mode=%q group=%q reason=%q method=%q route=%q user_id=%d token_id=%d request_id=%q",
		mode, group, reason, c.Request.Method, c.FullPath(), c.GetInt("id"), common.GetContextKeyInt(c, constant.ContextKeyTokenId), c.GetString(common.RequestIdKey),
	))
	if mode != "enforce" {
		return false
	}
	if constant.ErrorLogEnabled {
		content := common.NewMessage("Claude Code gate rejected the request")
		switch reason {
		case "protocol_not_allowed":
			content = common.NewMessage("Claude Code gate rejected the request because the protocol is not allowed")
		case "user_agent_mismatch":
			content = common.NewMessage("Claude Code gate rejected the request because the User-Agent is missing or invalid")
		case "x_app_mismatch":
			content = common.NewMessage("Claude Code gate rejected the request because x-app must be cli or cli-bg")
		case "session_missing":
			content = common.NewMessage("Claude Code gate rejected the request because x-claude-code-session-id is missing")
		case "beta_missing":
			content = common.NewMessage("Claude Code gate rejected the request because the Claude Code beta header is missing")
		case "body_unreadable":
			content = common.NewMessage("Claude Code gate rejected the request because the request body could not be parsed")
		case "metadata_missing":
			content = common.NewMessage("Claude Code gate rejected the request because Claude Code metadata is missing or invalid")
		case "billing_marker_missing":
			content = common.NewMessage("Claude Code gate rejected the request because the billing marker is missing")
		case "playground_not_allowed":
			content = common.NewMessage("Claude Code gate rejected the request because Playground is not allowed for this group")
		}
		other := model.NewLogOther()
		other.SetPublic("content_parts", []*common.Message{content})
		other.SetPublic("request_path", c.FullPath())
		other.SetPublic("request_method", c.Request.Method)
		other.SetPublic("error_type", "permission_error")
		other.SetPublic("error_code", "claude_code_only")
		other.SetPublic("status_code", http.StatusForbidden)
		other.SetAdmin("reject_reason", content.Error())
		// Channel selection has not run yet; the requested model is already known.
		model.RecordErrorLog(c, c.GetInt("id"), 0, c.GetString("original_model"), c.GetString("token_name"),
			"status_code=403, "+content.Error(), common.GetContextKeyInt(c, constant.ContextKeyTokenId),
			0, common.GetContextKeyBool(c, constant.ContextKeyIsStream), group, other)
	}
	message := i18n.T(c, "relay.claude_code_only")
	if c.Request.URL.Path == "/v1/messages" {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"type":  "error",
			"error": gin.H{"type": "permission_error", "message": common.MessageWithRequestId(message, c.GetString(common.RequestIdKey))},
		})
	} else {
		abortWithOpenAiMessage(c, http.StatusForbidden, message)
	}
	return true
}

// ClaudeCodePlaygroundGuard checks the chosen group, including overrides by
// users whose own group is unrestricted. It never checks browser CC headers.
func ClaudeCodePlaygroundGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		policy := model.CurrentRequestPolicy().ClaudeCodeGate
		if policy.Mode == "off" || len(policy.Groups) == 0 {
			c.Next()
			return
		}
		request, err := getModelFromRequest(c)
		if err != nil {
			// Distribute reports malformed requests through its normal error path.
			c.Next()
			return
		}
		group := request.Group
		if group == "" {
			group = common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
		}
		if policy.AppliesToModel(group, request.Model) {
			common.SetContextKey(c, constant.ContextKeyOriginalModel, request.Model)
			c.Set("claude_code_gate_checked_model", ModelRequest{Model: request.Model, Group: group})
			if rejectClaudeCodeRequest(c, group, "playground_not_allowed", policy.Mode) {
				return
			}
		}
		c.Next()
	}
}
