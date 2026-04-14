package agent

import (
	"strings"
	"testing"

	"github.com/nextlevelbuilder/goclaw/internal/bootstrap"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// TestBuildSystemPrompt_NoOnboardingRitual verifies that the onboarding
// prompt sections (FIRST RUN, USER PROFILE INCOMPLETE) are never injected,
// regardless of whether BOOTSTRAP.md is present or USER.md is blank.
// Bots should answer the user's first message directly without greeting
// or asking for name/language/timezone.
func TestBuildSystemPrompt_NoOnboardingRitual(t *testing.T) {
	blankUserMD := "# USER.md\n\n- **Name:**\n- **Language:**\n- **Timezone:**\n"
	populatedUserMD := "# USER.md\n\n- **Name:** Alice\n- **Language:** English\n- **Timezone:** UTC+7\n"

	tests := []struct {
		name string
		cfg  SystemPromptConfig
	}{
		{
			name: "open agent with BOOTSTRAP.md present",
			cfg: SystemPromptConfig{
				IsBootstrap: true,
				AgentType:   store.AgentTypeOpen,
				ContextFiles: []bootstrap.ContextFile{
					{Path: bootstrap.BootstrapFile, Content: "# BOOTSTRAP"},
					{Path: bootstrap.UserFile, Content: blankUserMD},
				},
				ToolNames: []string{"write_file", "Write"},
			},
		},
		{
			name: "predefined agent with BOOTSTRAP.md present",
			cfg: SystemPromptConfig{
				AgentType: store.AgentTypePredefined,
				ContextFiles: []bootstrap.ContextFile{
					{Path: bootstrap.BootstrapFile, Content: "# BOOTSTRAP"},
					{Path: bootstrap.UserFile, Content: blankUserMD},
				},
				ToolNames: []string{"write_file", "skill_search"},
			},
		},
		{
			name: "blank USER.md without BOOTSTRAP.md",
			cfg: SystemPromptConfig{
				AgentType: store.AgentTypePredefined,
				ContextFiles: []bootstrap.ContextFile{
					{Path: bootstrap.UserFile, Content: blankUserMD},
				},
				ToolNames: []string{"write_file"},
			},
		},
		{
			name: "populated USER.md without BOOTSTRAP.md",
			cfg: SystemPromptConfig{
				AgentType: store.AgentTypePredefined,
				ContextFiles: []bootstrap.ContextFile{
					{Path: bootstrap.UserFile, Content: populatedUserMD},
				},
				ToolNames: []string{"write_file"},
			},
		},
		{
			name: "no context files at all",
			cfg: SystemPromptConfig{
				AgentType: store.AgentTypePredefined,
				ToolNames: []string{"write_file"},
			},
		},
	}

	forbidden := []string{
		"FIRST RUN",
		"USER PROFILE INCOMPLETE",
		"only have write_file available",
		"MUST ALSO call write_file",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := BuildSystemPrompt(tt.cfg)
			for _, phrase := range forbidden {
				if strings.Contains(prompt, phrase) {
					t.Errorf("system prompt must not contain %q (onboarding ritual is disabled)", phrase)
				}
			}
		})
	}
}
