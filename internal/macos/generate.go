package macos

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/phpToro/cli/internal/plugin"
	"github.com/phpToro/cli/internal/ui"
)

// EmbedFS holds the embedded macos/ Swift layer files.
var EmbedFS embed.FS

// NativeFS holds the embedded native/ C header files.
var NativeFS embed.FS

// GenerateOptions configures the macOS project generation.
type GenerateOptions struct {
	ProjectRoot  string
	AppName      string
	BundleID     string
	DeployTarget string // macOS deployment target (e.g., "12.0")
	RuntimeDir   string // e.g. ~/.phptoro/runtimes/php-8.5.3-macos-arm64
}

// Generate creates the macos/ directory for a phpToro project.
func Generate(opts GenerateOptions) error {
	macDir := filepath.Join(opts.ProjectRoot, "macos")
	targetDir := filepath.Join(macDir, opts.AppName)

	ui.Dim("  Generating macOS Xcode project...")

	// Clean previous generation
	os.RemoveAll(macDir)

	// Create directory structure
	dirs := []string{
		filepath.Join(macDir, opts.AppName+".xcodeproj"),
		filepath.Join(targetDir, "Bridge"),
		filepath.Join(targetDir, "Engine"),
		filepath.Join(targetDir, "Plugins"),
		filepath.Join(targetDir, "Headers"),
		filepath.Join(macDir, "Libraries"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
	}

	// 1. Copy Swift layer files from embedded FS (skip assets/ — copied separately)
	if err := copyEmbeddedDirSkipping(EmbedFS, "embed/macos", targetDir, "Engine/assets"); err != nil {
		return fmt.Errorf("copy swift layer: %w", err)
	}

	// 2. Resolve plugins and copy their Swift files
	plugins, err := plugin.Resolve(opts.ProjectRoot)
	if err != nil {
		ui.Warning(fmt.Sprintf("  Plugin resolution failed: %s", err))
		plugins = nil
	}

	// Filter to plugins that declare macOS support.
	// Plugins with only "ios" platform use UIKit and won't compile on macOS.
	var macPlugins []plugin.ResolvedPlugin
	for _, p := range plugins {
		if _, ok := p.Manifest.Platforms["macos"]; ok {
			macPlugins = append(macPlugins, p)
		}
	}
	plugins = macPlugins

	var pluginFiles []pbxFile
	var pluginFrameworks []string

	for _, p := range plugins {
		macFiles := p.MacOSFiles()
		for _, src := range macFiles {
			fileName := filepath.Base(src)
			dst := filepath.Join(targetDir, "Plugins", fileName)
			if err := copyFile(src, dst); err != nil {
				return fmt.Errorf("copy plugin file %s: %w", src, err)
			}
			pluginFiles = append(pluginFiles, pbxFile{
				Name:  fileName,
				Path:  opts.AppName + "/Plugins/" + fileName,
				Group: "Plugins",
				Type:  "sourcecode.swift",
			})
		}
		pluginFrameworks = append(pluginFrameworks, p.MacOSFrameworks()...)
	}

	// 2b. Collect plugin assets (JS/CSS)
	pluginJS, pluginCSS := plugin.CollectMacOSAssets(plugins)
	var pluginAssetNames []string
	for _, a := range pluginJS {
		pluginAssetNames = append(pluginAssetNames, a.BundleName)
	}
	for _, a := range pluginCSS {
		pluginAssetNames = append(pluginAssetNames, a.BundleName)
	}

	// 3. Generate PhpToroApp.swift with plugin registration
	phpToroAppPath := filepath.Join(targetDir, "Engine", "PhpToroApp.swift")
	phpToroApp := generatePhpToroApp(plugins, pluginAssetNames)
	if err := os.WriteFile(phpToroAppPath, []byte(phpToroApp), 0644); err != nil {
		return fmt.Errorf("write PhpToroApp.swift: %w", err)
	}

	// 4. Copy C headers for bridging
	if err := copyEmbeddedDir(NativeFS, "embed/native", filepath.Join(targetDir, "Headers")); err != nil {
		return fmt.Errorf("copy native headers: %w", err)
	}

	// 5. Copy phptoro_ext.h from runtime includes
	extHeaderSrc := filepath.Join(opts.RuntimeDir, "include", "phptoro_ext.h")
	if err := copyFile(extHeaderSrc, filepath.Join(targetDir, "Headers", "phptoro_ext.h")); err != nil {
		return fmt.Errorf("copy phptoro_ext.h: %w", err)
	}

	// 6. Symlink PHP headers directory from runtime
	phpHeadersSrc := filepath.Join(opts.RuntimeDir, "include", "php")
	phpHeadersDst := filepath.Join(targetDir, "Headers", "php")
	if err := os.Symlink(phpHeadersSrc, phpHeadersDst); err != nil {
		return fmt.Errorf("symlink php headers: %w", err)
	}

	// 7. Symlink runtime libraries
	if err := symlinkLibraries(opts.RuntimeDir, filepath.Join(macDir, "Libraries")); err != nil {
		return fmt.Errorf("symlink libraries: %w", err)
	}

	// 8. Create bridging header
	bridgingHeader := fmt.Sprintf(`#ifndef %s_Bridging_Header_h
#define %s_Bridging_Header_h

#include "phptoro_sapi.h"

// Native handler (subset of phptoro_ext.h, avoids PHP header dependency)
typedef char *(*phptoro_native_handler_t)(const char *ns, const char *method, const char *args_json);
void phptoro_set_native_handler(phptoro_native_handler_t handler);

#endif
`, opts.AppName, opts.AppName)
	bridgingPath := filepath.Join(targetDir, opts.AppName+"-Bridging-Header.h")
	if err := os.WriteFile(bridgingPath, []byte(bridgingHeader), 0644); err != nil {
		return err
	}

	// 9. Create Info.plist (macOS)
	permissions := plugin.CollectMacOSPermissions(plugins)
	infoPlist := generateInfoPlist(opts.AppName, opts.BundleID, permissions)
	if err := os.WriteFile(filepath.Join(targetDir, "Info.plist"), []byte(infoPlist), 0644); err != nil {
		return err
	}

	// 9b. Generate entitlements
	entPlist := generateEntitlements()
	entPath := filepath.Join(targetDir, opts.AppName+".entitlements")
	if err := os.WriteFile(entPath, []byte(entPlist), 0644); err != nil {
		return err
	}

	// 10. Copy asset files into bundle
	assetsDir := filepath.Join(targetDir, "assets")
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		return fmt.Errorf("mkdir assets: %w", err)
	}
	// Copy embedded framework assets (phptoro.js, phptoro.macos.css)
	if err := copyEmbeddedDir(EmbedFS, "embed/macos/Engine/assets", assetsDir); err != nil {
		return fmt.Errorf("copy framework assets: %w", err)
	}
	// Copy plugin assets
	for _, a := range pluginJS {
		if err := copyFile(a.SrcPath, filepath.Join(assetsDir, a.BundleName)); err != nil {
			return fmt.Errorf("copy plugin js %s: %w", a.BundleName, err)
		}
	}
	for _, a := range pluginCSS {
		if err := copyFile(a.SrcPath, filepath.Join(assetsDir, a.BundleName)); err != nil {
			return fmt.Errorf("copy plugin css %s: %w", a.BundleName, err)
		}
	}
	// Copy developer assets
	devAssetsDir := filepath.Join(opts.ProjectRoot, "assets")
	for _, name := range []string{"app.css", "app.js"} {
		src := filepath.Join(devAssetsDir, name)
		if err := copyFile(src, filepath.Join(assetsDir, name)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("copy developer %s: %w", name, err)
		}
	}

	// 11. Copy PHP app files into bundle
	appDst := filepath.Join(targetDir, "app")
	if err := os.MkdirAll(appDst, 0755); err != nil {
		return err
	}
	phpItems := []string{"index.php", "app", "vendor", "composer.json"}
	for _, item := range phpItems {
		src := filepath.Join(opts.ProjectRoot, item)
		if _, err := os.Stat(src); err == nil {
			dst := filepath.Join(appDst, item)
			if err := copyPath(src, dst); err != nil {
				return fmt.Errorf("copy %s: %w", item, err)
			}
		}
	}

	// 12. Generate project.pbxproj
	pbxContent := generatePbxproj(pbxOptions{
		AppName:          opts.AppName,
		BundleID:         opts.BundleID,
		DeploymentTarget: opts.DeployTarget,
		PluginFiles:      pluginFiles,
		PluginFrameworks: pluginFrameworks,
	})
	pbxPath := filepath.Join(macDir, opts.AppName+".xcodeproj", "project.pbxproj")
	if err := os.WriteFile(pbxPath, []byte(pbxContent), 0644); err != nil {
		return err
	}

	if len(plugins) > 0 {
		ui.Dim(fmt.Sprintf("  %d plugin(s) integrated", len(plugins)))
	}
	ui.Dim("  Generated macos/ directory")
	return nil
}

