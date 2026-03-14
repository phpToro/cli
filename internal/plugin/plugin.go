package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// PluginsDir is where plugins are cached globally.
func PluginsDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".phptoro", "plugins")
}

// Manifest represents a plugin's plugin.json file.
type Manifest struct {
	Name      string              `json:"name"`
	Namespace string              `json:"namespace"`
	Platforms map[string]Platform  `json:"platforms"`
	JS        []string            `json:"js,omitempty"`  // cross-platform JS files (e.g. "js/camera.js")
	PHP       *PHPConfig          `json:"php,omitempty"`
}

// Platform describes the native files and requirements for a platform.
type Platform struct {
	Files       []string     `json:"files"`
	Handlers    []string     `json:"handlers,omitempty"`   // native handler files (e.g. "ios/CameraHandler.swift")
	CSS         []string     `json:"css,omitempty"`         // platform CSS files (e.g. "ios/camera.css")
	Frameworks  []string     `json:"frameworks,omitempty"`
	Permissions []Permission `json:"permissions,omitempty"`
	Entitlements []string    `json:"entitlements,omitempty"`
	BackgroundModes []string `json:"backgroundModes,omitempty"`
}

// Permission declares a required permission with its usage description.
type Permission struct {
	Key         string `json:"key"`         // e.g. "NSCameraUsageDescription"
	Description string `json:"description"` // User-facing reason (iOS requires this)
}

// PHPConfig describes optional PHP helper files.
type PHPConfig struct {
	Files []string `json:"files"`
}

// Installed represents a plugin entry in phptoro.json.
type Installed struct {
	Repo    string `json:"repo"`    // e.g. "phpToro/plugin-storage"
	Version string `json:"version"` // e.g. "main" or "v1.0.0"
}

// ResolvedPlugin is a fully resolved plugin ready for integration.
type ResolvedPlugin struct {
	Manifest  Manifest
	LocalPath string // path to the plugin on disk
	Repo      string
}

// Add downloads a plugin from GitHub and adds it to phptoro.json.
func Add(projectRoot, repo string) error {
	localPath := pluginLocalPath(repo)

	// Clone or update
	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		if err := gitClone(repo, localPath); err != nil {
			return fmt.Errorf("failed to clone %s: %w", repo, err)
		}
	} else {
		if err := gitPull(localPath); err != nil {
			return fmt.Errorf("failed to update %s: %w", repo, err)
		}
	}

	// Validate plugin.json exists
	manifest, err := LoadManifest(localPath)
	if err != nil {
		// Clean up invalid plugin
		os.RemoveAll(localPath)
		return fmt.Errorf("%s is not a valid phpToro plugin (missing or invalid plugin.json): %w", repo, err)
	}

	// Add to phptoro.json
	if err := addToConfig(projectRoot, repo); err != nil {
		return err
	}

	// Copy PHP files to vendor/plugins/<namespace>/ if they exist
	if manifest.PHP != nil && len(manifest.PHP.Files) > 0 {
		vendorDir := filepath.Join(projectRoot, "vendor", "plugins", manifest.Namespace)
		os.MkdirAll(vendorDir, 0755)
		for _, f := range manifest.PHP.Files {
			src := filepath.Join(localPath, f)
			dst := filepath.Join(vendorDir, filepath.Base(f))
			data, err := os.ReadFile(src)
			if err != nil {
				return fmt.Errorf("copy php file %s: %w", f, err)
			}
			if err := os.WriteFile(dst, data, 0644); err != nil {
				return err
			}
		}
	}

	return nil
}

// Remove removes a plugin from phptoro.json and cleans up PHP files.
func Remove(projectRoot, repo string) error {
	// Load manifest to get namespace for cleanup
	localPath := pluginLocalPath(repo)
	if manifest, err := LoadManifest(localPath); err == nil {
		// Remove vendor PHP files
		vendorDir := filepath.Join(projectRoot, "vendor", "plugins", manifest.Namespace)
		os.RemoveAll(vendorDir)
	}

	return removeFromConfig(projectRoot, repo)
}

