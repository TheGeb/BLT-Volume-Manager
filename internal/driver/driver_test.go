package driver

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/app"
	"github.com/TheGeb/BLT-Volume-Manager/internal/cfg"
	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata/store"
	"github.com/TheGeb/BLT-Volume-Manager/internal/s3"
	"github.com/docker/go-plugins-helpers/volume"
)

// recordingBackend implements store.Backend for testing, tracking call counts.
type recordingBackend struct {
	mu         sync.Mutex
	entries    map[string][]byte
	order      []string
	putCallCnt int
}

func newRecordingBackend() *recordingBackend {
	return &recordingBackend{entries: make(map[string][]byte)}
}

func (b *recordingBackend) PutObject(_ context.Context, key string, data []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.putCallCnt++
	b.entries[key] = data
	b.order = append(b.order, key)
	return nil
}

func (b *recordingBackend) ReadObject(_ context.Context, key string) ([]byte, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	data, ok := b.entries[key]
	if !ok {
		return nil, store.ErrKeyNotFound
	}
	return data, nil
}

func (b *recordingBackend) DeleteObject(_ context.Context, key string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.entries, key)
	return nil
}

func (b *recordingBackend) ListObjects(_ context.Context, prefix string) ([]s3.Object, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	var objs []s3.Object
	for i, k := range b.order {
		// Only list keys that still exist: a "deleted" object must not appear.
		if _, ok := b.entries[k]; !ok {
			continue
		}
		if strings.HasPrefix(k, prefix) {
			key := k
			mc := int64(i + 1)
			objs = append(objs, s3.Object{Key: &key, ModificationCounter: &mc})
		}
	}
	return objs, nil
}

func (b *recordingBackend) DeleteObjectsWithPrefix(_ context.Context, prefix string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	for k := range b.entries {
		if strings.HasPrefix(k, prefix) {
			delete(b.entries, k)
		}
	}
	return nil
}

func (b *recordingBackend) PutCallCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.putCallCnt
}

func TestVolumeConfigReadWrite(t *testing.T) {
	t.Parallel()
	d := &Driver{volumePath: t.TempDir()}
	volPath := filepath.Join(d.volumePath, "volumes", "test-vol")
	if err := os.MkdirAll(volPath, app.DefaultDirPerm); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	cfg := &volumeConfig{FsType: "btrfs"}
	if err := d.writeVolumeConfig(volPath, cfg); err != nil {
		t.Fatalf("writeVolumeConfig: %v", err)
	}

	read := d.readVolumeConfig(volPath)
	if read == nil {
		t.Fatal("readVolumeConfig returned nil")
	}
	if read.FsType != "btrfs" {
		t.Errorf("FsType = %q, want %q", read.FsType, "btrfs")
	}
}

func TestVolumeConfigReadNonExistent(t *testing.T) {
	t.Parallel()
	d := &Driver{volumePath: t.TempDir()}
	missing := d.readVolumeConfig(filepath.Join(d.volumePath, "volumes", "nonexistent"))
	if missing != nil {
		t.Error("expected nil for missing config")
	}
}

func TestVolumeConfigDefaultFsType(t *testing.T) {
	t.Parallel()
	d := &Driver{volumePath: t.TempDir()}
	volPath := filepath.Join(d.volumePath, "volumes", "plain-vol")
	if err := os.MkdirAll(volPath, app.DefaultDirPerm); err != nil {
		t.Fatal(err)
	}

	if err := d.writeVolumeConfig(volPath, &volumeConfig{FsType: ""}); err != nil {
		t.Fatal(err)
	}
	read := d.readVolumeConfig(volPath)
	if read == nil {
		t.Fatal("expected non-nil config")
	}
	if read.FsType != "" {
		t.Errorf("expected empty FsType, got %q", read.FsType)
	}
}

func TestSnapVolumes(t *testing.T) {
	t.Parallel()
	d := &Driver{
		vols: map[string]*VolumeInfo{
			"vol1": {Name: "vol1", FsType: "btrfs"},
			"vol2": {Name: "vol2", FsType: ""},
			"vol3": {Name: "vol3", FsType: "zfs"},
		},
	}

	snaps := d.SnapVolumes()
	if len(snaps) != 2 {
		t.Fatalf("expected 2 snap volumes, got %d", len(snaps))
	}
	m := make(map[string]string)
	for _, sv := range snaps {
		m[sv.Name] = sv.FsType
	}
	if m["vol1"] != "btrfs" {
		t.Errorf("vol1 = %q, want btrfs", m["vol1"])
	}
	if m["vol3"] != "zfs" {
		t.Errorf("vol3 = %q, want zfs", m["vol3"])
	}
	if _, ok := m["vol2"]; ok {
		t.Error("vol2 should not be in snap volumes (no fs_type)")
	}
}

