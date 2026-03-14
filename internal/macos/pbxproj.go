package macos

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strings"
)

func pbxUUID(parts ...string) string {
	h := md5.Sum([]byte("phptoro-macos:" + strings.Join(parts, ":")))
	return strings.ToUpper(hex.EncodeToString(h[:12]))
}

type pbxFile struct {
	Name  string
	Path  string
	Group string
	Type  string
}

func coreSwiftFiles(target string) []pbxFile {
	return []pbxFile{
		{"main.swift", target + "/main.swift", "root", "sourcecode.swift"},
		{"AppDelegate.swift", target + "/AppDelegate.swift", "root", "sourcecode.swift"},
		{"PhpEngine.swift", target + "/Bridge/PhpEngine.swift", "Bridge", "sourcecode.swift"},
		{"PluginHost.swift", target + "/Engine/PluginHost.swift", "Engine", "sourcecode.swift"},
		{"AppKernel.swift", target + "/Engine/AppKernel.swift", "Engine", "sourcecode.swift"},
		{"AppCoordinator.swift", target + "/Engine/AppCoordinator.swift", "Engine", "sourcecode.swift"},
		{"ScreenViewController.swift", target + "/Engine/ScreenViewController.swift", "Engine", "sourcecode.swift"},
		{"HotReloadClient.swift", target + "/Engine/HotReloadClient.swift", "Engine", "sourcecode.swift"},
		{"SchemeHandler.swift", target + "/Engine/SchemeHandler.swift", "Engine", "sourcecode.swift"},
		{"PhpToroApp.swift", target + "/Engine/PhpToroApp.swift", "Engine", "sourcecode.swift"},
		{"DebugLogger.swift", target + "/Engine/DebugLogger.swift", "Engine", "sourcecode.swift"},
		{"MenuBarManager.swift", target + "/Engine/MenuBarManager.swift", "Engine", "sourcecode.swift"},
		{"WindowManager.swift", target + "/Engine/WindowManager.swift", "Engine", "sourcecode.swift"},
		{"ToolbarManager.swift", target + "/Engine/ToolbarManager.swift", "Engine", "sourcecode.swift"},
		{"WindowHandler.swift", target + "/Handlers/WindowHandler.swift", "Handlers", "sourcecode.swift"},
		{"StateHandler.swift", target + "/Handlers/StateHandler.swift", "Handlers", "sourcecode.swift"},
		{"LinkingHandler.swift", target + "/Handlers/LinkingHandler.swift", "Handlers", "sourcecode.swift"},
		{"PlatformHandler.swift", target + "/Handlers/PlatformHandler.swift", "Handlers", "sourcecode.swift"},
	}
}

type pbxOptions struct {
	AppName          string
	BundleID         string
	DeploymentTarget string
	PluginFiles      []pbxFile
	PluginFrameworks []string
}

