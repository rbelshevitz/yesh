package main

import (
	"os"

	"yesh/internal/yeshcore"
)

func main() {
	os.Exit(yeshcore.RunCLI())
}
