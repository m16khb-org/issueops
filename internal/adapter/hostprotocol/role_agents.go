package hostprotocol

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	modelcontract "issueops/internal/contract/agentmodel"
)

// RoleAgent is one sub-agent injected into an owner session.
type RoleAgent struct {
	Role   modelcontract.Role
	Model  string
	Effort string
}

var roleAgentText = map[modelcontract.Role]struct{ description, prompt string }{
	modelcontract.RolePlanReview: {
		"IssueOps plan review: adversarially review a cycle plan in a fresh context.",
		"You review an IssueOps cycle plan adversarially in a fresh context. Follow the review packet you are given and the issueops-review skill, and return only the verdict and findings it asks for. Do not edit files.",
	},
	modelcontract.RoleDiffReview: {
		"IssueOps implementation review: adversarially review a cycle diff in a fresh context.",
		"You review an IssueOps implementation diff adversarially in a fresh context. Follow the review packet you are given and the issueops-review skill, and return only the verdict and findings it asks for. Do not edit files.",
	},
	modelcontract.RoleReviewEscalate: {
		"IssueOps review rounds 3 to 5: the same review one effort rung higher.",
		"You run an escalated IssueOps review round (3 to 5) in a fresh context. Follow the review packet and the issueops-review skill exactly as a regular review would, and return only the verdict and findings it asks for. Do not edit files.",
	},
	modelcontract.RoleResearch: {
		"IssueOps bounded research: read-only lookups that answer one stated question.",
		"You answer one bounded research question for an IssueOps cycle. Read files, run read-only commands, and report findings with file:line or URL evidence. Do not edit files, change state, or write to remotes.",
	},
	modelcontract.RoleReaderCheck: {
		"IssueOps reader check: read a drafted issue or PR body as a first-time reader.",
		"You read a drafted issue, PR, or MR body as someone seeing the work for the first time. Report each sentence you could not follow and why, and what a reader would still need to know. Do not rewrite the body or edit files.",
	},
}

// roleAgentName is the agent name a host sees for role.
func roleAgentName(role modelcontract.Role) string { return "issueops-" + string(role) }

// ClaudeAgentsJSON renders the claude --agents object. Agents without a
// model are skipped.
func ClaudeAgentsJSON(agents []RoleAgent) (string, error) {
	type definition struct {
		Description string `json:"description"`
		Prompt      string `json:"prompt"`
		Model       string `json:"model"`
		Effort      string `json:"effort,omitempty"`
	}
	definitions := map[string]definition{}
	for _, agent := range agents {
		text, ok := roleAgentText[agent.Role]
		if !ok {
			return "", fmt.Errorf("role %q is not an injectable sub-agent", agent.Role)
		}
		if agent.Model == "" {
			continue
		}
		definitions[roleAgentName(agent.Role)] = definition{Description: text.description, Prompt: text.prompt, Model: agent.Model, Effort: agent.Effort}
	}
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(definitions); err != nil {
		return "", err
	}
	return strings.TrimSuffix(out.String(), "\n"), nil
}

// CodexRoleFile renders the Codex agent role file for agent.
func CodexRoleFile(agent RoleAgent) (string, error) {
	text, ok := roleAgentText[agent.Role]
	if !ok {
		return "", fmt.Errorf("role %q is not an injectable sub-agent", agent.Role)
	}
	var out strings.Builder
	fmt.Fprintf(&out, "name = %s\n", tomlString(roleAgentName(agent.Role)))
	fmt.Fprintf(&out, "description = %s\n", tomlString(text.description))
	fmt.Fprintf(&out, "model = %s\n", tomlString(agent.Model))
	if agent.Effort != "" {
		fmt.Fprintf(&out, "model_reasoning_effort = %s\n", tomlString(agent.Effort))
	}
	fmt.Fprintf(&out, "developer_instructions = %s\n", tomlString(text.prompt))
	return out.String(), nil
}

// RoleAgentArgs renders the launch arguments that inject agents into a host
// session: claude gets one --agents JSON, codex gets one -c override per role
// file that writeRoleFile stores. omo and omp get none.
func RoleAgentArgs(host string, agents []RoleAgent, writeRoleFile func(content string) (string, error)) ([]string, error) {
	if len(agents) == 0 {
		return nil, nil
	}
	switch host {
	case "claude":
		definitions, err := ClaudeAgentsJSON(agents)
		if err != nil {
			return nil, err
		}
		return []string{"--agents", definitions}, nil
	case "codex":
		var args []string
		for _, agent := range agents {
			if agent.Model == "" {
				continue
			}
			content, err := CodexRoleFile(agent)
			if err != nil {
				return nil, err
			}
			path, err := writeRoleFile(content)
			if err != nil {
				return nil, fmt.Errorf("write Codex role file for %s: %w", agent.Role, err)
			}
			args = append(args, "-c", "agents."+roleAgentName(agent.Role)+".config_file="+tomlString(path))
		}
		return args, nil
	default:
		return nil, nil
	}
}

// BuildPrintArgv is the empty-context, non-interactive argv a skill uses to
// run a role model directly. omo has none.
func BuildPrintArgv(host, model, effort string) []string {
	switch host {
	case "claude":
		argv := []string{"claude", "-p", "--model", model}
		if effort != "" {
			argv = append(argv, "--effort", effort)
		}
		return argv
	case "codex":
		argv := []string{"codex", "exec", "-m", model}
		if effort != "" {
			argv = append(argv, "-c", "model_reasoning_effort="+effort)
		}
		return argv
	case "omp":
		argv := []string{"omp", "-p", "--model", model}
		if effort != "" {
			argv = append(argv, "--thinking", effort)
		}
		return argv
	default:
		return nil
	}
}

// tomlString renders a TOML basic string. JSON string escapes are a subset of
// TOML basic string escapes.
func tomlString(value string) string {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(value)
	return strings.TrimSuffix(out.String(), "\n")
}