func TestSnapVolumesEmpty(t *testing.T) {
	t.Parallel()
	d := &Driver{vols: map[string]*VolumeInfo{}}
	snaps := d.SnapVolumes()
	if len(snaps) != 0 {
		t.Errorf("expected 0, got %d", len(snaps))
	}
}

func TestSnapVolumesNilMap(t *testing.T) {
	t.Parallel()
	d := &Driver{vols: nil}
	snaps := d.SnapVolumes()
	if len(snaps) != 0 {
		t.Errorf("expected 0, got %d", len(snaps))
	}
}

func TestCollectVolumeNames(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	d := &Driver{volumePath: root}

	volumesDir := filepath.Join(root, "volumes")

	vol1 := filepath.Join(volumesDir, "my-vol")
	if err := os.MkdirAll(vol1, app.DefaultDirPerm); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vol1, "volume.json"), []byte(`{}`), app.DefaultFilePerm); err != nil {
		t.Fatal(err)
	}

	vol2 := filepath.Join(volumesDir, "group", "nested-vol")
	if err := os.MkdirAll(vol2, app.DefaultDirPerm); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vol2, "volume.json"), []byte(`{}`), app.DefaultFilePerm); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(volumesDir, "no-config"), app.DefaultDirPerm); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(volumesDir, ".hidden-vol"), app.DefaultDirPerm); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(volumesDir, ".hidden-vol", "volume.json"), []byte(`{}`), app.DefaultFilePerm); err != nil {
		t.Fatal(err)
	}

	var names []string
	d.collectVolumeNames(volumesDir, "", &names)

	found := make(map[string]bool)
	for _, n := range names {
		found[n] = true
	}

	if !found["my-vol"] {
		t.Error("expected 'my-vol'")
	}
	if !found["group/nested-vol"] {
		t.Error("expected 'group/nested-vol'")
	}
	if found["no-config"] {
		t.Error("did not expect 'no-config' without volume.json")
	}
	if found[".hidden-vol"] {
		t.Error("did not expect '.hidden-vol' (starts with dot)")
	}
}

func TestCollectVolumeNamesEmpty(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	d := &Driver{volumePath: root}

	var names []string
	d.collectVolumeNames(filepath.Join(root, "volumes"), "", &names)
	if len(names) != 0 {
		t.Errorf("expected 0, got %d", len(names))
	}
}

func TestVolumeNames(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	d := &Driver{volumePath: root}

	for _, name := range []string{"vol-a", "vol-b"} {
		p := filepath.Join(root, "volumes", name)
		if err := os.MkdirAll(p, app.DefaultDirPerm); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(p, "volume.json"), []byte(`{}`), app.DefaultFilePerm); err != nil {
			t.Fatal(err)
		}
	}

	names := d.VolumeNames()
	if len(names) != 2 {
		t.Errorf("expected 2, got %d: %v", len(names), names)
	}
}

func TestResticManager(t *testing.T) {
	t.Parallel()
	d := &Driver{resticPath: "/data"}
	rm := d.ResticManager("test-vol")
	if rm == nil {
		t.Fatal("expected non-nil restic manager")
	}
	if rm.Repo() != "/data/restic/test-vol" {
		t.Errorf("repo = %q, want '/data/restic/test-vol'", rm.Repo())
	}
}

func TestNewDriverDefaults(t *testing.T) {
	t.Parallel()
	d := New(cfg.Config{DataDir: t.TempDir(), ResticBase: "/tmp/restic"}, context.Background())
	if d == nil {
		t.Fatal("expected non-nil driver")
	}
	if d.volumePath == "" {
		t.Error("expected non-empty root")
	}
	if d.vols == nil {
		t.Error("expected non-nil vols map")
	}
	if d.ownerStore != nil {
		t.Error("expected nil ownerStore (no S3)")
	}
	if d.lockMode != cfg.LockModeCreate {
		t.Errorf("expected default lock mode %q, got %q", cfg.LockModeCreate, d.lockMode)
	}
}

