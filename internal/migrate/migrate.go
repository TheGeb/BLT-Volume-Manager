// Package migrate implements the metadata backend migration tool. It is
// invoked as the "migrate" subcommand of the web binary.
package migrate

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/app"
	"github.com/TheGeb/BLT-Volume-Manager/internal/app/log"
	"github.com/TheGeb/BLT-Volume-Manager/internal/cfg"
	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata"
	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata/store"
	"github.com/TheGeb/BLT-Volume-Manager/internal/s3"
)

type backendFlags struct {
	kind      string
	bucket    string
	endpoint  string
	region    string
	forcePath bool
	etcdList  string
}

func (f *backendFlags) config() cfg.Config {
	return cfg.Config{
		MetadataBackend:  f.kind,
		S3Bucket:         f.bucket,
		S3Endpoint:       f.endpoint,
		S3Region:         f.region,
		S3ForcePathStyle: f.forcePath,
		EtcdEndpoints:    splitComma(f.etcdList),
	}
}

func (f *backendFlags) register(fs *flag.FlagSet, prefix string) {
	fs.StringVar(&f.kind, prefix+"type", "", "metadata backend type: s3 or etcd")
	fs.StringVar(&f.bucket, prefix+"bucket", "", "S3 bucket (for "+prefix+"type=s3)")
	fs.StringVar(&f.endpoint, prefix+"endpoint", "", "S3 endpoint (for "+prefix+"type=s3)")
	fs.StringVar(&f.region, prefix+"region", "", "S3 region (for "+prefix+"type=s3)")
	fs.BoolVar(&f.forcePath, prefix+"force-path-style", true, "use path-style S3 URLs (for "+prefix+"type=s3)")
	fs.StringVar(&f.etcdList, prefix+"etcd-endpoints", "", "comma-separated etcd endpoints (for "+prefix+"type=etcd)")
}

