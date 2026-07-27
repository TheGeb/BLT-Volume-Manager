package backend

import (
	"testing"
)

func TestConfigValidate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		cfg     S3Config
		wantErr bool
	}{
		{"valid", S3Config{S3Bucket: "my-bucket"}, false},
		{"empty bucket", S3Config{}, true},
		{"empty bucket with endpoint", S3Config{S3Endpoint: "http://localhost:9000"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.validate()
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
