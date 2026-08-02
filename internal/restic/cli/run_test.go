package cli

import (
	"context"
	"os"
	"testing"
)

func TestCommandForRepo_DefaultsFromPasswordToPassword(t *testing.T) {
	t.Setenv("RESTIC_PASSWORD", "secret")
	fromPw, hadFromPw := os.LookupEnv("RESTIC_FROM_PASSWORD")
	t.Cleanup(func() {
		if hadFromPw {
			_ = os.Setenv("RESTIC_FROM_PASSWORD", fromPw)
		} else {
			_ = os.Unsetenv("RESTIC_FROM_PASSWORD")
		}
	})
	if err := os.Unsetenv("RESTIC_FROM_PASSWORD"); err != nil {
		t.Fatalf("unset RESTIC_FROM_PASSWORD: %v", err)
	}

	r := &Runner{Repo: "s3:http://example/bucket"}
	cmd, err := r.commandForRepo(context.Background(), "s3:http://example/bucket/dest", "copy")
	if err != nil {
		t.Fatalf("commandForRepo: %v", err)
	}
	if !envHas(cmd.Env, "RESTIC_FROM_PASSWORD=secret") {
		t.Fatalf("expected RESTIC_FROM_PASSWORD to default to RESTIC_PASSWORD, env=%v", cmd.Env)
	}
}

func TestCommandForRepo_KeepsExplicitFromPassword(t *testing.T) {
	t.Setenv("RESTIC_PASSWORD", "secret")
	t.Setenv("RESTIC_FROM_PASSWORD", "other")

	r := &Runner{Repo: "s3:http://example/bucket"}
	cmd, err := r.commandForRepo(context.Background(), "s3:http://example/bucket/dest", "copy")
	if err != nil {
		t.Fatalf("commandForRepo: %v", err)
	}
	if !envHas(cmd.Env, "RESTIC_FROM_PASSWORD=other") {
		t.Fatalf("expected explicit RESTIC_FROM_PASSWORD to be kept, env=%v", cmd.Env)
	}
}

func envHas(env []string, want string) bool {
	for _, e := range env {
		if e == want {
			return true
		}
	}
	return false
}