func splitComma(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(s, ",") {
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

type Result struct {
	OwnerLocks int  `json:"owner_locks"`
	Keys       int  `json:"keys"`
	DryRun     bool `json:"dry_run"`
}

// Run executes the migration with the given arguments (the same flags the
// standalone invocation accepts) and returns a process exit code.
func Run(args []string) int {
	var from, to backendFlags
	var dryRun bool
	fs := flag.NewFlagSet("blt-volume-manager-migrate", flag.ContinueOnError)
	from.register(fs, "from-")
	to.register(fs, "to-")
	fs.BoolVar(&dryRun, "dry-run", false, "print what would be migrated without writing anything")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if err := validateFlags(&from, &to); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		fs.Usage()
		return 1
	}

	ctx, stop := app.WithShutdown()
	defer stop()

	res, err := MigrateBackends(ctx, from.config(), to.config(), dryRun)
	if err != nil {
		log.Error("migration_failed", err)
		return 1
	}
	log.Infof("migration_complete",
		"owner_locks=%d keys=%d dry_run=%v",
		res.OwnerLocks, res.Keys, res.DryRun)
	return 0
}

// MigrateBackends opens the two metadata backends described by from and to,
// copies metadata between them, and closes them. Owner locks are refreshed and
// other keyspaces are copied verbatim (see Copy).
func MigrateBackends(ctx context.Context, from, to cfg.Config, dryRun bool) (Result, error) {
	src, err := cfg.OpenMetadataBackend(from)
	if err != nil {
		return Result{}, fmt.Errorf("open source backend: %w", err)
	}
	dst, err := cfg.OpenMetadataBackend(to)
	if err != nil {
		return Result{}, fmt.Errorf("open target backend: %w", err)
	}
	defer closeIfCloser(src)
	defer closeIfCloser(dst)

	res, err := Copy(ctx, src, dst, dryRun)
	res.DryRun = dryRun
	return res, err
}

func validateFlags(from, to *backendFlags) error {
	if from.kind == "" || to.kind == "" {
		return fmt.Errorf("--from-type and --to-type are required (s3 or etcd)")
	}
	for label, f := range map[string]*backendFlags{"from": from, "to": to} {
		if f.kind != "s3" && f.kind != "etcd" {
			return fmt.Errorf("--%s-type must be s3 or etcd, got %q", label, f.kind)
		}
		if f.kind == "s3" && f.bucket == "" {
			return fmt.Errorf("--%s-type=s3 requires --%s-bucket", label, label)
		}
		if f.kind == "etcd" && f.etcdList == "" {
			return fmt.Errorf("--%s-type=etcd requires --%s-etcd-endpoints", label, label)
		}
	}
	return nil
}

func closeIfCloser(b store.Backend) {
	if c, ok := b.(interface{ Close() error }); ok {
		_ = c.Close()
	}
}

// Copy copies all metadata keyspaces from src to dst. Owner locks are
// migrated per volume (winner only) with a refreshed expiry: permanent locks
// stay permanent and time-limited locks are re-anchored to the export time so
// they remain valid in the target. All other keys are copied verbatim.
func Copy(ctx context.Context, src store.MetadataStore, dst store.Backend, dryRun bool) (Result, error) {
	var res Result

	objs, err := src.ListObjects(ctx, store.KeyspaceRoot)
	if err != nil {
		return res, fmt.Errorf("list source keyspace: %w", err)
	}

	var otherObjs []s3.Object
	for _, obj := range objs {
		if obj.Key == nil || strings.HasPrefix(*obj.Key, store.OwnerKeyspace) {
			continue
		}
		otherObjs = append(otherObjs, obj)
	}

	// Owner locks are enumerated through the source backend so both storage
	// formats are handled: S3's timestamped proposal keys and etcd's single
	// "<volume>/lock" key (whose owner/expiry live in the value).
	owners, err := store.NewOwnerStore(src).ListAllGrouped(ctx)
	if err != nil {
		return res, fmt.Errorf("group owner locks: %w", err)
	}
	// Deterministic order for repeatable dry-runs.
	volumes := make([]string, 0, len(owners))
	for vol := range owners {
		volumes = append(volumes, vol)
	}
	sort.Strings(volumes)

	for _, vol := range volumes {
		vo := owners[vol]
		expiry, ok := refreshExpiry(vo.Expiry)
		if !ok {
			log.Infof("migrate_skip_expired_lock", "volume=%s", vol)
			continue
		}
		res.OwnerLocks++
		if dryRun {
			log.Infof("migrate_lock_dryrun", "volume=%s owner=%s expiry=%d", vol, vo.Owner, expiry)
			continue
		}
		if err := metadata.MigrateOwnerLock(ctx, dst, vol, vo.Owner, expiry); err != nil {
			if errors.Is(err, store.ErrLockConflict) {
				return res, fmt.Errorf("migrate owner lock for %q: target already holds an active lock with a different owner (aborting; resolve the conflict before re-running)", vol)
			}
			return res, fmt.Errorf("migrate owner lock for %q: %w", vol, err)
		}
	}

	for _, obj := range otherObjs {
		res.Keys++
		if dryRun {
			log.Infof("migrate_key_dryrun", "key=%s", *obj.Key)
			continue
		}
		data, err := src.ReadObject(ctx, *obj.Key)
		if err != nil {
			if errors.Is(err, store.ErrKeyNotFound) {
				continue
			}
			return res, fmt.Errorf("read source key %q: %w", *obj.Key, err)
		}
		if err := dst.PutObject(ctx, *obj.Key, data); err != nil {
			return res, fmt.Errorf("write target key %q: %w", *obj.Key, err)
		}
	}

	return res, nil
}

// refreshExpiry re-anchors a lock expiry to now so the migrated lock is valid
// in the target. Permanent locks (expiry 0) stay permanent. Returns false when
// the lock has already expired and should not be migrated.
func refreshExpiry(expiry int64) (int64, bool) {
	if expiry == 0 {
		return 0, true
	}
	now := time.Now().Unix()
	remaining := expiry - now
	if remaining <= 0 {
		return 0, false
	}
	return now + remaining, true
}
