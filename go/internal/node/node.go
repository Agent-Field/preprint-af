package node

import (
	"fmt"
	"os"
	"strings"

	"github.com/Agent-Field/agentfield/sdk/go/agent"
	"github.com/Agent-Field/agentfield/sdk/go/ai"
	"github.com/Agent-Field/preprint-af/go/internal/pipeline"
)

const (
	DefaultModel = "openrouter/deepseek/deepseek-v4-pro"
	Description  = "Turns a research folder into a submission-ready scientific paper: evidence ledger, positioning tournament, parallel section/figure drafting, compile gate, and a convergence-driven critique and repair loop."
)

type Node struct {
	App        *agent.Agent
	Pipeline   *pipeline.Service
	registered []string
}

func Build() (*Node, error) {
	// The pinned Go SDK defaults OpenCode to four concurrent processes, while
	// the reference Python provider defaults to ten and exposes this env knob.
	if os.Getenv("OPENCODE_MAX_CONCURRENT") == "" {
		if err := os.Setenv("OPENCODE_MAX_CONCURRENT", "10"); err != nil {
			return nil, fmt.Errorf("set OpenCode concurrency default: %w", err)
		}
	}
	model := envOr("AI_MODEL", DefaultModel)
	harnessModel := envOr("OPENCODE_MODEL", DefaultModel)
	server := envOr("AGENTFIELD_SERVER", "http://localhost:8080")
	port := envOr("PORT", "8001")
	key := os.Getenv("OPENROUTER_API_KEY")

	cfg := agent.Config{
		NodeID:        envOr("AGENT_NODE_ID", "preprint-af"),
		Version:       "2.0.0",
		AgentFieldURL: server,
		ListenAddress: ":" + port,
		PublicURL:     os.Getenv("AGENT_CALLBACK_URL"),
		Token:         os.Getenv("AGENTFIELD_API_KEY"),
		EnableDID:     true,
		VCEnabled:     true,
		Tags:          []string{"scientific-writing", "paper-generation", "latex", "figures"},
		CLIConfig:     &agent.CLIConfig{AppName: "preprint-af", AppDescription: Description},
		HarnessConfig: &agent.HarnessConfig{
			Provider:       "opencode",
			Model:          harnessModel,
			MaxTurns:       envInt("HARNESS_MAX_TURNS", 40),
			PermissionMode: envOr("HARNESS_PERMISSION_MODE", "auto"),
			Timeout:        envInt("AGENTFIELD_HARNESS_TIMEOUT_SECONDS", 1800),
			Env:            map[string]string{"OPENROUTER_API_KEY": key},
		},
	}
	if key != "" {
		cfg.AIConfig = &ai.Config{
			APIKey:   key,
			BaseURL:  "https://openrouter.ai/api/v1",
			Model:    strings.TrimPrefix(model, "openrouter/"),
			SiteName: "preprint-af",
		}
	}
	app, err := agent.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("create preprint-af Go node: %w", err)
	}
	return &Node{App: app, Pipeline: pipeline.New(app, cfg.NodeID)}, nil
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	var value int
	if _, err := fmt.Sscanf(os.Getenv(key), "%d", &value); err == nil && value > 0 {
		return value
	}
	return fallback
}
