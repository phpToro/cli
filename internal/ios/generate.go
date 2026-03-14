package ios

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

// EmbedFS holds the embedded ios/ Swift layer files.
// Set from main.go via: ios.EmbedFS = embedFS
var EmbedFS embed.FS

// NativeFS holds the embedded native/ C header files.
var NativeFS embed.FS

// GenerateOptions configures the iOS project generation.
type GenerateOptions struct {
	ProjectRoot  string // path to phptoro project
	AppName      string // from phptoro.json (used as Xcode target name)
	BundleID     string // from phptoro.json
	TeamID       string // from phptoro.json platforms.ios.teamId
	DeployTarget string // from phptoro.json platforms.ios.deploymentTarget
	RuntimeDir   string // e.g. ~/.phptoro/runtimes/php-8.5.3-ios-arm64-sim
}

// Generate creates the ios/ directory for a phpToro project.
func Generate(opts GenerateOptions) error {
	iosDir := filepath.Join(opts.ProjectRoot, "ios")
	targetDir := filepath.Join(iosDir, opts.AppName)

	ui.Dim("  Generating Xcode project...")

	// Clean previous generation
	os.RemoveAll(iosDir)

	// Create directory structure
	dirs := []string{
		filepath.Join(iosDir, opts.AppName+".xcodeproj"),
		filepath.Join(targetDir, "Bridge"),
		filepath.Join(targetDir, "Engine"),
		filepath.Join(targetDir, "Plugins"),
		filepath.Join(targetDir, "Headers"),
		filepath.Join(iosDir, "Libraries"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
	}

	// 1. Copy Swift layer files from embedded FS (skip assets/ — copied separately in step 10)
	if err := copyEmbeddedDirSkipping(EmbedFS, "embed/ios", targetDir, "Engine/assets"); err != nil {
		return fmt.Errorf("copy swift layer: %w", err)
	}

	// 2. Resolve plugins and copy their Swift files
	plugins, err := plugin.Resolve(opts.ProjectRoot)
	if err != nil {
		ui.Warning(fmt.Sprintf("  Plugin resolution failed: %s", err))
		plugins = nil
	}

	var pluginFiles []pbxFile
	var pluginFrameworks []string

	for _, p := range plugins {
		iosFiles := p.IOSFiles()
		for _, src := range iosFiles {
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
		pluginFrameworks = append(pluginFrameworks, p.IOSFrameworks()...)
	}

	// 2b. Collect plugin assets (JS/CSS)
	pluginJS, pluginCSS := plugin.CollectIOSAssets(plugins)
	var pluginAssetNames []string
	for _, a := range pluginJS {
		pluginAssetNames = append(pluginAssetNames, a.BundleName)
	}
	for _, a := range pluginCSS {
		pluginAssetNames = append(pluginAssetNames, a.BundleName)
	}

	// 3. Generate PhpToroApp.swift with plugin registration and asset list
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
	if err := symlinkLibraries(opts.RuntimeDir, filepath.Join(iosDir, "Libraries")); err != nil {
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

	// 9. Create Info.plist (with plugin permissions)
	permissions := plugin.CollectIOSPermissions(plugins)
	backgroundModes := plugin.CollectIOSBackgroundModes(plugins)
	infoPlist := generateInfoPlist(opts.AppName, opts.BundleID, permissions, backgroundModes)
	if err := os.WriteFile(filepath.Join(targetDir, "Info.plist"), []byte(infoPlist), 0644); err != nil {
		return err
	}

	// 9b. Generate entitlements if any plugin requires them
	entitlements := plugin.CollectIOSEntitlements(plugins)
	if len(entitlements) > 0 {
		entPlist := generateEntitlements(entitlements)
		entPath := filepath.Join(targetDir, opts.AppName+".entitlements")
		if err := os.WriteFile(entPath, []byte(entPlist), 0644); err != nil {
			return err
		}
	}

	// 10. Copy asset files into bundle
	assetsDir := filepath.Join(targetDir, "assets")
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		return fmt.Errorf("mkdir assets: %w", err)
	}
	// Copy embedded framework assets (phptoro.js, phptoro.ios.css)
	if err := copyEmbeddedDir(EmbedFS, "embed/ios/Engine/assets", assetsDir); err != nil {
		return fmt.Errorf("copy framework assets: %w", err)
	}
	// Copy plugin assets (JS and CSS) with namespaced filenames
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
	// Copy developer assets (assets/app.css, assets/app.js) if they exist
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
		TeamID:           opts.TeamID,
		PluginFiles:      pluginFiles,
		PluginFrameworks: pluginFrameworks,
	})
	pbxPath := filepath.Join(iosDir, opts.AppName+".xcodeproj", "project.pbxproj")
	if err := os.WriteFile(pbxPath, []byte(pbxContent), 0644); err != nil {
		return err
	}

	if len(plugins) > 0 {
		ui.Dim(fmt.Sprintf("  %d plugin(s) integrated", len(plugins)))
	}
	ui.Dim("  Generated ios/ directory")
	return nil
}

