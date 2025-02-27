package completion

// ReadlineCompleter 适配 CommandCompleter 以满足 readline.AutoCompleter 接口
type ReadlineCompleter struct {
	completer *CommandCompleter
}

// NewReadlineCompleter 创建一个新的readline兼容的自动补全器
func NewReadlineCompleter(specificCommands []string) *ReadlineCompleter {
	return &ReadlineCompleter{
		completer: NewCommandCompleter(specificCommands),
	}
}

// Do 实现 readline.AutoCompleter 接口
func (r *ReadlineCompleter) Do(line []rune, pos int) (newLine [][]rune, length int) {
	return r.completer.DoComplete(line, pos)
}