// generatePhpToroApp creates PhpToroApp.swift for macOS.
func generatePhpToroApp(plugins []plugin.ResolvedPlugin, pluginAssets []string) string {
	var imports []string
	var registrations []string

	importSet := map[string]bool{"AppKit": true}
	for _, p := range plugins {
		for _, fw := range p.MacOSFrameworks() {
			if !importSet[fw] {
				importSet[fw] = true
				imports = append(imports, fw)
			}
		}
	}

	for _, p := range plugins {
		mac, ok := p.Manifest.Platforms["macos"]
		if !ok {
			continue
		}
		sources := mac.Handlers
		if len(sources) == 0 {
			sources = mac.Files
		}
		for _, f := range sources {
			base := filepath.Base(f)
			className := strings.TrimSuffix(base, filepath.Ext(base))
			registrations = append(registrations, fmt.Sprintf("        host.register(%s())", className))
		}
	}

	importLines := "import AppKit\n"
	for _, fw := range imports {
		importLines += "import " + fw + "\n"
	}

	var assetsLiteral string
	if len(pluginAssets) == 0 {
		assetsLiteral = "[]"
	} else {
		var items []string
		for _, a := range pluginAssets {
			items = append(items, fmt.Sprintf("        \"%s\"", a))
		}
		assetsLiteral = "[\n" + strings.Join(items, ",\n") + "\n    ]"
	}

	var b strings.Builder
	b.WriteString(importLines)
	b.WriteString(`
/// Entry point for a phpToro macOS app.
/// Generated by the phpToro CLI — do not edit manually.
final class PhpToroApp {
    static let pluginAssets: [String] = `)
	b.WriteString(assetsLiteral)
	b.WriteString(`

    static var logSink: ((String, String) -> Void)?

    private(set) var coordinator: AppCoordinator?
    private var hotReloadClient: HotReloadClient?

    func launch(in window: NSWindow) {
        dbg.configure()
        dbg.log("App", "launch() starting")

        var documentRoot = Bundle.main.path(forResource: "app", ofType: nil)
            ?? Bundle.main.bundlePath

        #if DEBUG
        if let devRoot = devDocumentRoot() {
            documentRoot = devRoot
        }
        #endif

        let dataDir = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask).first?.path
            ?? NSTemporaryDirectory()

        dbg.log("App", "documentRoot: \(documentRoot)")
        dbg.log("App", "dataDir: \(dataDir)")

        registerPlugins()

        let coordinator = AppCoordinator(window: window, documentRoot: documentRoot)
        coordinator.start(dataDir: dataDir)
        self.coordinator = coordinator

        wireAsyncCallbacks()

        #if DEBUG
        startHotReload()
        #endif

        dbg.log("App", "launch() complete")
    }

    // MARK: - Plugins (auto-generated)

    private func registerPlugins() {
        let host = PluginHost.shared
        host.register(StateHandler())
        host.register(LinkingHandler())
        host.register(PlatformHandler())
`)

	for _, line := range registrations {
		b.WriteString(line + "\n")
	}

	b.WriteString(`    }

    private func wireAsyncCallbacks() {
        dbg.log("App", "wireAsyncCallbacks() starting")
        PluginHost.shared.wireAsyncCallbacks { [weak self] ref, data in
            dbg.log("App", "async callback received: ref=\(ref), data=\(String(describing: data))")
            DispatchQueue.main.async {
                let vc = self?.coordinator?.currentScreenVC()
                dbg.log("App", "dispatching callback to VC: \(vc == nil ? "nil" : "found")")
                vc?.executeCallback(ref: ref, data: data)
            }
        }
        dbg.log("App", "wireAsyncCallbacks() complete")
    }

    // MARK: - Dev Mode

    private func devDocumentRoot() -> String? {
        guard let configPath = Bundle.main.path(forResource: "phptoro_dev", ofType: "json", inDirectory: "app"),
              let data = try? Data(contentsOf: URL(fileURLWithPath: configPath)),
              let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              let root = json["projectRoot"] as? String else {
            return nil
        }
        // macOS: filesystem is always accessible — use live source
        if FileManager.default.fileExists(atPath: root + "/index.php") {
            NSLog("[phpToro] Dev mode: using source at %@", root)
            return root
        }
        return nil
    }

    private func startHotReload() {
        let client = HotReloadClient.fromBundleConfig()
        client.onReload = { [weak self] _, _ in
            DispatchQueue.main.async {
                self?.coordinator?.rerenderCurrentScreen()
            }
        }
        client.onReloadConfig = { [weak self] in
            DispatchQueue.main.async {
                self?.coordinator?.rerenderCurrentScreen()
            }
        }
        client.connect()
        hotReloadClient = client

        dbg.remoteSink = { [weak client] level, message in
            client?.sendLog(level: level, message: message)
        }
        PhpToroApp.logSink = { [weak client] level, message in
            client?.sendLog(level: level, message: message)
        }
    }
}
`)

	return b.String()
}

