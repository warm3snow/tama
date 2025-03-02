package agent

import (
	"testing"

	"github.com/warm3snow/tama/internal/config"
)

func TestNewAgent(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := New(cfg)

	if agent == nil {
		t.Fatal("Expected agent to be created, got nil")
	}

	if agent.config != cfg {
		t.Errorf("Expected agent.config to be %v, got %v", cfg, agent.config)
	}

	if agent.llm == nil {
		t.Error("Expected agent.llm to be initialized, got nil")
	}

	if agent.workspace == nil {
		t.Error("Expected agent.workspace to be initialized, got nil")
	}

	if agent.tools == nil {
		t.Error("Expected agent.tools to be initialized, got nil")
	}
}
