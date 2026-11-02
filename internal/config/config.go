package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"chalk-tracker/internal/todo"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	appDirName = "chalk-tracker"
	envPrefix  = "TODO"

	keyDataFile       = "data_file"
	keyDefaultProject = "default_project"
	keyColor          = "color"

	flagConfig   = "config"
	flagDataFile = "data-file"
	flagColor    = "color"
)

type Config struct {
	DataFile       string
	DefaultProject string
	Color          bool
}

// AddFlags registers the flags that Load reads.
func AddFlags(flags *pflag.FlagSet) {
	flags.String(flagConfig, "", "config file (default ~/.config/chalk-tracker/config.yaml)")
	flags.String(flagDataFile, "", "task data file (default ~/.config/chalk-tracker/store.json)")
	flags.Bool(flagColor, true, "enable colored output")
}

// Load resolves settings in this order: flags, TODO_* environment variables,
// config file, defaults. A missing config file is only an error when its path
// was set explicitly with --config.
func Load(flags *pflag.FlagSet) (Config, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return Config{}, err
	}
	appDir := filepath.Join(configDir, appDirName)

	v := viper.New()
	v.SetDefault(keyDataFile, filepath.Join(appDir, "store.json"))
	v.SetDefault(keyDefaultProject, todo.DefaultProject)
	v.SetDefault(keyColor, true)
	v.SetEnvPrefix(envPrefix)
	v.AutomaticEnv()

	if err := v.BindPFlag(keyDataFile, flags.Lookup(flagDataFile)); err != nil {
		return Config{}, err
	}
	if err := v.BindPFlag(keyColor, flags.Lookup(flagColor)); err != nil {
		return Config{}, err
	}

	configPath, err := flags.GetString(flagConfig)
	if err != nil {
		return Config{}, err
	}
	explicit := configPath != ""
	if !explicit {
		configPath = filepath.Join(appDir, "config.yaml")
	}
	v.SetConfigFile(configPath)
	if err := v.ReadInConfig(); err != nil {
		if explicit || !errors.Is(err, fs.ErrNotExist) {
			return Config{}, fmt.Errorf("read config %s: %w", configPath, err)
		}
	}

	dataFile, err := expandHome(v.GetString(keyDataFile))
	if err != nil {
		return Config{}, err
	}
	return Config{
		DataFile:       dataFile,
		DefaultProject: v.GetString(keyDefaultProject),
		Color:          v.GetBool(keyColor),
	}, nil
}

// expandHome resolves a leading ~/, which the shell does not expand for values
// read from a config file.
func expandHome(path string) (string, error) {
	if !strings.HasPrefix(path, "~/") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, path[2:]), nil
}
