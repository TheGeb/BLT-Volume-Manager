package store

import "testing"

func TestS3StoreOptsValidate(t *testing.T) {
	opts := S3StoreOpts{S3Bucket: ""}
	if err := opts.validate(); err == nil {
		t.Error("expected error for empty bucket name")
	}

	opts2 := S3StoreOpts{S3Bucket: "my-bucket"}
	if err := opts2.validate(); err != nil {
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
