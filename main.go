package main

import (
	"embed"

	"github.com/phpToro/cli/cmd"
	"github.com/phpToro/cli/internal/generator"
	"github.com/phpToro/cli/internal/project"
)

//go:embed all:embed
var embedFS embed.FS

func main() {
	project.EmbedFS = embedFS
	generator.StubFS = embedFS
	cmd.Execute()
}
