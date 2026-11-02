package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/pflag"
)

// setup isolates the test from the real config directory and returns the path
// of the default config file.
func setup(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	return filepath.Join(dir, appDirName, "config.yaml")
}

func load(t *testing.T, args ...string) (Config, error) {
	t.Helper()
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	AddFlags(flags)
	if err := flags.Parse(args); err != nil {
		t.Fatal(err)
	}
	return Load(flags)
}

func writeConfig(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultsWhenConfigFileMissing(t *testing.T) {
	configPath := setup(t)

	cfg, err := load(t)
	if err != nil {
		t.Fatal(err)
	}
	want := Config{
		DataFile:       filepath.Join(filepath.Dir(configPath), "store.json"),
		DefaultProject: "Inbox",
		Color:          true,
	}
	if cfg != want {
		t.Errorf("got %+v, want %+v", cfg, want)
	}
}

func TestDataFilePrecedence(t *testing.T) {
	configPath := setup(t)
	writeConfig(t, configPath, "data_file: /from/file.json\n")

	cfg, err := load(t)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DataFile != "/from/file.json" {
		t.Errorf("config file over default: got %q", cfg.DataFile)
	}

	t.Setenv("TODO_DATA_FILE", "/from/env.json")
	cfg, err = load(t)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DataFile != "/from/env.json" {
		t.Errorf("env over config file: got %q", cfg.DataFile)
	}

	cfg, err = load(t, "--data-file=/from/flag.json")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DataFile != "/from/flag.json" {
		t.Errorf("flag over env: got %q", cfg.DataFile)
	}
}

func TestConfigFileValues(t *testing.T) {
	configPath := setup(t)
	writeConfig(t, configPath, "default_project: Work\ncolor: false\n")

	cfg, err := load(t)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultProject != "Work" {
		t.Errorf("DefaultProject = %q", cfg.DefaultProject)
	}
	if cfg.Color {
		t.Error("Color = true, want false")
	}
}

func TestColorFlagOverridesConfigFile(t *testing.T) {
	configPath := setup(t)
	writeConfig(t, configPath, "color: false\n")

	cfg, err := load(t, "--color=true")
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Color {
		t.Error("Color = false, want true")
	}
}

func TestExpandsHomeInDataFile(t *testing.T) {
	configPath := setup(t)
	writeConfig(t, configPath, "data_file: ~/tasks.json\n")

	cfg, err := load(t)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(os.Getenv("HOME"), "tasks.json")
	if cfg.DataFile != want {
		t.Errorf("got %q, want %q", cfg.DataFile, want)
	}
}

func TestExplicitConfigFileMustExist(t *testing.T) {
	setup(t)

	if _, err := load(t, "--config=/does/not/exist.yaml"); err == nil {
		t.Error("expected an error for a missing explicit config file")
	}
}

func TestMalformedConfigFileIsAnError(t *testing.T) {
	configPath := setup(t)
	writeConfig(t, configPath, "data_file: [unclosed\n")

	if _, err := load(t); err == nil {
		t.Error("expected an error for malformed YAML")
	}
}
