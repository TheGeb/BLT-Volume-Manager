package appconfig

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	DataDir    string
	ResticBase string
	S3Bucket   string
	S3Endpoint string
	S3Region   string
}

func FromEnv(dataDir string) (Config, error) {
	abs, err := filepath.Abs(dataDir)
	if err != nil {
		return Config{}, err
	}

	return Config{
		DataDir:    abs,
		ResticBase: deriveResticBase(),
		S3Bucket:   deriveLockBucket(),
		S3Endpoint: deriveS3Endpoint(),
		S3Region:   os.Getenv("S3_REGION"),
	}, nil
}

func deriveResticBase() string {
	repo := strings.TrimSpace(os.Getenv("RESTIC_REPOSITORY"))
	if repo == "" {
		return ""
	}
	return strings.TrimSuffix(repo, "/")
}

func deriveLockBucket() string {
	if b := os.Getenv("S3_LOCK_BUCKET"); b != "" {
		return b
	}

	if repo := os.Getenv("RESTIC_REPOSITORY"); repo != "" {
		repo = strings.TrimPrefix(repo, "s3:")
		if u, err := url.Parse(repo); err == nil && u.Path != "" {
			return strings.Trim(u.Path, "/")
		}
	}

	if ep := os.Getenv("S3_ENDPOINT"); ep != "" {
		if u, err := url.Parse(ep); err == nil && u.Path != "" {
			return strings.Trim(u.Path, "/")
		}
	}

	return ""
}

func deriveS3Endpoint() string {
	if ep := os.Getenv("S3_ENDPOINT"); ep != "" {
		return ep
	}
	repo := os.Getenv("RESTIC_REPOSITORY")
	if repo == "" {
		return ""
	}
	repo = strings.TrimPrefix(repo, "s3:")
	if !strings.Contains(repo, "://") {
		return ""
	}
	u, err := url.Parse(repo)
	if err != nil || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}
