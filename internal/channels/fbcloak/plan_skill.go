//go:build !sqliteonly

package fbcloak

import (
	_ "embed"
	"strings"
)

// orchestrateSkill is the embedded LLM system prompt. Loaded from
// bundled-skills/fbcloak/orchestrate.md at build time.
//
//go:embed orchestrate_skill.md
var orchestrateSkill string

// OrchestrateSkillTemplate returns the raw system prompt template with
// placeholders intact. Caller substitutes {{fanpage_name}} per-request.
func OrchestrateSkillTemplate() string {
	return orchestrateSkill
}

// RenderOrchestrateSkill substitutes the template variables and returns
// the final system prompt to send to the LLM.
func RenderOrchestrateSkill(vars map[string]string) string {
	out := orchestrateSkill
	for k, v := range vars {
		out = strings.ReplaceAll(out, "{{"+k+"}}", v)
	}
	return out
}
