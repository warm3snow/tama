package main

import (
	"fmt"
	"os"

	"github.com/warm3snow/tama/cmd"
)

func main() {
	// 执行命令
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}