// generatePhpToroApp creates PhpToroApp.swift with plugin registration code.
func generatePhpToroApp(plugins []plugin.ResolvedPlugin, pluginAssets []string) string {
	var imports []string
	var registrations []string

	// Collect unique imports
	importSet := map[string]bool{"UIKit": true}
	for _, p := range plugins {
		for _, fw := range p.IOSFrameworks() {
			if !importSet[fw] {
				importSet[fw] = true
				imports = append(imports, fw)
			}
		}
	}

	// Build registration lines from plugin manifests
	for _, p := range plugins {
		ios, ok := p.Manifest.Platforms["ios"]
		if !ok {
			continue
		}
		// Prefer "handlers" field, fall back to "files"
		sources := ios.Handlers
		if len(sources) == 0 {
			sources = ios.Files
		}
		for _, f := range sources {
			// Extract class name from filename: CameraHandler.swift → CameraHandler
			base := filepath.Base(f)
			className := strings.TrimSuffix(base, filepath.Ext(base))
			registrations = append(registrations, fmt.Sprintf("        host.register(%s())", className))
		}
	}

	// Async callbacks are wired generically via the AsyncHandler protocol.
	// No per-plugin wiring needed — see wireAsyncCallbacks() in the generated code.

	// Build imports string
	importLines := "import UIKit\n"
	for _, fw := range imports {
		importLines += "import " + fw + "\n"
	}

	// Build plugin assets array literal
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

	// Build the file
	var b strings.Builder
	b.WriteString(importLines)
	b.WriteString(`
/// Entry point for a phpToro iOS app.
/// Generated by the phpToro CLI — do not edit manually.
final class PhpToroApp {
    /// Plugin asset filenames (JS + CSS), injected between framework and developer assets.
    static let pluginAssets: [String] = `)
	b.WriteString(assetsLiteral)
	b.WriteString(`

    /// Called by ScreenViewController when a JS console log is received.
    /// Set by startHotReload() to forward logs to the dev server.
    static var logSink: ((String, String) -> Void)?

    private(set) var coordinator: AppCoordinator?
    private var hotReloadClient: HotReloadClient?

    func launch(in window: UIWindow) {
        dbg.configure()
        dbg.log("App", "launch() starting")

        var documentRoot = Bundle.main.path(forResource: "app", ofType: nil)
            ?? Bundle.main.bundlePath

        #if DEBUG
        if let devRoot = devDocumentRoot() {
            documentRoot = devRoot
        }
        #endif

        let dataDir = NSSearchPathForDirectoriesInDomains(.documentDirectory, .userDomainMask, true).first
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

    /// Path to the writable app directory (for physical device hot reload).
    /// On simulator, this is nil (we read from Mac source directly).
    private var writableAppDir: String?

    private func devDocumentRoot() -> String? {
        guard let configPath = Bundle.main.path(forResource: "phptoro_dev", ofType: "json", inDirectory: "app"),
              let data = try? Data(contentsOf: URL(fileURLWithPath: configPath)),
              let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              let root = json["projectRoot"] as? String else {
            return nil
        }
        // Simulator: Mac filesystem is accessible — use live source
        if FileManager.default.fileExists(atPath: root + "/index.php") {
            NSLog("[phpToro] Dev mode: using source at %@", root)
            return root
        }
        // Physical device: create writable copy of bundled app/ for hot reload syncing
        return setupWritableAppDir()
    }

    /// Creates a writable copy of the bundled app/ directory in Documents/.
    /// Returns the path to use as document root.
    private func setupWritableAppDir() -> String? {
        guard let bundledApp = Bundle.main.path(forResource: "app", ofType: nil) else {
            return nil
        }
        let caches = FileManager.default.urls(for: .cachesDirectory, in: .userDomainMask).first!.path
        let devDir = caches + "/phptoro-dev-app"
        let fm = FileManager.default

        // Always start fresh from bundle to match the built version
        try? fm.removeItem(atPath: devDir)
        do {
            try fm.copyItem(atPath: bundledApp, toPath: devDir)
        } catch {
            NSLog("[phpToro] Failed to create writable app dir: %@", error.localizedDescription)
            return nil
        }

        writableAppDir = devDir
        NSLog("[phpToro] Dev mode (device): writable app dir at %@", devDir)
        return devDir
    }

    /// Write a synced file to the writable app directory (physical device hot reload).
    private func syncFile(relativePath: String, base64Content: String) {
        guard let dir = writableAppDir else { return }
        guard let data = Data(base64Encoded: base64Content) else {
            dbg.error("HotReload", "Failed to decode base64 for \(relativePath)")
            return
        }
        let filePath = dir + "/" + relativePath
        let parentDir = (filePath as NSString).deletingLastPathComponent
        let fm = FileManager.default
        try? fm.createDirectory(atPath: parentDir, withIntermediateDirectories: true, attributes: nil)
        do {
            try data.write(to: URL(fileURLWithPath: filePath))
            dbg.log("HotReload", "Synced \(relativePath) (\(data.count) bytes)")
        } catch {
            dbg.error("HotReload", "Failed to write \(relativePath): \(error)")
        }
    }

    private func startHotReload() {
        let client = HotReloadClient.fromBundleConfig()
        client.onReload = { [weak self] file, content in
            // If file content is provided (physical device), write it first
            if let file = file, let content = content {
                self?.syncFile(relativePath: file, base64Content: content)
            }
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

        // Forward all logs (native + JS) to the dev server
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

		// Skip the specified subdirectory
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

// copyPath copies a file or directory recursively.
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
		srcPath := filepath.Join(src, e.Name())
		dstPath := filepath.Join(dst, e.Name())
		if err := copyPath(srcPath, dstPath); err != nil {
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

func generateInfoPlist(appName, bundleID string, permissions []plugin.Permission, backgroundModes []string) string {
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
	<key>LSRequiresIPhoneOS</key>
	<true/>
	<key>UIApplicationSceneManifest</key>
	<dict>
		<key>UIApplicationSupportsMultipleScenes</key>
		<false/>
		<key>UISceneConfigurations</key>
		<dict>
			<key>UIWindowSceneSessionRoleApplication</key>
			<array>
				<dict>
					<key>UISceneConfigurationName</key>
					<string>Default Configuration</string>
					<key>UISceneDelegateClassName</key>
					<string>$(PRODUCT_MODULE_NAME).SceneDelegate</string>
				</dict>
			</array>
		</dict>
	</dict>
	<key>UILaunchScreen</key>
	<dict/>
	<key>UISupportedInterfaceOrientations</key>
	<array>
		<string>UIInterfaceOrientationPortrait</string>
	</array>
	<key>UIRequiredDeviceCapabilities</key>
	<array>
		<string>armv7</string>
	</array>
	<key>NSAppTransportSecurity</key>
	<dict>
		<key>NSAllowsLocalNetworking</key>
		<true/>
		<key>NSAllowsArbitraryLoads</key>
		<true/>
	</dict>
`)

	// Plugin permissions (e.g. NSCameraUsageDescription)
	for _, perm := range permissions {
		b.WriteString(fmt.Sprintf("\t<key>%s</key>\n", perm.Key))
		b.WriteString(fmt.Sprintf("\t<string>%s</string>\n", perm.Description))
	}

	// Background modes
	if len(backgroundModes) > 0 {
		b.WriteString("\t<key>UIBackgroundModes</key>\n")
		b.WriteString("\t<array>\n")
		for _, mode := range backgroundModes {
			b.WriteString(fmt.Sprintf("\t\t<string>%s</string>\n", mode))
		}
		b.WriteString("\t</array>\n")
	}

	b.WriteString(`</dict>
</plist>
`)
	return b.String()
}

func generateEntitlements(entitlements []string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
`)
	for _, ent := range entitlements {
		b.WriteString(fmt.Sprintf("\t<key>%s</key>\n", ent))
		b.WriteString("\t<true/>\n")
	}
	b.WriteString(`</dict>
</plist>
`)
	return b.String()
}
