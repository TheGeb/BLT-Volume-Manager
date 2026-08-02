package cfg

import (
	"os"
	"path/filepath"
	"testing"
)

func writeEnvFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func unsetenv(t *testing.T, keys ...string) {
	t.Helper()
	for _, k := range keys {
		old, had := os.LookupEnv(k)
		if err := os.Unsetenv(k); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if had {
				_ = os.Setenv(k, old)
			} else {
				_ = os.Unsetenv(k)
			}
		})
	}
}

func TestLoadEnv_ConfigFileFillsMissingVars(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	config := writeEnvFile(t, dir, "blt.env",
		"RESTIC_REPOSITORY=s3:http://127.0.0.1:3900/repo\nRESTIC_PASSWORD=from-file\n")
	t.Setenv(configFileEnvVar, config)
	unsetenv(t, "RESTIC_REPOSITORY", "RESTIC_PASSWORD")

	LoadEnv()

	if got := os.Getenv("RESTIC_REPOSITORY"); got != "s3:http://127.0.0.1:3900/repo" {
		t.Errorf("RESTIC_REPOSITORY = %q, want value from config file", got)
	}
	if got := os.Getenv("RESTIC_PASSWORD"); got != "from-file" {
		t.Errorf("RESTIC_PASSWORD = %q, want %q", got, "from-file")
	}
}

func TestLoadEnv_EnvironmentWinsOverConfigFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	config := writeEnvFile(t, dir, "blt.env", "RESTIC_PASSWORD=from-file\n")
	t.Setenv(configFileEnvVar, config)
	t.Setenv("RESTIC_PASSWORD", "from-env")

	LoadEnv()

	if got := os.Getenv("RESTIC_PASSWORD"); got != "from-env" {
		t.Errorf("RESTIC_PASSWORD = %q, want env value %q", got, "from-env")
	}
}

func TestLoadEnv_EnvFileBeatsConfigFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeEnvFile(t, dir, ".env", "RESTIC_PASSWORD=from-dotenv\n")
	config := writeEnvFile(t, dir, "blt.env", "RESTIC_PASSWORD=from-config\n")
	t.Setenv(configFileEnvVar, config)
	unsetenv(t, "RESTIC_PASSWORD")

	LoadEnv()

	if got := os.Getenv("RESTIC_PASSWORD"); got != "from-dotenv" {
		t.Errorf("RESTIC_PASSWORD = %q, want %q", got, "from-dotenv")
	}
}

func TestLoadEnv_MissingConfigFileIsIgnored(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv(configFileEnvVar, filepath.Join(dir, "does-not-exist.env"))
	unsetenv(t, "RESTIC_PASSWORD")

	LoadEnv()

	if got := os.Getenv("RESTIC_PASSWORD"); got != "" {
		t.Errorf("RESTIC_PASSWORD = %q, want empty", got)
	}
}

func TestPermsTooLoose(t *testing.T) {
	cases := []struct {
		name string
		perm os.FileMode
		want bool
	}{
		{"root only", 0o600, false},
		{"root+group read", 0o640, false},
		{"group writable only", 0o620, false},
		{"world readable", 0o644, true},
		{"world writable", 0o662, true},
		{"world rwx", 0o666, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := permsTooLoose(tc.perm); got != tc.want {
				t.Errorf("permsTooLoose(%#o) = %v, want %v", tc.perm, got, tc.want)
			}
		})
	}
}

func TestLoadEnv_LoosePermsStillLoads(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	config := filepath.Join(dir, "blt.env")
	if err := os.WriteFile(config, []byte("RESTIC_PASSWORD=from-file\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(configFileEnvVar, config)
	unsetenv(t, "RESTIC_PASSWORD")

	LoadEnv()

	if got := os.Getenv("RESTIC_PASSWORD"); got != "from-file" {
		t.Errorf("RESTIC_PASSWORD = %q, want value from config file", got)
	}
}