func TestList(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	d := &Driver{volumePath: root}

	for _, name := range []string{"vol-a", "vol-b", "group/nested-vol"} {
		p := filepath.Join(root, "volumes", name)
		if err := os.MkdirAll(p, app.DefaultDirPerm); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(p, "volume.json"), []byte(`{}`), app.DefaultFilePerm); err != nil {
			t.Fatal(err)
		}
	}

	resp, err := d.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if len(resp.Volumes) != 3 {
		t.Fatalf("expected 3 volumes, got %d", len(resp.Volumes))
	}

	seen := make(map[string]bool)
	for _, v := range resp.Volumes {
		seen[v.Name] = true
		if v.Name != "" && v.Mountpoint == "" {
			t.Errorf("volume %q has empty Mountpoint", v.Name)
		}
	}
	for _, name := range []string{"vol-a", "vol-b", "group/nested-vol"} {
		if !seen[name] {
			t.Errorf("expected volume %q in list", name)
		}
	}
}

func TestListEmpty(t *testing.T) {
	t.Parallel()
	d := &Driver{volumePath: t.TempDir()}
	resp, err := d.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if len(resp.Volumes) != 0 {
		t.Errorf("expected 0 volumes, got %d", len(resp.Volumes))
	}
}

func TestCapabilities(t *testing.T) {
	t.Parallel()
	d := &Driver{}
	cap := d.Capabilities()
	if cap == nil {
		t.Fatal("expected non-nil capabilities")
	}
	if cap.Capabilities.Scope != "local" {
		t.Errorf("expected 'local', got %q", cap.Capabilities.Scope)
	}
}

func TestPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	d := &Driver{volumePath: root}

	resp, err := d.Path(&volume.PathRequest{Name: "test-vol"})
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if resp.Mountpoint != filepath.Join(root, "volumes", "test-vol") {
		t.Errorf("Mountpoint = %q", resp.Mountpoint)
	}
}

func TestRemoveNonExistent(t *testing.T) {
	t.Parallel()
	d := &Driver{volumePath: t.TempDir()}
	err := d.Remove(&volume.RemoveRequest{Name: "no-such-vol"})
	if err != nil {
		t.Fatalf("Remove on non-existent volume: %v", err)
	}
}

func TestGetNonExistent(t *testing.T) {
	t.Parallel()
	d := &Driver{volumePath: t.TempDir()}
	resp, err := d.Get(&volume.GetRequest{Name: "no-such-vol"})
	if err != nil {
		t.Fatalf("Get on non-existent volume: %v", err)
	}
	if resp == nil || resp.Volume == nil {
		t.Fatal("expected non-nil response with Volume")
	}
	if resp.Volume.Name != "no-such-vol" {
		t.Errorf("Name = %q, want %q", resp.Volume.Name, "no-such-vol")
	}
	if resp.Volume.Mountpoint != filepath.Join(d.volumePath, "volumes", "no-such-vol") {
		t.Errorf("Mountpoint = %q", resp.Volume.Mountpoint)
	}
	if state, ok := resp.Volume.Status["state"].(string); !ok || state != "unclaimed" {
		t.Errorf("state = %q, want %q", state, "unclaimed")
	}
}

func TestUnmountNonExistent(t *testing.T) {
	t.Parallel()
	d := &Driver{}
	err := d.Unmount(&volume.UnmountRequest{Name: "no-such-vol", ID: "test"})
	if err != nil {
		t.Fatalf("Unmount on non-existent volume: %v", err)
	}
}

