package main

import (
	"os"
	"testing"
)

func TestDeriveResticBase(t *testing.T) {
	os.Setenv("RESTIC_REPOSITORY", "s3:https://bucket.s3.amazonaws.com/restic")
	defer os.Unsetenv("RESTIC_REPOSITORY")

	got := deriveResticBase()
	want := "s3:https://bucket.s3.amazonaws.com/restic"
	if got != want {
		t.Errorf("deriveResticBase() = %q, want %q", got, want)
	}
}

func TestDeriveResticBaseTrailingSlash(t *testing.T) {
	os.Setenv("RESTIC_REPOSITORY", "s3:https://bucket.s3.amazonaws.com/restic/")
	defer os.Unsetenv("RESTIC_REPOSITORY")

	got := deriveResticBase()
	want := "s3:https://bucket.s3.amazonaws.com/restic"
	if got != want {
		t.Errorf("deriveResticBase() = %q, want %q", got, want)
	}
}

func TestDeriveResticBaseEmpty(t *testing.T) {
	os.Unsetenv("RESTIC_REPOSITORY")
	if got := deriveResticBase(); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestDeriveResticBaseWithWhitespace(t *testing.T) {
	os.Setenv("RESTIC_REPOSITORY", "  s3:https://bucket.s3.amazonaws.com/restic  ")
	defer os.Unsetenv("RESTIC_REPOSITORY")

	got := deriveResticBase()
	want := "s3:https://bucket.s3.amazonaws.com/restic"
	if got != want {
		t.Errorf("deriveResticBase() = %q, want %q", got, want)
	}
}

func TestDeriveLockBucket_Explicit(t *testing.T) {
	os.Clearenv()
	os.Setenv("S3_LOCK_BUCKET", "my-lock-bucket")
	defer os.Unsetenv("S3_LOCK_BUCKET")

	if got := deriveLockBucket(); got != "my-lock-bucket" {
		t.Errorf("deriveLockBucket() = %q, want %q", got, "my-lock-bucket")
	}
}

func TestDeriveLockBucket_FromResticRepo(t *testing.T) {
	os.Clearenv()
	os.Setenv("RESTIC_REPOSITORY", "s3:https://my-bucket.s3.amazonaws.com/restic/repo")
	defer os.Unsetenv("RESTIC_REPOSITORY")

	got := deriveLockBucket()
	if got == "" {
		t.Fatal("expected non-empty bucket")
	}
}

func TestDeriveLockBucket_FromS3Endpoint(t *testing.T) {
	os.Clearenv()
	os.Setenv("S3_ENDPOINT", "http://localhost:9000/my-bucket")
	defer os.Unsetenv("S3_ENDPOINT")

	got := deriveLockBucket()
	if got == "" {
		t.Fatal("expected non-empty bucket")
	}
}

func TestDeriveLockBucket_Empty(t *testing.T) {
	os.Clearenv()
	if got := deriveLockBucket(); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestDeriveS3Endpoint_Explicit(t *testing.T) {
	os.Clearenv()
	os.Setenv("S3_ENDPOINT", "http://localhost:9000")
	defer os.Unsetenv("S3_ENDPOINT")

	got := deriveS3Endpoint()
	if got != "http://localhost:9000" {
		t.Errorf("deriveS3Endpoint() = %q, want %q", got, "http://localhost:9000")
	}
}

func TestDeriveS3Endpoint_FromResticRepo(t *testing.T) {
	os.Clearenv()
	os.Setenv("RESTIC_REPOSITORY", "s3:https://bucket.s3.amazonaws.com/restic")
	defer os.Unsetenv("RESTIC_REPOSITORY")

	got := deriveS3Endpoint()
	if got == "" {
		t.Fatal("expected non-empty endpoint")
	}
}

func TestDeriveS3Endpoint_AlreadyHasScheme(t *testing.T) {
	os.Clearenv()
	os.Setenv("S3_ENDPOINT", "https://s3.amazonaws.com")
	defer os.Unsetenv("S3_ENDPOINT")

	got := deriveS3Endpoint()
	if got != "https://s3.amazonaws.com" {
		t.Errorf("deriveS3Endpoint() = %q, want %q", got, "https://s3.amazonaws.com")
	}
}

func TestDeriveS3Endpoint_Empty(t *testing.T) {
	os.Clearenv()
	if got := deriveS3Endpoint(); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}
