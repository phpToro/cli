package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const FileName = "phptoro.json"

type Config struct {
	App         AppConfig         `json:"app"`
	Icon        string            `json:"icon,omitempty"`
	Splash      SplashConfig      `json:"splash,omitempty"`
	Platforms   PlatformsConfig   `json:"platforms,omitempty"`
	Orientation string            `json:"orientation,omitempty"`
	StatusBar   StatusBarConfig   `json:"statusBar,omitempty"`
	Fonts       []string          `json:"fonts,omitempty"`
	DeepLinks   *DeepLinksConfig  `json:"deepLinks,omitempty"`
}

type AppConfig struct {
	Name     string `json:"name"`
	Entry    string `json:"entry"`
	BundleID string `json:"bundleId"`
	Version  string `json:"version"`
	Debug    bool   `json:"debug"`
}

type SplashConfig struct {
	Image           string `json:"image,omitempty"`
	BackgroundColor string `json:"backgroundColor,omitempty"`
	ResizeMode      string `json:"resizeMode,omitempty"`
}

type PlatformsConfig struct {
	IOS     *IOSConfig     `json:"ios,omitempty"`
	Android *AndroidConfig `json:"android,omitempty"`
}

type IOSConfig struct {
	DeploymentTarget string `json:"deploymentTarget,omitempty"`
	TeamID           string `json:"teamId,omitempty"`
}

type AndroidConfig struct {
	MinSdk       int              `json:"minSdk,omitempty"`
	TargetSdk    int              `json:"targetSdk,omitempty"`
	AdaptiveIcon *AdaptiveIcon    `json:"adaptiveIcon,omitempty"`
}

type AdaptiveIcon struct {
	Foreground string `json:"foreground,omitempty"`
	Background string `json:"background,omitempty"`
}

type StatusBarConfig struct {
	Style  string `json:"style,omitempty"`
	Hidden bool   `json:"hidden,omitempty"`
}

type DeepLinksConfig struct {
	Prefixes []string          `json:"prefixes,omitempty"`
	Screens  map[string]string `json:"screens,omitempty"`
}

func Load(dir string) (*Config, error) {
	path := filepath.Join(dir, FileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func FindProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, FileName)); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", errors.New("not a phpToro project (phptoro.json not found)")
}
