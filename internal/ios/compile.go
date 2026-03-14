package ios

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/phpToro/cli/internal/ui"
)

// EnsureNativeLibs compiles SAPI and extension libraries from runtimes source.
func EnsureNativeLibs(runtimeDir, runtimesSourceDir string) error {
	extDir := filepath.Join(runtimesSourceDir, "ext")
	if _, err := os.Stat(extDir); err != nil {
		return fmt.Errorf("runtimes source not found at %s", extDir)
	}

	sdk := sdkForRuntime(runtimeDir)
	cc, err := newCrossCompiler(sdk)
	if err != nil {
		return err
	}

	libDir := filepath.Join(runtimeDir, "lib")
	includeDir := filepath.Join(runtimeDir, "include")

	// PHP header include flags for SAPI/ext compilation
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
		ui.Dim("  Compiling SAPI...")
		obj := filepath.Join(tmpDir, "phptoro_sapi.o")
		if err := cc.compileC(filepath.Join(extDir, "phptoro_sapi.c"), obj, phpIncludes); err != nil {
			return nil, err
		}
		return []string{obj}, nil
	}); err != nil {
		return err
	}

	// 2. Compile libphptoro_ext.a (native_call bridge)
	if err := cc.ensureLib(libDir, "libphptoro_ext.a", func(tmpDir string) ([]string, error) {
		ui.Dim("  Compiling extension...")
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

// crossCompiler handles cross-compilation for iOS targets.
type crossCompiler struct {
	sdk     string
	sdkPath string
	flags   []string
}

func newCrossCompiler(sdk string) (*crossCompiler, error) {
	sdkPath, err := exec.Command("xcrun", "--sdk", sdk, "--show-sdk-path").Output()
	if err != nil {
		return nil, fmt.Errorf("xcrun sdk path: %w", err)
	}

	minVersion := "-mios-simulator-version-min=16.0"
	if sdk == "iphoneos" {
		minVersion = "-miphoneos-version-min=16.0"
	}

	return &crossCompiler{
		sdk:     sdk,
		sdkPath: strings.TrimSpace(string(sdkPath)),
		flags: []string{
			"-arch", "arm64",
			"-isysroot", strings.TrimSpace(string(sdkPath)),
			minVersion,
		},
	}, nil
}

func (cc *crossCompiler) compileC(src, obj string, extraFlags []string) error {
	args := []string{"--sdk", cc.sdk, "clang", "-c", src, "-o", obj}
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

func sdkForRuntime(runtimeDir string) string {
	if strings.Contains(runtimeDir, "ios-arm64-sim") {
		return "iphonesimulator"
	}
	return "iphoneos"
}