// List returns all plugins from phptoro.json.
func List(projectRoot string) ([]Installed, error) {
	return loadPluginsFromConfig(projectRoot)
}

// Resolve loads the manifest and local path for each installed plugin.
// Checks local paths first (project plugins/, sibling plugins/), then falls back to ~/.phptoro/plugins/.
func Resolve(projectRoot string) ([]ResolvedPlugin, error) {
	installed, err := loadPluginsFromConfig(projectRoot)
	if err != nil {
		return nil, err
	}

	var resolved []ResolvedPlugin
	for _, p := range installed {
		localPath := resolvePluginPath(projectRoot, p.Repo)
		manifest, err := LoadManifest(localPath)
		if err != nil {
			return nil, fmt.Errorf("plugin %s: %w", p.Repo, err)
		}
		resolved = append(resolved, ResolvedPlugin{
			Manifest:  *manifest,
			LocalPath: localPath,
			Repo:      p.Repo,
		})
	}
	return resolved, nil
}

// resolvePluginPath finds a plugin on disk, checking local paths before the global cache.
// For repo "phpToro/plugin-camera", checks:
//  1. {projectRoot}/plugins/plugin-camera/
//  2. {projectRoot}/../plugins/plugin-camera/  (monorepo sibling)
//  3. ~/.phptoro/plugins/phpToro/plugin-camera/ (global cache)
func resolvePluginPath(projectRoot, repo string) string {
	// Extract plugin name from repo (e.g. "phpToro/plugin-camera" → "plugin-camera")
	parts := strings.Split(repo, "/")
	pluginName := parts[len(parts)-1]

	// Check local project plugins/ directory
	candidates := []string{
		filepath.Join(projectRoot, "plugins", pluginName),
		filepath.Join(projectRoot, "..", "plugins", pluginName),
	}

	for _, c := range candidates {
		if _, err := os.Stat(filepath.Join(c, "plugin.json")); err == nil {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}

	// Fall back to global cache
	return pluginLocalPath(repo)
}

// LoadManifest reads plugin.json from a plugin directory.
func LoadManifest(pluginDir string) (*Manifest, error) {
	data, err := os.ReadFile(filepath.Join(pluginDir, "plugin.json"))
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if m.Namespace == "" {
		return nil, fmt.Errorf("plugin.json missing 'namespace' field")
	}
	return &m, nil
}

// IOSFiles returns the Swift file paths for a resolved plugin.
// Checks both "handlers" (new) and "files" (legacy) fields.
func (p *ResolvedPlugin) IOSFiles() []string {
	platform, ok := p.Manifest.Platforms["ios"]
	if !ok {
		return nil
	}
	// Prefer "handlers" field, fall back to "files"
	sources := platform.Handlers
	if len(sources) == 0 {
		sources = platform.Files
	}
	var paths []string
	for _, f := range sources {
		paths = append(paths, filepath.Join(p.LocalPath, f))
	}
	return paths
}

// IOSCSS returns the platform CSS file paths for a resolved plugin.
func (p *ResolvedPlugin) IOSCSS() []string {
	platform, ok := p.Manifest.Platforms["ios"]
	if !ok {
		return nil
	}
	var paths []string
	for _, f := range platform.CSS {
		paths = append(paths, filepath.Join(p.LocalPath, f))
	}
	return paths
}

// CrossPlatformJS returns the cross-platform JS file paths for a resolved plugin.
func (p *ResolvedPlugin) CrossPlatformJS() []string {
	var paths []string
	for _, f := range p.Manifest.JS {
		paths = append(paths, filepath.Join(p.LocalPath, f))
	}
	return paths
}

// IOSFrameworks returns additional frameworks needed by this plugin.
func (p *ResolvedPlugin) IOSFrameworks() []string {
	platform, ok := p.Manifest.Platforms["ios"]
	if !ok {
		return nil
	}
	return platform.Frameworks
}

// IOSPermissions returns iOS permissions (Info.plist keys) needed by this plugin.
func (p *ResolvedPlugin) IOSPermissions() []Permission {
	platform, ok := p.Manifest.Platforms["ios"]
	if !ok {
		return nil
	}
	return platform.Permissions
}

// IOSEntitlements returns iOS entitlements needed by this plugin.
func (p *ResolvedPlugin) IOSEntitlements() []string {
	platform, ok := p.Manifest.Platforms["ios"]
	if !ok {
		return nil
	}
	return platform.Entitlements
}

// IOSBackgroundModes returns UIBackgroundModes needed by this plugin.
func (p *ResolvedPlugin) IOSBackgroundModes() []string {
	platform, ok := p.Manifest.Platforms["ios"]
	if !ok {
		return nil
	}
	return platform.BackgroundModes
}

// MacOSFiles returns the handler file paths for the "macos" platform.
func (p *ResolvedPlugin) MacOSFiles() []string {
	platform, ok := p.Manifest.Platforms["macos"]
	if !ok {
		return nil
	}
	sources := platform.Handlers
	if len(sources) == 0 {
		sources = platform.Files
	}
	var paths []string
	for _, f := range sources {
		paths = append(paths, filepath.Join(p.LocalPath, f))
	}
	return paths
}

// MacOSFrameworks returns additional frameworks needed by this plugin on macOS.
func (p *ResolvedPlugin) MacOSFrameworks() []string {
	platform, ok := p.Manifest.Platforms["macos"]
	if !ok {
		return nil
	}
	return platform.Frameworks
}

// MacOSPermissions returns macOS permissions (Info.plist keys) needed by this plugin.
func (p *ResolvedPlugin) MacOSPermissions() []Permission {
	platform, ok := p.Manifest.Platforms["macos"]
	if !ok {
		return nil
	}
	return platform.Permissions
}

// MacOSCSS returns the platform CSS file paths for the macOS platform.
func (p *ResolvedPlugin) MacOSCSS() []string {
	platform, ok := p.Manifest.Platforms["macos"]
	if !ok {
		return nil
	}
	var paths []string
	for _, f := range platform.CSS {
		paths = append(paths, filepath.Join(p.LocalPath, f))
	}
	return paths
}

// CollectMacOSAssets gathers all plugin JS and CSS files for the macOS bundle.
func CollectMacOSAssets(plugins []ResolvedPlugin) (jsAssets []PluginAsset, cssAssets []PluginAsset) {
	for _, p := range plugins {
		ns := p.Manifest.Namespace

		for _, src := range p.CrossPlatformJS() {
			jsAssets = append(jsAssets, PluginAsset{
				SrcPath:    src,
				BundleName: "plugin-" + ns + ".js",
			})
		}

		for _, src := range p.MacOSCSS() {
			cssAssets = append(cssAssets, PluginAsset{
				SrcPath:    src,
				BundleName: "plugin-" + ns + ".css",
			})
		}
	}
	return
}

// CollectMacOSPermissions gathers all macOS permissions from a list of plugins (deduplicated).
func CollectMacOSPermissions(plugins []ResolvedPlugin) []Permission {
	seen := map[string]bool{}
	var result []Permission
	for _, p := range plugins {
		for _, perm := range p.MacOSPermissions() {
			if !seen[perm.Key] {
				seen[perm.Key] = true
				result = append(result, perm)
			}
		}
	}
	return result
}

// CollectIOSPermissions gathers all permissions from a list of plugins (deduplicated).
func CollectIOSPermissions(plugins []ResolvedPlugin) []Permission {
	seen := map[string]bool{}
	var result []Permission
	for _, p := range plugins {
		for _, perm := range p.IOSPermissions() {
			if !seen[perm.Key] {
				seen[perm.Key] = true
				result = append(result, perm)
			}
		}
	}
	return result
}

// CollectIOSBackgroundModes gathers all background modes from a list of plugins (deduplicated).
func CollectIOSBackgroundModes(plugins []ResolvedPlugin) []string {
	seen := map[string]bool{}
	var result []string
	for _, p := range plugins {
		for _, mode := range p.IOSBackgroundModes() {
			if !seen[mode] {
				seen[mode] = true
				result = append(result, mode)
			}
		}
	}
	return result
}

// CollectIOSEntitlements gathers all entitlements from a list of plugins (deduplicated).
func CollectIOSEntitlements(plugins []ResolvedPlugin) []string {
	seen := map[string]bool{}
	var result []string
	for _, p := range plugins {
		for _, ent := range p.IOSEntitlements() {
			if !seen[ent] {
				seen[ent] = true
				result = append(result, ent)
			}
		}
	}
	return result
}

// PluginAsset represents a plugin asset file to be bundled.
type PluginAsset struct {
	SrcPath  string // absolute path to source file
	BundleName string // filename in the assets/ bundle (e.g. "plugin-camera.js")
}

// CollectIOSAssets gathers all plugin JS and CSS files for the iOS bundle.
// Returns namespaced filenames to avoid collisions.
func CollectIOSAssets(plugins []ResolvedPlugin) (jsAssets []PluginAsset, cssAssets []PluginAsset) {
	for _, p := range plugins {
		ns := p.Manifest.Namespace

		for _, src := range p.CrossPlatformJS() {
			jsAssets = append(jsAssets, PluginAsset{
				SrcPath:    src,
				BundleName: "plugin-" + ns + ".js",
			})
		}

		for _, src := range p.IOSCSS() {
			cssAssets = append(cssAssets, PluginAsset{
				SrcPath:    src,
				BundleName: "plugin-" + ns + ".css",
			})
		}
	}
	return
}

// --- Git operations ---

func gitClone(repo, dest string) error {
	os.MkdirAll(filepath.Dir(dest), 0755)
	url := "https://github.com/" + repo + ".git"
	cmd := exec.Command("git", "clone", "--depth", "1", url, dest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func gitPull(dir string) error {
	cmd := exec.Command("git", "pull", "--ff-only")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func pluginLocalPath(repo string) string {
	return filepath.Join(PluginsDir(), repo)
}

// --- Config read/write ---

func loadPluginsFromConfig(projectRoot string) ([]Installed, error) {
	data, err := os.ReadFile(filepath.Join(projectRoot, "phptoro.json"))
	if err != nil {
		return nil, err
	}
	var cfg map[string]json.RawMessage
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	raw, ok := cfg["plugins"]
	if !ok {
		return nil, nil
	}

	// Try array of strings first (simple format): ["phpToro/plugin-storage"]
	var simpleList []string
	if json.Unmarshal(raw, &simpleList) == nil {
		var result []Installed
		for _, repo := range simpleList {
			result = append(result, Installed{Repo: repo, Version: "main"})
		}
		return result, nil
	}

	// Try array of objects: [{"repo": "phpToro/plugin-storage", "version": "v1.0"}]
	var objectList []Installed
	if err := json.Unmarshal(raw, &objectList); err != nil {
		return nil, fmt.Errorf("invalid plugins format in phptoro.json: %w", err)
	}
	return objectList, nil
}

func addToConfig(projectRoot, repo string) error {
	path := filepath.Join(projectRoot, "phptoro.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var cfg map[string]json.RawMessage
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}

	// Load existing plugins
	var plugins []string
	if raw, ok := cfg["plugins"]; ok {
		json.Unmarshal(raw, &plugins)
	}

	// Check if already installed
	for _, p := range plugins {
		if strings.EqualFold(p, repo) {
			return nil // already installed
		}
	}

	plugins = append(plugins, repo)
	pluginsJSON, _ := json.Marshal(plugins)
	cfg["plugins"] = pluginsJSON

	out, err := json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0644)
}

func removeFromConfig(projectRoot, repo string) error {
	path := filepath.Join(projectRoot, "phptoro.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var cfg map[string]json.RawMessage
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}

	var plugins []string
	if raw, ok := cfg["plugins"]; ok {
		json.Unmarshal(raw, &plugins)
	}

	var filtered []string
	for _, p := range plugins {
		if !strings.EqualFold(p, repo) {
			filtered = append(filtered, p)
		}
	}

	if len(filtered) == 0 {
		delete(cfg, "plugins")
	} else {
		pluginsJSON, _ := json.Marshal(filtered)
		cfg["plugins"] = pluginsJSON
	}

	out, err := json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0644)
}
