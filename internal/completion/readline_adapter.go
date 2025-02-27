package completion

// ReadlineCompleter adapts CommandCompleter to satisfy the readline.AutoCompleter interface
type ReadlineCompleter struct {
	completer *CommandCompleter
}

// NewReadlineCompleter creates a new readline-compatible auto-completer
func NewReadlineCompleter(specificCommands []string) *ReadlineCompleter {
	return &ReadlineCompleter{
		completer: NewCommandCompleter(specificCommands),
	}
}

// Do implements the readline.AutoCompleter interface
func (r *ReadlineCompleter) Do(line []rune, pos int) (newLine [][]rune, length int) {
	return r.completer.DoComplete(line, pos)
}
