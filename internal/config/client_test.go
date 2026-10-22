package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/havspect/project-times/internal/client"
)

func TestLoadClientPrecedence(t *testing.T) {
	file := client.Config{URL: "https://file.example", Token: "file-token"}
	tests := []struct {
		name      string
		save      bool
		env       map[string]string
		urlFlag   string
		tokenFlag string
		want      client.Config
	}{
		{
			name: "missing file is empty",
			want: client.Config{},
		},
		{
			name: "file only",
			save: true,
			want: file,
		},
		{
			name: "env beats file",
			save: true,
			env:  map[string]string{"TIMES_URL": "https://env.example", "TIMES_TOKEN": "env-token"},
			want: client.Config{URL: "https://env.example", Token: "env-token"},
		},
		{
			name:      "flags beat env and file",
			save:      true,
			env:       map[string]string{"TIMES_URL": "https://env.example", "TIMES_TOKEN": "env-token"},
			urlFlag:   "https://flag.example",
			tokenFlag: "flag-token",
			want:      client.Config{URL: "https://flag.example", Token: "flag-token"},
		},
		{
			name:    "url flag keeps the file token",
			save:    true,
			urlFlag: "https://flag.example",
			want:    client.Config{URL: "https://flag.example", Token: "file-token"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			if tt.save {
				if err := saveClient(file, home); err != nil {
					t.Fatalf("save: %v", err)
				}
			}
			getenv := func(key string) string { return tt.env[key] }
			got, err := loadClient(tt.urlFlag, tt.tokenFlag, getenv, home)
			if err != nil {
				t.Fatalf("loadClient: %v", err)
			}
			if got != tt.want {
				t.Errorf("loadClient = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestSaveClient(t *testing.T) {
	home := t.TempDir()
	want := client.Config{URL: "https://times.example.com", Token: "ts_secret"}

	if err := saveClient(want, home); err != nil {
		t.Fatalf("save: %v", err)
	}

	path := filepath.Join(home, ".config", "times", "config.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat config: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("config mode = %o, want 600", got)
	}
	if dir, err := os.Stat(filepath.Dir(path)); err != nil {
		t.Fatalf("stat config dir: %v", err)
	} else if got := dir.Mode().Perm(); got != 0o700 {
		t.Fatalf("config dir mode = %o, want 700", got)
	}

	got, err := loadClient("", "", func(string) string { return "" }, home)
	if err != nil {
		t.Fatalf("load saved config: %v", err)
	}
	if got != want {
		t.Fatalf("round trip = %+v, want %+v", got, want)
	}
}

func TestSaveClientTightensExistingMode(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "times")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	if err := saveClient(client.Config{URL: "https://times.example.com"}, home); err != nil {
		t.Fatalf("save: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("config mode = %o, want 600 after overwriting", got)
	}
}

func TestLoadClientMalformed(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "times")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("not json"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := loadClient("", "", func(string) string { return "" }, home); err == nil {
		t.Fatal("loadClient accepted a malformed config file")
	}
}
