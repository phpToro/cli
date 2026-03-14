package main

import (
	"embed"

	"github.com/phpToro/cli/cmd"
	"github.com/phpToro/cli/internal/ios"
)

//go:embed all:embed
var embedFS embed.FS

func main() {
	ios.EmbedFS = embedFS
	ios.NativeFS = embedFS
	cmd.Execute()
}