func generatePbxproj(opts pbxOptions) string {
	target := opts.AppName
	files := coreSwiftFiles(target)
	files = append(files, opts.PluginFiles...)

	projectUUID := pbxUUID("project")
	mainGroupUUID := pbxUUID("mainGroup")
	targetUUID := pbxUUID("target")
	productRefUUID := pbxUUID("product")
	productsGroupUUID := pbxUUID("productsGroup")
	sourcesPhaseUUID := pbxUUID("sourcesPhase")
	frameworksPhaseUUID := pbxUUID("frameworksPhase")
	resourcesPhaseUUID := pbxUUID("resourcesPhase")
	projectConfigListUUID := pbxUUID("projectConfigList")
	targetConfigListUUID := pbxUUID("targetConfigList")
	projectDebugUUID := pbxUUID("projectDebug")
	projectReleaseUUID := pbxUUID("projectRelease")
	targetDebugUUID := pbxUUID("targetDebug")
	targetReleaseUUID := pbxUUID("targetRelease")

	rootGroupUUID := pbxUUID("group", "root")
	bridgeGroupUUID := pbxUUID("group", "Bridge")
	engineGroupUUID := pbxUUID("group", "Engine")
	handlersGroupUUID := pbxUUID("group", "Handlers")
	pluginsGroupUUID := pbxUUID("group", "Plugins")

	bridgingHeaderRefUUID := pbxUUID("fileRef", "bridgingHeader")
	bridgeHeaderRefUUID := pbxUUID("fileRef", "PhpToroBridge.h")
	infoPlistRefUUID := pbxUUID("fileRef", "Info.plist")
	appFolderRefUUID := pbxUUID("fileRef", "app")
	appFolderBuildUUID := pbxUUID("buildFile", "app")
	assetsFolderRefUUID := pbxUUID("fileRef", "assets")
	assetsFolderBuildUUID := pbxUUID("buildFile", "assets")

	var b strings.Builder
	w := func(format string, args ...any) {
		fmt.Fprintf(&b, format+"\n", args...)
	}

	w(`// !$*UTF8*$!`)
	w(`{`)
	w(`	archiveVersion = 1;`)
	w(`	classes = {`)
	w(`	};`)
	w(`	objectVersion = 56;`)
	w(`	objects = {`)
	w(``)

	// PBXBuildFile
	w(`/* Begin PBXBuildFile section */`)
	for _, f := range files {
		buildUUID := pbxUUID("buildFile", f.Path)
		w(`		%s /* %s in Sources */ = {isa = PBXBuildFile; fileRef = %s /* %s */; };`,
			buildUUID, f.Name, pbxUUID("fileRef", f.Path), f.Name)
	}
	w(`		%s /* app in Resources */ = {isa = PBXBuildFile; fileRef = %s /* app */; };`,
		appFolderBuildUUID, appFolderRefUUID)
	w(`		%s /* assets in Resources */ = {isa = PBXBuildFile; fileRef = %s /* assets */; };`,
		assetsFolderBuildUUID, assetsFolderRefUUID)
	w(`/* End PBXBuildFile section */`)
	w(``)

	// PBXFileReference
	w(`/* Begin PBXFileReference section */`)
	w(`		%s /* %s.app */ = {isa = PBXFileReference; explicitFileType = wrapper.application; includeInIndex = 0; path = %s.app; sourceTree = BUILT_PRODUCTS_DIR; };`,
		productRefUUID, target, target)
	for _, f := range files {
		refUUID := pbxUUID("fileRef", f.Path)
		w(`		%s /* %s */ = {isa = PBXFileReference; lastKnownFileType = %s; path = %s; sourceTree = "<group>"; };`,
			refUUID, f.Name, f.Type, f.Name)
	}
	w(`		%s /* %s-Bridging-Header.h */ = {isa = PBXFileReference; lastKnownFileType = sourcecode.c.h; path = "%s-Bridging-Header.h"; sourceTree = "<group>"; };`,
		bridgingHeaderRefUUID, target, target)
	w(`		%s /* PhpToroBridge.h */ = {isa = PBXFileReference; lastKnownFileType = sourcecode.c.h; path = PhpToroBridge.h; sourceTree = "<group>"; };`,
		bridgeHeaderRefUUID)
	w(`		%s /* Info.plist */ = {isa = PBXFileReference; lastKnownFileType = text.plist.xml; path = Info.plist; sourceTree = "<group>"; };`,
		infoPlistRefUUID)
	w(`		%s /* app */ = {isa = PBXFileReference; lastKnownFileType = folder; path = app; sourceTree = "<group>"; };`,
		appFolderRefUUID)
	w(`		%s /* assets */ = {isa = PBXFileReference; lastKnownFileType = folder; path = assets; sourceTree = "<group>"; };`,
		assetsFolderRefUUID)
	w(`/* End PBXFileReference section */`)
	w(``)

	// PBXFrameworksBuildPhase
	w(`/* Begin PBXFrameworksBuildPhase section */`)
	w(`		%s /* Frameworks */ = {`, frameworksPhaseUUID)
	w(`			isa = PBXFrameworksBuildPhase;`)
	w(`			buildActionMask = 2147483647;`)
	w(`			files = (`)
	w(`			);`)
	w(`			runOnlyForDeploymentPostprocessing = 0;`)
	w(`		};`)
	w(`/* End PBXFrameworksBuildPhase section */`)
	w(``)

	// PBXGroup
	w(`/* Begin PBXGroup section */`)
	w(`		%s = {`, mainGroupUUID)
	w(`			isa = PBXGroup;`)
	w(`			children = (`)
	w(`				%s /* %s */,`, rootGroupUUID, target)
	w(`				%s /* Products */,`, productsGroupUUID)
	w(`			);`)
	w(`			sourceTree = "<group>";`)
	w(`		};`)
	w(`		%s /* Products */ = {`, productsGroupUUID)
	w(`			isa = PBXGroup;`)
	w(`			children = (`)
	w(`				%s /* %s.app */,`, productRefUUID, target)
	w(`			);`)
	w(`			name = Products;`)
	w(`			sourceTree = "<group>";`)
	w(`		};`)

	rootChildren := []string{}
	for _, f := range files {
		if f.Group == "root" {
			rootChildren = append(rootChildren, fmt.Sprintf("\t\t\t\t%s /* %s */,", pbxUUID("fileRef", f.Path), f.Name))
		}
	}
	w(`		%s /* %s */ = {`, rootGroupUUID, target)
	w(`			isa = PBXGroup;`)
	w(`			children = (`)
	for _, c := range rootChildren {
		w(c)
	}
	w(`				%s /* Bridge */,`, bridgeGroupUUID)
	w(`				%s /* Engine */,`, engineGroupUUID)
	w(`				%s /* Handlers */,`, handlersGroupUUID)
	w(`				%s /* Plugins */,`, pluginsGroupUUID)
	w(`				%s /* %s-Bridging-Header.h */,`, bridgingHeaderRefUUID, target)
	w(`				%s /* Info.plist */,`, infoPlistRefUUID)
	w(`				%s /* app */,`, appFolderRefUUID)
	w(`				%s /* assets */,`, assetsFolderRefUUID)
	w(`			);`)
	w(`			path = %s;`, target)
	w(`			sourceTree = "<group>";`)
	w(`		};`)

	groups := []struct {
		name string
		uuid string
	}{
		{"Bridge", bridgeGroupUUID},
		{"Engine", engineGroupUUID},
		{"Handlers", handlersGroupUUID},
		{"Plugins", pluginsGroupUUID},
	}
	for _, g := range groups {
		w(`		%s /* %s */ = {`, g.uuid, g.name)
		w(`			isa = PBXGroup;`)
		w(`			children = (`)
		for _, f := range files {
			if f.Group == g.name {
				w(`				%s /* %s */,`, pbxUUID("fileRef", f.Path), f.Name)
			}
		}
		if g.name == "Bridge" {
			w(`				%s /* PhpToroBridge.h */,`, bridgeHeaderRefUUID)
		}
		w(`			);`)
		w(`			path = %s;`, g.name)
		w(`			sourceTree = "<group>";`)
		w(`		};`)
	}

	w(`/* End PBXGroup section */`)
	w(``)

	// PBXNativeTarget
	w(`/* Begin PBXNativeTarget section */`)
	w(`		%s /* %s */ = {`, targetUUID, target)
	w(`			isa = PBXNativeTarget;`)
	w(`			buildConfigurationList = %s /* Build configuration list for PBXNativeTarget "%s" */;`, targetConfigListUUID, target)
	w(`			buildPhases = (`)
	w(`				%s /* Sources */,`, sourcesPhaseUUID)
	w(`				%s /* Frameworks */,`, frameworksPhaseUUID)
	w(`				%s /* Resources */,`, resourcesPhaseUUID)
	w(`			);`)
	w(`			buildRules = (`)
	w(`			);`)
	w(`			dependencies = (`)
	w(`			);`)
	w(`			name = %s;`, target)
	w(`			productName = %s;`, target)
	w(`			productReference = %s /* %s.app */;`, productRefUUID, target)
	w(`			productType = "com.apple.product-type.application";`)
	w(`		};`)
	w(`/* End PBXNativeTarget section */`)
	w(``)

	// PBXProject
	w(`/* Begin PBXProject section */`)
	w(`		%s /* Project object */ = {`, projectUUID)
	w(`			isa = PBXProject;`)
	w(`			attributes = {`)
	w(`				BuildIndependentTargetsInParallel = 1;`)
	w(`				LastSwiftUpdateCheck = 1500;`)
	w(`				LastUpgradeCheck = 1500;`)
	w(`			};`)
	w(`			buildConfigurationList = %s /* Build configuration list for PBXProject */;`, projectConfigListUUID)
	w(`			compatibilityVersion = "Xcode 14.0";`)
	w(`			developmentRegion = en;`)
	w(`			hasScannedForEncodings = 0;`)
	w(`			knownRegions = (`)
	w(`				en,`)
	w(`				Base,`)
	w(`			);`)
	w(`			mainGroup = %s;`, mainGroupUUID)
	w(`			productRefGroup = %s /* Products */;`, productsGroupUUID)
	w(`			projectDirPath = "";`)
	w(`			projectRoot = "";`)
	w(`			targets = (`)
	w(`				%s /* %s */,`, targetUUID, target)
	w(`			);`)
	w(`		};`)
	w(`/* End PBXProject section */`)
	w(``)

	// PBXResourcesBuildPhase
	w(`/* Begin PBXResourcesBuildPhase section */`)
	w(`		%s /* Resources */ = {`, resourcesPhaseUUID)
	w(`			isa = PBXResourcesBuildPhase;`)
	w(`			buildActionMask = 2147483647;`)
	w(`			files = (`)
	w(`				%s /* app in Resources */,`, appFolderBuildUUID)
	w(`				%s /* assets in Resources */,`, assetsFolderBuildUUID)
	w(`			);`)
	w(`			runOnlyForDeploymentPostprocessing = 0;`)
	w(`		};`)
	w(`/* End PBXResourcesBuildPhase section */`)
	w(``)

	// PBXSourcesBuildPhase
	w(`/* Begin PBXSourcesBuildPhase section */`)
	w(`		%s /* Sources */ = {`, sourcesPhaseUUID)
	w(`			isa = PBXSourcesBuildPhase;`)
	w(`			buildActionMask = 2147483647;`)
	w(`			files = (`)
	for _, f := range files {
		w(`				%s /* %s in Sources */,`, pbxUUID("buildFile", f.Path), f.Name)
	}
	w(`			);`)
	w(`			runOnlyForDeploymentPostprocessing = 0;`)
	w(`		};`)
	w(`/* End PBXSourcesBuildPhase section */`)
	w(``)

	// XCBuildConfiguration
	deployTarget := opts.DeploymentTarget
	if deployTarget == "" {
		deployTarget = "12.0"
	}

	ldFlagsList := []string{
		`"-lphp"`,
		`"-lphptoro_sapi"`,
		`"-lphptoro_ext"`,
		`"-lcrypto"`,
		`"-lssl"`,
		`"-lxml2"`,
		`"-liconv"`,
		`"-lcharset"`,
		`"-lsodium"`,
		`"-lsqlite3"`,
		`"-lz"`,
		`"-lresolv"`,
		`"-framework WebKit"`,
		`"-framework AppKit"`,
	}
	for _, fw := range opts.PluginFrameworks {
		if fw == "UIKit" || fw == "SafariServices" {
			continue
		}
		ldFlagsList = append(ldFlagsList, fmt.Sprintf(`"-framework %s"`, fw))
	}
	ldFlags := strings.Join(ldFlagsList, ",\n\t\t\t\t\t")

	headerSearchPaths := fmt.Sprintf(`"$(PROJECT_DIR)/%s/Headers","$(PROJECT_DIR)/%s/Headers/php/main","$(PROJECT_DIR)/%s/Headers/php/Zend","$(PROJECT_DIR)/%s/Headers/php/TSRM","$(PROJECT_DIR)/%s/Headers/php/ext","$(PROJECT_DIR)/%s/Headers/php/sapi","$(PROJECT_DIR)/%s/Headers/php/sapi/embed"`,
		target, target, target, target, target, target, target)

	w(`/* Begin XCBuildConfiguration section */`)

	// Project Debug
	w(`		%s /* Debug */ = {`, projectDebugUUID)
	w(`			isa = XCBuildConfiguration;`)
	w(`			buildSettings = {`)
	w(`				ALWAYS_SEARCH_USER_PATHS = NO;`)
	w(`				CLANG_ANALYZER_NONNULL = YES;`)
	w(`				CLANG_CXX_LANGUAGE_STANDARD = "c++20";`)
	w(`				CLANG_ENABLE_MODULES = YES;`)
	w(`				CLANG_ENABLE_OBJC_ARC = YES;`)
	w(`				COPY_PHASE_STRIP = NO;`)
	w(`				DEBUG_INFORMATION_FORMAT = dwarf;`)
	w(`				ENABLE_STRICT_OBJC_MSGSEND = YES;`)
	w(`				ENABLE_TESTABILITY = YES;`)
	w(`				GCC_DYNAMIC_NO_PIC = NO;`)
	w(`				GCC_NO_COMMON_BLOCKS = YES;`)
	w(`				GCC_OPTIMIZATION_LEVEL = 0;`)
	w(`				GCC_PREPROCESSOR_DEFINITIONS = (`)
	w(`					"DEBUG=1",`)
	w(`					"$(inherited)",`)
	w(`				);`)
	w(`				MACOSX_DEPLOYMENT_TARGET = %s;`, deployTarget)
	w(`				MTL_ENABLE_DEBUG_INFO = INCLUDE_SOURCE;`)
	w(`				ONLY_ACTIVE_ARCH = YES;`)
	w(`				SDKROOT = macosx;`)
	w(`				SWIFT_ACTIVE_COMPILATION_CONDITIONS = DEBUG;`)
	w(`				SWIFT_OPTIMIZATION_LEVEL = "-Onone";`)
	w(`			};`)
	w(`			name = Debug;`)
	w(`		};`)

	// Project Release
	w(`		%s /* Release */ = {`, projectReleaseUUID)
	w(`			isa = XCBuildConfiguration;`)
	w(`			buildSettings = {`)
	w(`				ALWAYS_SEARCH_USER_PATHS = NO;`)
	w(`				CLANG_ANALYZER_NONNULL = YES;`)
	w(`				CLANG_CXX_LANGUAGE_STANDARD = "c++20";`)
	w(`				CLANG_ENABLE_MODULES = YES;`)
	w(`				CLANG_ENABLE_OBJC_ARC = YES;`)
	w(`				COPY_PHASE_STRIP = NO;`)
	w(`				DEBUG_INFORMATION_FORMAT = "dwarf-with-dsym";`)
	w(`				ENABLE_NS_ASSERTIONS = NO;`)
	w(`				ENABLE_STRICT_OBJC_MSGSEND = YES;`)
	w(`				GCC_NO_COMMON_BLOCKS = YES;`)
	w(`				MACOSX_DEPLOYMENT_TARGET = %s;`, deployTarget)
	w(`				MTL_ENABLE_DEBUG_INFO = NO;`)
	w(`				SDKROOT = macosx;`)
	w(`				SWIFT_COMPILATION_MODE = wholemodule;`)
	w(`				SWIFT_OPTIMIZATION_LEVEL = "-O";`)
	w(`				VALIDATE_PRODUCT = YES;`)
	w(`			};`)
	w(`			name = Release;`)
	w(`		};`)

	// Target Debug
	w(`		%s /* Debug */ = {`, targetDebugUUID)
	w(`			isa = XCBuildConfiguration;`)
	w(`			buildSettings = {`)
	w(`				INFOPLIST_FILE = %s/Info.plist;`, target)
	w(`				CODE_SIGN_ENTITLEMENTS = "%s/%s.entitlements";`, target, target)
	w(`				LD_RUNPATH_SEARCH_PATHS = (`)
	w(`					"$(inherited)",`)
	w(`					"@executable_path/../Frameworks",`)
	w(`				);`)
	w(`				LIBRARY_SEARCH_PATHS = "$(PROJECT_DIR)/Libraries";`)
	w(`				HEADER_SEARCH_PATHS = (`)
	w(`					%s`, headerSearchPaths)
	w(`				);`)
	w(`				OTHER_LDFLAGS = (`)
	w(`					%s`, ldFlags)
	w(`				);`)
	w(`				PRODUCT_BUNDLE_IDENTIFIER = "%s";`, opts.BundleID)
	w(`				PRODUCT_NAME = "$(TARGET_NAME)";`)
	w(`				SWIFT_EMIT_LOC_STRINGS = YES;`)
	w(`				SWIFT_OBJC_BRIDGING_HEADER = "%s/%s-Bridging-Header.h";`, target, target)
	w(`				SWIFT_VERSION = 5.0;`)
	w(`				CODE_SIGN_IDENTITY = "-";`)
	w(`			};`)
	w(`			name = Debug;`)
	w(`		};`)

	// Target Release
	w(`		%s /* Release */ = {`, targetReleaseUUID)
	w(`			isa = XCBuildConfiguration;`)
	w(`			buildSettings = {`)
	w(`				INFOPLIST_FILE = %s/Info.plist;`, target)
	w(`				CODE_SIGN_ENTITLEMENTS = "%s/%s.entitlements";`, target, target)
	w(`				LD_RUNPATH_SEARCH_PATHS = (`)
	w(`					"$(inherited)",`)
	w(`					"@executable_path/../Frameworks",`)
	w(`				);`)
	w(`				LIBRARY_SEARCH_PATHS = "$(PROJECT_DIR)/Libraries";`)
	w(`				HEADER_SEARCH_PATHS = (`)
	w(`					%s`, headerSearchPaths)
	w(`				);`)
	w(`				OTHER_LDFLAGS = (`)
	w(`					%s`, ldFlags)
	w(`				);`)
	w(`				PRODUCT_BUNDLE_IDENTIFIER = "%s";`, opts.BundleID)
	w(`				PRODUCT_NAME = "$(TARGET_NAME)";`)
	w(`				SWIFT_EMIT_LOC_STRINGS = YES;`)
	w(`				SWIFT_OBJC_BRIDGING_HEADER = "%s/%s-Bridging-Header.h";`, target, target)
	w(`				SWIFT_VERSION = 5.0;`)
	w(`				CODE_SIGN_IDENTITY = "-";`)
	w(`			};`)
	w(`			name = Release;`)
	w(`		};`)

	w(`/* End XCBuildConfiguration section */`)
	w(``)

	// XCConfigurationList
	w(`/* Begin XCConfigurationList section */`)
	w(`		%s /* Build configuration list for PBXProject */ = {`, projectConfigListUUID)
	w(`			isa = XCConfigurationList;`)
	w(`			buildConfigurations = (`)
	w(`				%s /* Debug */,`, projectDebugUUID)
	w(`				%s /* Release */,`, projectReleaseUUID)
	w(`			);`)
	w(`			defaultConfigurationIsVisible = 0;`)
	w(`			defaultConfigurationName = Release;`)
	w(`		};`)
	w(`		%s /* Build configuration list for PBXNativeTarget "%s" */ = {`, targetConfigListUUID, target)
	w(`			isa = XCConfigurationList;`)
	w(`			buildConfigurations = (`)
	w(`				%s /* Debug */,`, targetDebugUUID)
	w(`				%s /* Release */,`, targetReleaseUUID)
	w(`			);`)
	w(`			defaultConfigurationIsVisible = 0;`)
	w(`			defaultConfigurationName = Release;`)
	w(`		};`)
	w(`/* End XCConfigurationList section */`)

	w(`	};`)
	w(`	rootObject = %s /* Project object */;`, projectUUID)
	w(`}`)

	return b.String()
}