func TestConcurrentMountSingleVolume(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	b := newRecordingBackend()

	d := &Driver{
		volumePath:   dir,
		resticPath:   dir,
		ownerMaxMins: 10,
		vols:         make(map[string]*VolumeInfo),
		mountStates:  make(map[string]*volMountState),
		ownerStore:   store.NewOwnerStore(store.NewS3MetadataStore(b)),
	}

	volPath := VolumePath(dir, "concurrent-test")
	if err := os.MkdirAll(volPath, app.DefaultDirPerm); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var mountErr error

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := d.Mount(&volume.MountRequest{Name: "concurrent-test", ID: "test"})
			if err != nil {
				mu.Lock()
				mountErr = err
				mu.Unlock()
				return
			}
			if resp == nil || resp.Mountpoint != volPath {
				mu.Lock()
				mountErr = fmt.Errorf("unexpected mount response: %v", resp)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if mountErr != nil {
		t.Fatalf("Mount error: %v", mountErr)
	}

	if n := b.PutCallCount(); n != 1 {
		t.Errorf("expected 1 owner lock acquisition (PutObject), got %d", n)
	}

	// Fully unmount both references so the hot-backup schedule goroutine is
	// cancelled and the mount-state entry is cleaned up.
	for i := 0; i < 2; i++ {
		if err := d.Unmount(&volume.UnmountRequest{Name: "concurrent-test", ID: "test"}); err != nil {
			t.Fatalf("Unmount: %v", err)
		}
	}
}

// TestCreateInitLockTTL verifies init_lock_ttl_mins is parsed at Create and
// persisted to the volume config, and that the prefixed form wins over the
// legacy unprefixed spelling.
func TestCreateInitLockTTL(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	d := &Driver{
		volumePath:   dir,
		resticPath:   dir,
		lockMode:     cfg.LockModeMount,
		ownerMaxMins: 10,
		vols:         make(map[string]*VolumeInfo),
		mountStates:  make(map[string]*volMountState),
	}

	req := &volume.CreateRequest{
		Name:    "ttl-vol",
		Options: map[string]string{"init_lock_ttl_mins": "25"},
	}
	if err := d.Create(req); err != nil {
		t.Fatalf("Create: %v", err)
	}
	volPath := VolumePath(dir, "ttl-vol")
	cfg := d.readVolumeConfig(volPath)
	if cfg == nil || cfg.LockTTLMins != 25 {
		t.Fatalf("persisted LockTTLMins = %+v, want 25", cfg)
	}
	if got := d.lockExpiry(cfg.LockTTLMins); got != d.lockExpiry(25) {
		t.Errorf("lockExpiry does not honor persisted TTL")
	}
}

func TestOptionTTLMinsResolution(t *testing.T) {
	t.Parallel()
	d := &Driver{ownerMaxMins: 10}
	cases := []struct {
		name string
		opts map[string]string
		want int
	}{
		{"defaults to OWNER_MAX_MINS", nil, 10},
		{"prefixed wins", map[string]string{"init_lock_ttl_mins": "30", "lock_ttl_mins": "5"}, 30},
		{"invalid ignored", map[string]string{"init_lock_ttl_mins": "abc"}, 10},
		{"zero ignored", map[string]string{"init_lock_ttl_mins": "0"}, 10},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := d.optionTTLMins(c.opts); got != c.want {
				t.Errorf("optionTTLMins(%v) = %d, want %d", c.opts, got, c.want)
			}
		})
	}
}

// TestCreateLockModeCreate verifies that lock-on-creation acquires a permanent
// lock at Create, persists it in the volume config, and reuses it on Mount.
func TestCreateLockModeCreate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	b := newRecordingBackend()

	d := &Driver{
		volumePath:  dir,
		resticPath:  dir,
		lockMode:    cfg.LockModeCreate,
		vols:        make(map[string]*VolumeInfo),
		mountStates: make(map[string]*volMountState),
		ownerStore:  store.NewOwnerStore(store.NewS3MetadataStore(b)),
	}

	if err := d.Create(&volume.CreateRequest{Name: "create-vol"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if n := b.PutCallCount(); n != 1 {
		t.Fatalf("expected 1 lock PutObject on create, got %d", n)
	}

	volPath := VolumePath(dir, "create-vol")
	cfg := d.readVolumeConfig(volPath)
	if cfg == nil || cfg.LockKey == "" {
		t.Fatal("expected persisted lock key in volume config")
	}

	if _, err := d.Mount(&volume.MountRequest{Name: "create-vol", ID: "test"}); err != nil {
		t.Fatalf("Mount: %v", err)
	}
	if n := b.PutCallCount(); n != 1 {
		t.Fatalf("expected no re-acquisition on mount, PutObjects=%d", n)
	}

	if err := d.Unmount(&volume.UnmountRequest{Name: "create-vol", ID: "test"}); err != nil {
		t.Fatalf("Unmount: %v", err)
	}
}

// TestCreateLockModeCreateDuplicate verifies that a second Create of a volume
// we already own (Docker re-creates existing volumes) is treated as a success
// in lock-on-creation mode, reusing the persisted lock.
func TestCreateLockModeCreateDuplicate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	b := newRecordingBackend()

	d := &Driver{
		volumePath:  dir,
		resticPath:  dir,
		lockMode:    cfg.LockModeCreate,
		vols:        make(map[string]*VolumeInfo),
		mountStates: make(map[string]*volMountState),
		ownerStore:  store.NewOwnerStore(store.NewS3MetadataStore(b)),
	}

	if err := d.Create(&volume.CreateRequest{Name: "dup-vol"}); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	if err := d.Create(&volume.CreateRequest{Name: "dup-vol"}); err != nil {
		t.Fatalf("duplicate Create should succeed for owned volume: %v", err)
	}

	volPath := VolumePath(dir, "dup-vol")
	cfg := d.readVolumeConfig(volPath)
	if cfg == nil || cfg.LockKey == "" {
		t.Fatal("expected persisted lock key after duplicate create")
	}
	if _, err := b.ReadObject(context.Background(), cfg.LockKey); err != nil {
		t.Errorf("lock should still be held after duplicate create: %v", err)
	}
}

