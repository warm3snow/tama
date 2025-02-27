package completion

import (
	"os"
	"strings"
)

// CommandCompleter 实现了命令自动补全的通用逻辑
type CommandCompleter struct {
	// 允许将来添加不同模式特定的命令
	SpecificCommands []string
}

// NewCommandCompleter 创建一个新的命令补全器
func NewCommandCompleter(specificCommands []string) *CommandCompleter {
	return &CommandCompleter{
		SpecificCommands: specificCommands,
	}
}

// DoComplete 实现通用的命令补全逻辑
func (c *CommandCompleter) DoComplete(line []rune, pos int) (newLine [][]rune, length int) {
	// 获取当前输入前缀
	lineStr := string(line[:pos])

	// 处理!命令的自动补全
	if len(lineStr) >= 1 && lineStr[0] == '!' {
		return c.completeShellCommands(lineStr[1:])
	}

	// 普通命令补全 - 只处理/开头的命令
	if len(lineStr) > 0 && lineStr[0] == '/' {
		// 通用命令 + 特定模式的命令
		commands := append([]string{"help"}, c.SpecificCommands...)
		prefix := lineStr[1:]

		// 根据前缀过滤命令
		var candidates [][]rune
		for _, cmd := range commands {
			if strings.HasPrefix(cmd, prefix) {
				candidates = append(candidates, []rune(cmd))
			}
		}

		if len(candidates) == 0 {
			return nil, 0
		}

		// 返回前缀长度，保留/前缀
		return candidates, len(prefix)
	}

	return nil, 0
}

// completeShellCommands 完成shell命令的自动补全
func (c *CommandCompleter) completeShellCommands(cmdPrefix string) (newLine [][]rune, length int) {
	// 获取系统中所有的可执行命令
	pathEnv := os.Getenv("PATH")
	if pathEnv == "" {
		return nil, 0
	}

	paths := strings.Split(pathEnv, ":")
	var matchingCommands []string

	// 检查每个PATH目录下的可执行文件
	for _, path := range paths {
		files, err := os.ReadDir(path)
		if err != nil {
			continue
		}

		for _, file := range files {
			// 跳过目录
			if file.IsDir() {
				continue
			}

			// 只匹配以cmdPrefix开头的文件
			fileName := file.Name()
			if strings.HasPrefix(fileName, cmdPrefix) {
				matchingCommands = append(matchingCommands, fileName)
			}

			// 限制候选数量，避免过多
			if len(matchingCommands) > 100 {
				break
			}
		}

		// 如果已经有足够多的候选项，就不再继续查找
		if len(matchingCommands) > 100 {
			break
		}
	}

	if len(matchingCommands) == 0 {
		return nil, 0
	}

	// 如果只有一个匹配项，返回需要追加的差异部分
	if len(matchingCommands) == 1 {
		// 只返回未输入的后缀部分
		suffix := matchingCommands[0][len(cmdPrefix):]
		return [][]rune{[]rune(suffix)}, 0 // 长度为0表示不替换任何内容，只追加
	}

	// 找到所有匹配项的共同前缀
	commonPrefix := matchingCommands[0]
	for _, cmd := range matchingCommands[1:] {
		i := 0
		for i < len(commonPrefix) && i < len(cmd) && commonPrefix[i] == cmd[i] {
			i++
		}
		commonPrefix = commonPrefix[:i]
	}

	// 如果共同前缀比已输入的前缀长，返回需要追加的部分
	if len(commonPrefix) > len(cmdPrefix) {
		// 只返回未输入的后缀部分
		suffix := commonPrefix[len(cmdPrefix):]
		return [][]rune{[]rune(suffix)}, 0 // 长度为0表示不替换任何内容，只追加
	}

	// 如果共同前缀不比已输入的前缀长，则显示所有匹配项
	// 为了兼容readline的行为，我们需要返回裸命令（没有!前缀）
	var candidates [][]rune
	for _, cmd := range matchingCommands {
		candidates = append(candidates, []rune(cmd))
	}

	// 返回前缀的长度，这样readline会替换掉当前命令部分
	return candidates, len(cmdPrefix)
}
