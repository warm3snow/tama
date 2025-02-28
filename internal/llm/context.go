package llm

// Context represents the context for an LLM interaction
type Context struct {
	Prompt    string                 // The user's prompt
	Machine   map[string]string      // Machine context information
	Workspace map[string]interface{} // Workspace context information
	Tools     []map[string]string    // Available tool descriptions
	History   []string               // History of tool executions
}

// AddToolResult adds a tool execution result to the context history
func (c *Context) AddToolResult(result string) {
	c.History = append(c.History, result)
}
