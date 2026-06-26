package cfg

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

func TestDeriveOwnerBucket_Explicit(t *testing.T) {
	t.Setenv("METADATA_S3_BUCKET", "my-owner-bucket")

	if got := deriveOwnerBucket(); got != "my-owner-bucket" {
		t.Errorf("deriveOwnerBucket() = %q, want %q", got, "my-owner-bucket")
	}
}

func TestDeriveOwnerBucket_FromResticRepo(t *testing.T) {
	t.Setenv("RESTIC_REPOSITORY", "s3:https://my-bucket.s3.amazonaws.com/restic/repo")

	got := deriveOwnerBucket()
	if got == "" {
		t.Fatal("expected non-empty bucket")
	}
}

func TestDeriveOwnerBucket_FromS3Endpoint(t *testing.T) {
	t.Setenv("S3_ENDPOINT", "http://localhost:9000/my-bucket")

	got := deriveOwnerBucket()
	if got == "" {
		t.Fatal("expected non-empty bucket")
	}
}

func TestDeriveOwnerBucket_Empty(t *testing.T) {
	if got := deriveOwnerBucket(); got != "" {
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