func generateInfoPlist(appName, bundleID string, permissions []plugin.Permission) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleDevelopmentRegion</key>
	<string>$(DEVELOPMENT_LANGUAGE)</string>
	<key>CFBundleDisplayName</key>
`)
	b.WriteString(fmt.Sprintf("\t<string>%s</string>\n", appName))
	b.WriteString(`	<key>CFBundleExecutable</key>
	<string>$(EXECUTABLE_NAME)</string>
	<key>CFBundleIdentifier</key>
	<string>$(PRODUCT_BUNDLE_IDENTIFIER)</string>
	<key>CFBundleInfoDictionaryVersion</key>
	<string>6.0</string>
	<key>CFBundleName</key>
	<string>$(PRODUCT_NAME)</string>
	<key>CFBundlePackageType</key>
	<string>$(PRODUCT_BUNDLE_PACKAGE_TYPE)</string>
	<key>CFBundleShortVersionString</key>
	<string>1.0</string>
	<key>CFBundleVersion</key>
	<string>1</string>
	<key>LSMinimumSystemVersion</key>
	<string>$(MACOSX_DEPLOYMENT_TARGET)</string>
	<key>NSPrincipalClass</key>
	<string>NSApplication</string>
	<key>NSAppTransportSecurity</key>
	<dict>
		<key>NSAllowsLocalNetworking</key>
		<true/>
		<key>NSAllowsArbitraryLoads</key>
		<true/>
	</dict>
