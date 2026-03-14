package macos

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/phpToro/cli/internal/ui"
)

// EnsureNativeLibs compiles SAPI and extension libraries for macOS.
func EnsureNativeLibs(runtimeDir, runtimesSourceDir string) error {
	extDir := filepath.Join(runtimesSourceDir, "ext")
	if _, err := os.Stat(extDir); err != nil {
		return fmt.Errorf("runtimes source not found at %s", extDir)
	}

	cc, err := newCrossCompiler()
	if err != nil {
		return err
	}

	libDir := filepath.Join(runtimeDir, "lib")
	includeDir := filepath.Join(runtimeDir, "include")

	phpIncludes := []string{
		"-I" + filepath.Join(includeDir, "php", "main"),
		"-I" + filepath.Join(includeDir, "php", "Zend"),
		"-I" + filepath.Join(includeDir, "php", "TSRM"),
		"-I" + filepath.Join(includeDir, "php", "ext"),
		"-I" + filepath.Join(includeDir, "php", "sapi"),
		"-I" + filepath.Join(includeDir, "php", "sapi", "embed"),
		"-I" + filepath.Join(includeDir, "php"),
		"-I" + extDir,
	}

	// 1. Compile libphptoro_sapi.a
	if err := cc.ensureLib(libDir, "libphptoro_sapi.a", func(tmpDir string) ([]string, error) {
		ui.Dim("  Compiling SAPI (macOS)...")
		obj := filepath.Join(tmpDir, "phptoro_sapi.o")
		if err := cc.compileC(filepath.Join(extDir, "phptoro_sapi.c"), obj, phpIncludes); err != nil {
			return nil, err
		}
		return []string{obj}, nil
	}); err != nil {
		return err
	}

	// 2. Compile libphptoro_ext.a
	if err := cc.ensureLib(libDir, "libphptoro_ext.a", func(tmpDir string) ([]string, error) {
		ui.Dim("  Compiling extension (macOS)...")
		var objs []string
		for _, src := range []string{"phptoro_ext.c", "phptoro_phpinfo.c"} {
			obj := filepath.Join(tmpDir, strings.TrimSuffix(src, ".c")+".o")
			if err := cc.compileC(filepath.Join(extDir, src), obj, phpIncludes); err != nil {
				return nil, err
			}
			objs = append(objs, obj)
		}
		return objs, nil
	}); err != nil {
		return err
	}

	return nil
}

type crossCompiler struct {
	sdkPath string
	flags   []string
}

func newCrossCompiler() (*crossCompiler, error) {
	sdkPath, err := exec.Command("xcrun", "--sdk", "macosx", "--show-sdk-path").Output()
	if err != nil {
		return nil, fmt.Errorf("xcrun sdk path: %w", err)
	}

	return &crossCompiler{
		sdkPath: strings.TrimSpace(string(sdkPath)),
		flags: []string{
			"-arch", "arm64",
			"-isysroot", strings.TrimSpace(string(sdkPath)),
			"-mmacosx-version-min=12.0",
		},
	}, nil
}

func (cc *crossCompiler) compileC(src, obj string, extraFlags []string) error {
	args := []string{"--sdk", "macosx", "clang", "-c", src, "-o", obj}
	args = append(args, extraFlags...)
	args = append(args, cc.flags...)

	cmd := exec.Command("xcrun", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("compile %s: %s — %w", filepath.Base(src), string(out), err)
	}
	return nil
}

func (cc *crossCompiler) ensureLib(libDir, libName string, buildFn func(tmpDir string) ([]string, error)) error {
	libPath := filepath.Join(libDir, libName)

	tmpDir, err := os.MkdirTemp("", "phptoro-"+libName+"-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	objs, err := buildFn(tmpDir)
	if err != nil {
		return err
	}

	os.Remove(libPath)

	args := append([]string{"rcs", libPath}, objs...)
	cmd := exec.Command("ar", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ar %s: %s — %w", libName, string(out), err)
	}

	ui.Dim("  Compiled " + libName)
	return nil
}
