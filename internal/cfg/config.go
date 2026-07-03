package cfg

import (
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	DataDir         string
	ResticBase      string
	MetadataBackend string // "s3" (default) or "etcd"
	S3Bucket        string
	S3Endpoint      string
	S3Region        string
	// S3ForcePathStyle controls whether to use path-style S3 URLs (s3://bucket/key)
	// instead of virtual-hosted-style (bucket.s3.amazonaws.com/key).
	// Most S3-compatible stores require path-style URLs.
	S3ForcePathStyle bool
	EtcdEndpoints    []string
	OwnerMaxMins     int
}

func FromEnv(dataDir string) (Config, error) { // FIXME: shouldn't this consider dotenv? And finalize env vars
	abs, err := filepath.Abs(dataDir)
	if err != nil {
		return Config{}, err
	}

	ownerMaxMins := 10
	if mv := os.Getenv("OWNER_MAX_MINS"); mv != "" {
		if v, err := strconv.Atoi(mv); err == nil && v > 0 {
			ownerMaxMins = v
		}
	}

	metaBackend := os.Getenv("BLT_METADATA_BACKEND")
	etcdEndpoints := parseEtcdEndpoints(os.Getenv("ETCD_ENDPOINTS"))

	return Config{
		DataDir:          abs,
		ResticBase:       deriveResticBase(),
		MetadataBackend:  metaBackend,
		S3Bucket:         deriveOwnerBucket(),
		S3Endpoint:       deriveS3Endpoint(),
		S3Region:         os.Getenv("S3_REGION"),
		S3ForcePathStyle: os.Getenv("S3_FORCE_PATH_STYLE") == "" || !strings.EqualFold(os.Getenv("S3_FORCE_PATH_STYLE"), "0") && !strings.EqualFold(os.Getenv("S3_FORCE_PATH_STYLE"), "false"),
		EtcdEndpoints:    etcdEndpoints,
		OwnerMaxMins:     ownerMaxMins,
	}, nil
}

func deriveResticBase() string {
	repo := strings.TrimSpace(os.Getenv("RESTIC_REPOSITORY"))
	if repo == "" {
		return ""
	}
	return strings.TrimSuffix(repo, "/")
}

func deriveOwnerBucket() string {
	if b := os.Getenv("METADATA_S3_BUCKET"); b != "" {
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

func parseEtcdEndpoints(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			if !strings.Contains(p, "://") {
				p = "http://" + p
			}
			out = append(out, p)
		}
	}
	return out
}
