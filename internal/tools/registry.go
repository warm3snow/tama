package tools

import (
	"context"
	"encoding/json"
)

// Tool represents a callable tool
type Tool interface {
	Name() string
	Description() string
	Execute(ctx context.Context, args map[string]interface{}) (string, error)
}

// Registry manages available tools
type Registry struct {
	tools map[string]Tool
}

// NewRegistry creates a new tool registry
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// RegisterTool adds a tool to the registry
func (r *Registry) RegisterTool(tool Tool) {
	r.tools[tool.Name()] = tool
}

// GetToolDescriptions returns descriptions of all registered tools
func (r *Registry) GetToolDescriptions() []map[string]string {
	var descriptions []map[string]string
	for _, tool := range r.tools {
		descriptions = append(descriptions, map[string]string{
			"name":        tool.Name(),
			"description": tool.Description(),
		})
	}
	return descriptions
}

// ParseToolCall parses a potential tool call from LLM response
func (r *Registry) ParseToolCall(response string) *ToolCall {
	// Try to parse as JSON tool call
	var call struct {
		Tool string                 `json:"tool"`
		Args map[string]interface{} `json:"args"`
	}

	if err := json.Unmarshal([]byte(response), &call); err != nil {
		return nil
	}

	tool, exists := r.tools[call.Tool]
	if !exists {
		return nil
	}

	return &ToolCall{
		tool: tool,
		args: call.Args,
	}
}

// ToolCall represents a parsed tool call ready for execution
type ToolCall struct {
	tool Tool
	args map[string]interface{}
}

// Execute runs the tool with provided arguments
func (tc *ToolCall) Execute(ctx context.Context) string {
	result, err := tc.tool.Execute(ctx, tc.args)
	if err != nil {
		return "Error executing tool: " + err.Error()
	}
	return result
}
