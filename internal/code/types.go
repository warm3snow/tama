package code

// SlashCommand represents a slash command that can be executed
type SlashCommand struct {
	Name        string
	Description string
	Execute     func() error
}

// CodeAction represents a code-related action that can be performed
type CodeAction struct {
	Type        string `json:"type"`        // "analyze", "edit", "create", etc.
	FilePath    string `json:"file_path"`   // Path to the file to be analyzed/edited
	Content     string `json:"content"`     // New content for edit/create actions
	StartLine   int    `json:"start_line"`  // Starting line for edits (optional)
	EndLine     int    `json:"end_line"`    // Ending line for edits (optional)
	Description string `json:"description"` // Description of the action
}

// CodeChangeResponse indicates the user's decision about a code change
type CodeChangeResponse int

const (
	Accept CodeChangeResponse = iota
	Reject
	Cancel
)

// CommandAnalysisResponse represents the structured response for command analysis
type CommandAnalysisResponse struct {
	IsCommand bool   `json:"is_command"`
	Command   string `json:"command"`
	Reason    string `json:"reason"`
}

// ContextType represents the type of context that can be provided to the code assistant
type ContextType string

const (
	// Context types
	FileContext     ContextType = "file"     // Single file context
	FolderContext   ContextType = "folder"   // Directory structure context
	CodebaseContext ContextType = "codebase" // Whole codebase context
	GitContext      ContextType = "git"      // Git repository context
	WebContext      ContextType = "web"      // Web search context
)

// ContextRequest represents a request for additional context
type ContextRequest struct {
	Type     ContextType `json:"type"`     // Type of context requested
	Target   string      `json:"target"`   // Target file/folder/URL/etc.
	Depth    int         `json:"depth"`    // Depth of context (for folder/codebase)
	Command  string      `json:"command"`  // Command for git operations
	Question string      `json:"question"` // User's question about the context
}