// TestRemoveLockModeCreate verifies Remove releases a lock persisted in the
// volume config even after a daemon restart (fresh in-memory state).
func TestRemoveLockModeCreate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	b := newRecordingBackend()

	d := &Driver{
		volumePath:  dir,
		resticPath:  dir,
		lockMode:    cfg.LockModeCreate,
		vols:        make(map[string]*VolumeInfo),
		mountStates: make(map[string]*volMountState),
		ownerStore:  store.NewOwnerStore(store.NewS3MetadataStore(b)),
	}

	if err := d.Create(&volume.CreateRequest{Name: "rm-vol"}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	volPath := VolumePath(dir, "rm-vol")
	volCfg := d.readVolumeConfig(volPath)
	if volCfg == nil || volCfg.LockKey == "" {
		t.Fatal("expected persisted lock key in volume config")
	}

	// Simulate a daemon restart: fresh driver, empty in-memory state.
	d2 := &Driver{
		volumePath:  dir,
		resticPath:  dir,
		lockMode:    cfg.LockModeCreate,
		vols:        make(map[string]*VolumeInfo),
		mountStates: make(map[string]*volMountState),
		ownerStore:  store.NewOwnerStore(store.NewS3MetadataStore(b)),
	}
	if err := d2.Remove(&volume.RemoveRequest{Name: "rm-vol"}); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if _, err := b.ReadObject(context.Background(), volCfg.LockKey); !errors.Is(err, store.ErrKeyNotFound) {
		t.Errorf("expected lock key to be released after remove, got err=%v", err)
	}
}

// TestMountLockModeCreateReacquiresPermanent verifies that when a create-mode
// volume's persisted lock is lost (e.g. released out-of-band or expired during
// a migration cutover), the mount re-acquires a PERMANENT lock - not a
// time-limited one - since renewal is a no-op in create mode.
func TestMountLockModeCreateReacquiresPermanent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	b := newRecordingBackend()

	d := &Driver{
		volumePath:   dir,
		resticPath:   dir,
		lockMode:     cfg.LockModeCreate,
		ownerMaxMins: 10,
		vols:         make(map[string]*VolumeInfo),
		mountStates:  make(map[string]*volMountState),
		ownerStore:   store.NewOwnerStore(store.NewS3MetadataStore(b)),
	}

	if err := d.Create(&volume.CreateRequest{Name: "perm-reacquire", Options: map[string]string{"init_lock_ttl_mins": "15"}}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	volPath := VolumePath(dir, "perm-reacquire")
	cfg := d.readVolumeConfig(volPath)
	if cfg == nil || cfg.LockKey == "" {
		t.Fatal("expected persisted lock key")
	}
	origKey := cfg.LockKey

	// Simulate lock loss (e.g. released out-of-band during cutover).
	if err := b.DeleteObject(context.Background(), origKey); err != nil {
		t.Fatal(err)
	}
	// Proposal keys encode the creation second; make sure the mount's
	// re-acquire lands on a different second than Create.
	time.Sleep(1100 * time.Millisecond)

	if _, err := d.Mount(&volume.MountRequest{Name: "perm-reacquire", ID: "test"}); err != nil {
		t.Fatalf("Mount after lock loss must succeed: %v", err)
	}
	cfg = d.readVolumeConfig(volPath)
	if cfg == nil || cfg.LockKey == "" || cfg.LockKey == origKey {
		t.Fatal("expected a fresh persisted lock key after re-acquire")
	}
	_, _, _, expiry, err := store.ParseOwnerKey(cfg.LockKey)
	if err != nil {
		t.Fatalf("ParseOwnerKey: %v", err)
	}
	if expiry != 0 {
		t.Errorf("re-acquired create-mode lock must be permanent, expiry = %d", expiry)
	}
	if valid, verr := d.ownerStore.LockIsValid(context.Background(), cfg.LockKey); verr != nil || !valid {
		t.Errorf("re-acquired lock invalid: valid=%v err=%v", valid, verr)
	}
}

