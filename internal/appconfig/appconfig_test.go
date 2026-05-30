package appconfig

import (
	"testing"
)

func TestDeriveResticBase(t *testing.T) {
	t.Setenv("RESTIC_REPOSITORY", "s3:https://bucket.s3.amazonaws.com/restic")

	got := deriveResticBase()
	want := "s3:https://bucket.s3.amazonaws.com/restic"
	if got != want {
		t.Errorf("deriveResticBase() = %q, want %q", got, want)
	}
}

func TestDeriveResticBaseTrailingSlash(t *testing.T) {
	t.Setenv("RESTIC_REPOSITORY", "s3:https://bucket.s3.amazonaws.com/restic/")

	got := deriveResticBase()
	want := "s3:https://bucket.s3.amazonaws.com/restic"
	if got != want {
		t.Errorf("deriveResticBase() = %q, want %q", got, want)
	}
}

func TestDeriveResticBaseEmpty(t *testing.T) {
	if got := deriveResticBase(); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestDeriveResticBaseWithWhitespace(t *testing.T) {
	t.Setenv("RESTIC_REPOSITORY", "  s3:https://bucket.s3.amazonaws.com/restic  ")

	got := deriveResticBase()
	want := "s3:https://bucket.s3.amazonaws.com/restic"
	if got != want {
		t.Errorf("deriveResticBase() = %q, want %q", got, want)
	}
}

func TestDeriveLockBucket_Explicit(t *testing.T) {
	t.Setenv("S3_LOCK_BUCKET", "my-lock-bucket")

	if got := deriveLockBucket(); got != "my-lock-bucket" {
		t.Errorf("deriveLockBucket() = %q, want %q", got, "my-lock-bucket")
	}
}

func TestDeriveLockBucket_FromResticRepo(t *testing.T) {
	t.Setenv("RESTIC_REPOSITORY", "s3:https://my-bucket.s3.amazonaws.com/restic/repo")

	got := deriveLockBucket()
	if got == "" {
		t.Fatal("expected non-empty bucket")
	}
}

func TestDeriveLockBucket_FromS3Endpoint(t *testing.T) {
	t.Setenv("S3_ENDPOINT", "http://localhost:9000/my-bucket")

	got := deriveLockBucket()
	if got == "" {
		t.Fatal("expected non-empty bucket")
	}
}

func TestDeriveLockBucket_Empty(t *testing.T) {
	if got := deriveLockBucket(); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestDeriveS3Endpoint_Explicit(t *testing.T) {
	t.Setenv("S3_ENDPOINT", "http://localhost:9000")

	got := deriveS3Endpoint()
	if got != "http://localhost:9000" {
		t.Errorf("deriveS3Endpoint() = %q, want %q", got, "http://localhost:9000")
	}
}

func TestDeriveS3Endpoint_FromResticRepo(t *testing.T) {
	t.Setenv("RESTIC_REPOSITORY", "s3:https://bucket.s3.amazonaws.com/restic")

	got := deriveS3Endpoint()
	if got == "" {
		t.Fatal("expected non-empty endpoint")
	}
}

func TestDeriveS3Endpoint_AlreadyHasScheme(t *testing.T) {
	t.Setenv("S3_ENDPOINT", "https://s3.amazonaws.com")

	got := deriveS3Endpoint()
	if got != "https://s3.amazonaws.com" {
		t.Errorf("deriveS3Endpoint() = %q, want %q", got, "https://s3.amazonaws.com")
	}
}

func TestDeriveS3Endpoint_Empty(t *testing.T) {
	if got := deriveS3Endpoint(); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}
