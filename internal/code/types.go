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
