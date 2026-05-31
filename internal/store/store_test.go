package store

import "testing"

func TestS3StoreConfigValidate(t *testing.T) {
	cfg := S3StoreConfig{S3Bucket: ""}
	if err := cfg.validate(); err == nil {
		t.Error("expected error for empty bucket name")
	}

	cfg2 := S3StoreConfig{S3Bucket: "my-bucket"}
	if err := cfg2.validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLockPrefixConstants(t *testing.T) {
	if LockPrefix != "blt-volume-manager/locks/" {
		t.Errorf("unexpected LockPrefix: %q", LockPrefix)
	}
	if VolumePrefix != "blt-volume-manager/registered-volumes/" {
		t.Errorf("unexpected VolumePrefix: %q", VolumePrefix)
	}
}
