package driver

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/docker/go-plugins-helpers/volume"
	"github.com/example/blt-volume-manager/locker"
	"github.com/example/blt-volume-manager/restic"
)

type VolumeInfo struct {
	Name     string
	Path     string
	Lock     locker.Lock
	attached int
	cancel   context.CancelFunc
}

type Driver struct {
	root   string
	locker locker.Locker
	restic *restic.Manager
	vols   map[string]*VolumeInfo
	mu     sync.Mutex
}

func NewDriver(root string, lockBucket string, s3Endpoint string, s3Region string) *Driver {
	var l locker.Locker
	if lockBucket != "" {
		if sl, err := locker.NewS3Locker(lockBucket, s3Endpoint, s3Region); err == nil {
			l = sl
		} else {
			log.Printf("failed to init s3 locker, falling back to file locker: %v", err)
			l = locker.NewFileLocker(filepath.Join(root, "locks"))
		}
	} else {
		l = locker.NewFileLocker(filepath.Join(root, "locks"))
	}
	r := restic.NewManager()
	return &Driver{
		root:   root,
		locker: l,
		restic: r,
		vols:   make(map[string]*VolumeInfo),
	}
}

func (d *Driver) ResticManager() *restic.Manager {
	return d.restic
}

func (d *Driver) Create(r *volume.CreateRequest) error {
	name := r.Name
	volPath := filepath.Join(d.root, "volumes", name)
	if err := os.MkdirAll(volPath, 0755); err != nil {
		return err
	}

	// Attempt to acquire lock and restore if requested
	ctx := context.Background()
	lock, err := d.locker.Acquire(ctx, name)
	if err == nil {
		// we acquired lock; pull backup if exists
		restoreMode := "latest"
		if r.Options != nil {
			restoreMode = r.Options["restore"]
		}
		if restoreMode == "" {
			restoreMode = "latest"
		}
		if err := d.restic.RestoreIfExists(volPath, restoreMode); err != nil {
			log.Printf("restore failed: %v", err)
		}
		lock.Release()
	} else {
		// couldn't acquire lock; continue without restore
		log.Printf("create: couldn't acquire lock for %s: %v", name, err)
	}

	return nil
}

func (d *Driver) Remove(r *volume.RemoveRequest) error {
	name := r.Name
	d.mu.Lock()
	vi, ok := d.vols[name]
	d.mu.Unlock()

	// Best-effort: make a final backup, then remove local dir and release lock
	volPath := filepath.Join(d.root, "volumes", name)
	if err := d.restic.Backup(volPath, "cold"); err != nil {
		log.Printf("final backup before remove failed: %v", err)
	}
	if err := os.RemoveAll(volPath); err != nil {
		return err
	}
	if ok && vi.Lock != nil {
		vi.Lock.Release()
	}
	return nil
}

func (d *Driver) Mount(r *volume.MountRequest) (*volume.MountResponse, error) {
	name := r.Name
	volPath := filepath.Join(d.root, "volumes", name)
	if err := os.MkdirAll(volPath, 0755); err != nil {
		return nil, err
	}

	d.mu.Lock()
	vi, ok := d.vols[name]
	if !ok {
		vi = &VolumeInfo{Name: name, Path: volPath}
		d.vols[name] = vi
	}
	vi.attached++
	d.mu.Unlock()

	// Ensure lock
	ctx := context.Background()
	lock, err := d.locker.Acquire(ctx, name)
	if err != nil {
		log.Printf("Mount: failed to acquire lock: %v", err)
	} else {
		vi.Lock = lock
		// Start scheduled backups while mounted
		hot := time.Minute * 5
		cold := time.Hour * 24
		ctx2, cancel := context.WithCancel(context.Background())
		vi.cancel = cancel
		d.restic.StartSchedule(ctx2, name, volPath, hot, cold)
	}

	return &volume.MountResponse{Mountpoint: volPath}, nil
}

func (d *Driver) Unmount(r *volume.UnmountRequest) error {
	name := r.Name
	d.mu.Lock()
	vi, ok := d.vols[name]
	if ok {
		vi.attached--
		if vi.attached <= 0 {
			// final backup
			if err := d.restic.Backup(vi.Path, "hot"); err != nil {
				log.Printf("unmount final backup failed: %v", err)
			}
			if vi.cancel != nil {
				vi.cancel()
			}
			// retain lock per spec
		}
	}
	d.mu.Unlock()
	return nil
}

func (d *Driver) Path(r *volume.PathRequest) (*volume.PathResponse, error) {
	volPath := filepath.Join(d.root, "volumes", r.Name)
	return &volume.PathResponse{Mountpoint: volPath}, nil
}

func (d *Driver) Get(r *volume.GetRequest) (*volume.GetResponse, error) {
	volPath := filepath.Join(d.root, "volumes", r.Name)
	d.mu.Lock()
	vi, ok := d.vols[r.Name]
	d.mu.Unlock()
	state := "unlocked"
	attached := 0
	if ok {
		attached = vi.attached
		if vi.Lock != nil {
			state = "locked"
		}
	}
	return &volume.GetResponse{Volume: &volume.Volume{Name: r.Name, Mountpoint: volPath, Status: map[string]interface{}{"state": state, "attached": fmt.Sprintf("%d", attached)}}}, nil
}

func (d *Driver) List() (*volume.ListResponse, error) {
	root := filepath.Join(d.root, "volumes")
	entries, _ := os.ReadDir(root)
	vols := make([]*volume.Volume, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(root, e.Name())
		vols = append(vols, &volume.Volume{Name: e.Name(), Mountpoint: p})
	}
	return &volume.ListResponse{Volumes: vols}, nil
}

func (d *Driver) Capabilities() *volume.CapabilitiesResponse {
	return &volume.CapabilitiesResponse{Capabilities: volume.Capability{Scope: "local"}}
}

// utility to run a shell command (used by restic wrapper sometimes)
func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
