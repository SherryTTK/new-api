package types

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const MaxFixedResponseBytes = 64 * 1024
const MaxFixedResponseDelayMS = 300000
const FixedResponseContextKey = "token_fixed_response"

// FixedResponseConfig replaces text generation with a locally billed reply.
type FixedResponseConfig struct {
	Enabled    bool   `json:"enabled"`
	MinDelayMS int    `json:"min_delay_ms"`
	MaxDelayMS int    `json:"max_delay_ms"`
	Content    string `json:"content"`
}

func (f *FixedResponseConfig) Validate() error {
	if f == nil {
		return nil
	}
	if f.MinDelayMS < 0 || f.MaxDelayMS < f.MinDelayMS || f.MaxDelayMS > MaxFixedResponseDelayMS {
		return fmt.Errorf("fixed response delay must satisfy 0 <= min <= max <= %d ms", MaxFixedResponseDelayMS)
	}
	if !utf8.ValidString(f.Content) || len(f.Content) > MaxFixedResponseBytes {
		return fmt.Errorf("fixed response content must be valid UTF-8 and at most %d bytes", MaxFixedResponseBytes)
	}
	if f.Enabled && strings.TrimSpace(f.Content) == "" {
		return fmt.Errorf("fixed response content is required")
	}
	return nil
}

func IsFixedResponsePath(path string) bool {
	switch path {
	case "/v1/chat/completions", "/v1/completions", "/v1/responses", "/v1/messages":
		return true
	}
	return (strings.HasPrefix(path, "/v1beta/models/") || strings.HasPrefix(path, "/v1/models/")) &&
		(strings.HasSuffix(path, ":generateContent") || strings.HasSuffix(path, ":streamGenerateContent"))
}
