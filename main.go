package main

import (
	"embed"

	"github.com/phpToro/cli/cmd"
	"github.com/phpToro/cli/internal/ios"
	"github.com/phpToro/cli/internal/macos"
)

//go:embed all:embed
var embedFS embed.FS

func main() {
	ios.EmbedFS = embedFS
	ios.NativeFS = embedFS
	macos.EmbedFS = embedFS
	macos.NativeFS = embedFS
	cmd.Execute()
}
