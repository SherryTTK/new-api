package operation_setting

import (
	"regexp"
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// ClaudeCodeGateSetting describes request fingerprints, not client authentication.
type ClaudeCodeGateSetting struct {
	Mode     string
	Groups   []string
	BodyMode string
}

func (s ClaudeCodeGateSetting) AppliesTo(group string) bool {
	return s.Mode != "off" && slices.Contains(s.Groups, group)
}

// Matches Claude names with provider prefixes, including OpenRouter and Bedrock.
// Scope is the requested model name; arbitrary channel mapping aliases are not
// inferred from the name. Ignore request modifiers when identifying the family.
var claudeModelName = regexp.MustCompile(`(?i)(?:^|[/:.])claude(?:-|$)`)

func (s ClaudeCodeGateSetting) AppliesToModel(group, modelName string) bool {
	modelName, _, _ = strings.Cut(strings.TrimSpace(modelName), "@")
	return s.AppliesTo(group) && claudeModelName.MatchString(modelName)
}

func (s ClaudeCodeGateSetting) Validate() error {
	for _, mode := range []string{s.Mode, s.BodyMode} {
		if mode != "off" && mode != "shadow" && mode != "enforce" {
			return common.NewMessage("Claude Code gate mode must be off, shadow or enforce")
		}
	}
	seen := make(map[string]bool)
	for _, group := range s.Groups {
		if group == "" || group != strings.TrimSpace(group) || group == "auto" || seen[group] {
			return common.NewMessage("Claude Code gate groups must be unique, non-empty explicit group names")
		}
		seen[group] = true
	}
	return nil
}