// TestGetLockModeCreateConfigOwned verifies Get reports "owned" when a
// lock-on-creation volume has a persisted lock but is not in memory.
func TestGetLockModeCreateConfigOwned(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	d := &Driver{volumePath: dir, lockMode: cfg.LockModeCreate}

	volPath := VolumePath(dir, "get-vol")
	if err := os.MkdirAll(volPath, app.DefaultDirPerm); err != nil {
		t.Fatal(err)
	}
	lockKey := store.OwnerPrefix("get-vol") + "host-1700000000-0.json"
	if err := d.writeVolumeConfig(volPath, &volumeConfig{LockKey: lockKey}); err != nil {
		t.Fatal(err)
	}

	resp, err := d.Get(&volume.GetRequest{Name: "get-vol"})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	state, ok := resp.Volume.Status["state"].(string)
	if !ok || state != "owned" {
		t.Errorf("state = %q, want owned", state)
	}
}

// TestMountLockModeMountReacquires verifies lock-on-mount re-acquires the lock
// when the previously acquired (time-limited) lock has expired.
func TestMountLockModeMountReacquires(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	b := newRecordingBackend()

	d := &Driver{
		volumePath:   dir,
		resticPath:   dir,
		ownerMaxMins: 10,
		lockMode:     cfg.LockModeMount,
		vols:         make(map[string]*VolumeInfo),
		mountStates:  make(map[string]*volMountState),
		ownerStore:   store.NewOwnerStore(store.NewS3MetadataStore(b)),
	}

	volPath := VolumePath(dir, "reacquire-vol")
	if err := os.MkdirAll(volPath, app.DefaultDirPerm); err != nil {
		t.Fatal(err)
	}

	if _, err := d.Mount(&volume.MountRequest{Name: "reacquire-vol", ID: "test"}); err != nil {
		t.Fatalf("Mount: %v", err)
	}
	if n := b.PutCallCount(); n != 1 {
		t.Fatalf("expected 1 lock PutObject on first mount, got %d", n)
	}

	// Expire the acquired lock by rewriting the key's stored entry to one
	// whose encoded expiry is already past.
	d.mu.Lock()
	vi := d.vols["reacquire-vol"]
	oldKey := vi.LockKey
	d.mu.Unlock()
	vol, owner, _, _, err := store.ParseOwnerKey(oldKey)
	if err != nil {
		t.Fatalf("ParseOwnerKey: %v", err)
	}
	past := time.Now().Add(-time.Hour).Unix()
	expiredKey, err := store.OwnerProposalKey(vol, owner, past, past+10)
	if err != nil {
		t.Fatalf("OwnerProposalKey: %v", err)
	}
	_ = b.DeleteObject(context.Background(), oldKey)
	if err := b.PutObject(context.Background(), expiredKey, []byte(`{}`)); err != nil {
		t.Fatal(err)
	}

	if err := d.Unmount(&volume.UnmountRequest{Name: "reacquire-vol", ID: "test"}); err != nil {
		t.Fatalf("Unmount: %v", err)
	}

	before := b.PutCallCount()
	if _, err := d.Mount(&volume.MountRequest{Name: "reacquire-vol", ID: "test"}); err != nil {
		t.Fatalf("second Mount: %v", err)
	}
	if n := b.PutCallCount(); n != before+1 {
		t.Fatalf("expected re-acquisition on mount after lock expiry, PutObjects=%d (before=%d)", n, before)
	}

	if err := d.Unmount(&volume.UnmountRequest{Name: "reacquire-vol", ID: "test"}); err != nil {
		t.Fatalf("Unmount: %v", err)
	}
}