`)

	for _, perm := range permissions {
		b.WriteString(fmt.Sprintf("\t<key>%s</key>\n", perm.Key))
		b.WriteString(fmt.Sprintf("\t<string>%s</string>\n", perm.Description))
	}

	b.WriteString(`</dict>
</plist>
`)
	return b.String()
}

func generateEntitlements() string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>com.apple.security.app-sandbox</key>
	<false/>
</dict>
</plist>
`
}

// File copy helpers (same as iOS package)

func copyEmbeddedDirSkipping(fsys embed.FS, root string, destDir string, skip string) error {
	return fs.WalkDir(fsys, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if rel == skip || strings.HasPrefix(rel, skip+"/") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		destPath := filepath.Join(destDir, rel)
		if d.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}
		data, err := fsys.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(destPath, data, 0644)
	})
}

func copyEmbeddedDir(fsys embed.FS, root string, destDir string) error {
	return copyEmbeddedDirSkipping(fsys, root, destDir, "")
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

func copyPath(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return copyFile(src, dst)
	}
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := copyPath(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

func symlinkLibraries(runtimeDir, libDir string) error {
	runtimeLibDir := filepath.Join(runtimeDir, "lib")
	entries, err := os.ReadDir(runtimeLibDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".a" {
			src := filepath.Join(runtimeLibDir, e.Name())
			dst := filepath.Join(libDir, e.Name())
			if err := os.Symlink(src, dst); err != nil {
				return err
			}
		}
	}
	return nil
}
