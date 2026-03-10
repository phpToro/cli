package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/phpToro/cli/internal/ui"
)

type TemplateData struct {
	AppName      string
	AppNameLower string
}

var dirs = []string{
	"app/Screens/templates",
	"app/Components/templates",
	"tests/Screens",
	"assets/fonts",
	"assets/store",
}

type fileMapping struct {
	tmpl string
	dest string
}

var defaultFiles = []fileMapping{
	{"embed/templates/default/phptoro.json.tmpl", "phptoro.json"},
	{"embed/templates/default/composer.json.tmpl", "composer.json"},
	{"embed/templates/default/index.php.tmpl", "index.php"},
	{"embed/templates/default/App.php.tmpl", "app/App.php"},
	{"embed/templates/default/HomeScreen.php.tmpl", "app/Screens/HomeScreen.php"},
	{"embed/templates/default/HomeScreen.toro.tmpl", "app/Screens/templates/HomeScreen.toro"},
}

var tabsFiles = []fileMapping{
	{"embed/templates/tabs/phptoro.json.tmpl", "phptoro.json"},
	{"embed/templates/default/composer.json.tmpl", "composer.json"},
	{"embed/templates/default/index.php.tmpl", "index.php"},
	{"embed/templates/tabs/App.php.tmpl", "app/App.php"},
	{"embed/templates/tabs/HomeScreen.php.tmpl", "app/Screens/HomeScreen.php"},
	{"embed/templates/tabs/SettingsScreen.php.tmpl", "app/Screens/SettingsScreen.php"},
}

func Scaffold(name string, tmplName string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("project name is required")
	}

	projectDir := filepath.Join(".", name)
	if _, err := os.Stat(projectDir); err == nil {
		return "", fmt.Errorf("directory %q already exists", name)
	}

	data := TemplateData{
		AppName:      name,
		AppNameLower: strings.ToLower(name),
	}

	// Create directories
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(projectDir, d), 0755); err != nil {
			return "", fmt.Errorf("failed to create %s: %w", d, err)
		}
	}

	// Select template set
	files := defaultFiles
	if tmplName == "tabs" {
		files = tabsFiles
	}

	// Render files
	for _, f := range files {
		if err := renderTemplate(projectDir, f.tmpl, f.dest, data); err != nil {
			return "", fmt.Errorf("failed to create %s: %w", f.dest, err)
		}
		ui.Success("Created " + f.dest)
	}

	// Create .gitignore
	gitignore := "vendor/\nios/\nandroid/\n.phptoro-cache/\n"
	if err := os.WriteFile(filepath.Join(projectDir, ".gitignore"), []byte(gitignore), 0644); err != nil {
		return "", err
	}
	ui.Success("Created .gitignore")

	absPath, _ := filepath.Abs(projectDir)
	return absPath, nil
}

func renderTemplate(projectDir, tmplPath, destPath string, data TemplateData) error {
	content, err := EmbedFS.ReadFile(tmplPath)
	if err != nil {
		return fmt.Errorf("template not found: %s: %w", tmplPath, err)
	}

	t, err := template.New(filepath.Base(tmplPath)).Parse(string(content))
	if err != nil {
		return fmt.Errorf("invalid template: %w", err)
	}

	dest := filepath.Join(projectDir, destPath)
	os.MkdirAll(filepath.Dir(dest), 0755)

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	return t.Execute(f, data)
}
