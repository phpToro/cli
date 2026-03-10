package generator

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
	"unicode"

	"github.com/phpToro/cli/internal/config"
	"github.com/phpToro/cli/internal/ui"
)

// StubFS is set by main to inject the embedded stubs filesystem.
var StubFS embed.FS

type StubData struct {
	Name string
}

func GenerateScreen(name string) error {
	if err := validateName(name); err != nil {
		return err
	}

	root, err := config.FindProjectRoot()
	if err != nil {
		return err
	}

	data := StubData{Name: name}

	phpDest := filepath.Join(root, "app", "Screens", name+"Screen.php")
	toroDest := filepath.Join(root, "app", "Screens", "templates", name+"Screen.toro")

	if err := renderStub("embed/stubs/screen.php.tmpl", phpDest, data); err != nil {
		return err
	}
	ui.Success("Created app/Screens/" + name + "Screen.php")

	if err := renderStub("embed/stubs/screen.toro.tmpl", toroDest, data); err != nil {
		return err
	}
	ui.Success("Created app/Screens/templates/" + name + "Screen.toro")

	return nil
}

func GenerateComponent(name string) error {
	if err := validateName(name); err != nil {
		return err
	}

	root, err := config.FindProjectRoot()
	if err != nil {
		return err
	}

	data := StubData{Name: name}

	phpDest := filepath.Join(root, "app", "Components", name+".php")
	toroDest := filepath.Join(root, "app", "Components", "templates", name+".toro")

	if err := renderStub("embed/stubs/component.php.tmpl", phpDest, data); err != nil {
		return err
	}
	ui.Success("Created app/Components/" + name + ".php")

	if err := renderStub("embed/stubs/component.toro.tmpl", toroDest, data); err != nil {
		return err
	}
	ui.Success("Created app/Components/templates/" + name + ".toro")

	return nil
}

func validateName(name string) error {
	if name == "" {
		return fmt.Errorf("name is required")
	}
	if !unicode.IsUpper(rune(name[0])) {
		return fmt.Errorf("name must be PascalCase (got %q)", name)
	}
	for _, r := range name {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return fmt.Errorf("name must contain only letters and digits (got %q)", name)
		}
	}
	return nil
}

func renderStub(stubPath, destPath string, data StubData) error {
	if _, err := os.Stat(destPath); err == nil {
		return fmt.Errorf("%s already exists", destPath)
	}

	content, err := StubFS.ReadFile(stubPath)
	if err != nil {
		return fmt.Errorf("stub not found: %s", stubPath)
	}

	t, err := template.New(filepath.Base(stubPath)).Parse(string(content))
	if err != nil {
		return err
	}

	os.MkdirAll(filepath.Dir(destPath), 0755)

	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer f.Close()

	return t.Execute(f, data)
}
