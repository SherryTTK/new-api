package types

import (
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestFixedResponseValidation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		config *FixedResponseConfig
		valid  bool
	}{
		{"absent", nil, true},
		{"disabled", &FixedResponseConfig{}, true},
		{"enabled", &FixedResponseConfig{Enabled: true, Content: "你好", MinDelayMS: 10, MaxDelayMS: 30}, true},
		{"blank", &FixedResponseConfig{Enabled: true, Content: " \n"}, false},
		{"negative", &FixedResponseConfig{MinDelayMS: -1}, false},
		{"reversed", &FixedResponseConfig{MinDelayMS: 2, MaxDelayMS: 1}, false},
		{"excessive delay", &FixedResponseConfig{MaxDelayMS: 300001}, false},
		{"excessive content", &FixedResponseConfig{Content: strings.Repeat("字", 21846)}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate()
			if tc.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestFixedResponsePaths(t *testing.T) {
	for _, path := range []string{"/v1/chat/completions", "/v1/completions", "/v1/responses", "/v1/messages", "/v1beta/models/gemini:generateContent", "/v1/models/gemini:streamGenerateContent"} {
		assert.True(t, IsFixedResponsePath(path), path)
	}
	for _, path := range []string{"/v1/embeddings", "/v1/responses/compact", "/v1/models/x:embedContent", "/v1/images/generations", "/v1/realtime", "/v1/audio/speech", "/v1beta/models/x:countTokens"} {
		assert.False(t, IsFixedResponsePath(path), path)
	}
}
