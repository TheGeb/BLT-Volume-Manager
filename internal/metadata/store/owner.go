package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/app/log"
	"github.com/TheGeb/BLT-Volume-Manager/internal/s3"
)

const OwnerKeyspace = "blt-volume-manager/owners/"

const DefaultOwnerTTL = 24 * time.Hour

type OwnerEntry struct {
	Name       string `json:"name"`
	ExpiryTime int64  `json:"expiry_time"`
}

type VolumeOwner struct {
	Volume   string
	Owner    string
	Creation int64
	Expiry   int64
}

type OwnerStore struct {
	b Backend
}

func NewOwnerStore(b Backend) *OwnerStore {
	return &OwnerStore{b: b}
}

func (s *OwnerStore) LockVolume(ctx context.Context, volumeName, ownerName string, expiry int64) (string, error) {
	coord, ok := s.b.(Coordinator)
	if ok {
		var ttl int64
		if expiry > 0 {
			ttl = expiry - time.Now().Unix()
			if ttl <= 0 {
				return "", fmt.Errorf("expiry must be in the future")
			}
		} else {
			ttl = 365 * 24 * 3600 * 10
		}
		return coord.AcquireLock(ctx, volumeName, ownerName, ttl)
	}
	folder := OwnerPrefix(volumeName)
	return AcquireOwnerLock(ctx, s.b, folder, ownerName, expiry)
}

func (s *OwnerStore) LockIsValid(ctx context.Context, key string) (bool, error) {
	coord, ok := s.b.(Coordinator)
	if ok {
		return coord.LockIsValid(ctx, key)
	}
	_, _, _, expiry, err := ParseOwnerKey(key)
	if err != nil {
		return false, fmt.Errorf("parse lock key: %w", err)
	}
	if expiry > 0 && expiry <= time.Now().Unix() {
		return false, nil
	}
	_, err = s.b.ReadObject(ctx, key)
	if err != nil {
		if errors.Is(err, ErrKeyNotFound) {
			return false, nil
		}
		return false, classifyErr(err, "read lock object")
	}
	return true, nil
}

func (s *OwnerStore) ReleaseLock(ctx context.Context, key string) error {
	coord, ok := s.b.(Coordinator)
	if ok {
		return coord.ReleaseLock(ctx, key)
	}
	return s.b.DeleteObject(ctx, key)
}

func (s *OwnerStore) FindForVolume(ctx context.Context, volumeName string) (*VolumeOwner, error) {
	if coord, ok := s.b.(Coordinator); ok {
		_, owner, creation, expiry, err := coord.FindLock(ctx, volumeName)
		if err != nil {
			if errors.Is(err, ErrKeyNotFound) {
				return &VolumeOwner{Volume: volumeName}, nil
			}
			return nil, classifyErr(err, "find lock")
		}
		return &VolumeOwner{Volume: volumeName, Owner: owner, Creation: creation, Expiry: expiry}, nil
	}
	objects, err := s.b.ListObjects(ctx, OwnerPrefix(volumeName))
	if err != nil {
		return nil, fmt.Errorf("list owner objects: %w", classifyErr(err, "list owner objects"))
	}

	objects = RemoveStaleObjects(ctx, s.b, objects, DefaultOwnerTTL)

	key, owner, creation, expiry := determineOwner(objects)
	if key == "" {
		return &VolumeOwner{Volume: volumeName}, nil
	}
	return &VolumeOwner{Volume: volumeName, Owner: owner, Creation: creation, Expiry: expiry}, nil
}

func (s *OwnerStore) ListAllGrouped(ctx context.Context) (map[string]VolumeOwner, error) {
	coord, ok := s.b.(Coordinator)
	if ok {
		return coord.ListAllLocks(ctx)
	}
	objects, err := s.b.ListObjects(ctx, OwnerKeyspace)
	if err != nil {
		return nil, classifyErr(err, "list owner keyspace")
	}

	objects = RemoveStaleObjects(ctx, s.b, objects, DefaultOwnerTTL)

	grouped := make(map[string][]s3.Object)
	for _, obj := range objects {
		if obj.Key == nil {
			continue
		}
		vol, _, _, _, err := ParseOwnerKey(*obj.Key)
		if err != nil || vol == "" {
			continue
		}
		grouped[vol] = append(grouped[vol], obj)
	}

	result := make(map[string]VolumeOwner, len(grouped))
	for vol, objs := range grouped {
		key, owner, creation, expiry := determineOwner(objs)
		if key != "" {
			result[vol] = VolumeOwner{Volume: vol, Owner: owner, Creation: creation, Expiry: expiry}
		}
	}
	return result, nil
}

func (s *OwnerStore) DeleteForVolume(ctx context.Context, volumeName string) error {
	if coord, ok := s.b.(Coordinator); ok {
		lockKey, _, _, _, fErr := coord.FindLock(ctx, volumeName)
		if fErr != nil && !errors.Is(fErr, ErrKeyNotFound) {
			return fmt.Errorf("find lock for volume: %w", fErr)
		}
		if fErr == nil {
			if err := coord.ReleaseLock(ctx, lockKey); err != nil {
				return fmt.Errorf("release lock for volume: %w", err)
			}
		}
	}
	return s.b.DeleteObjectsWithPrefix(ctx, OwnerPrefix(volumeName))
}

func (s *OwnerStore) AcquireForVolume(ctx context.Context, volumeName, ownerName string, durationMins int) (int64, error) {
	if ownerName == "" {
		return 0, fmt.Errorf("owner name is required")
	}
	var expiry int64
	if durationMins > 0 {
		expiry = time.Now().Add(time.Duration(durationMins) * time.Minute).Unix()
	}

	_, err := s.LockVolume(ctx, volumeName, ownerName, expiry)
	if err != nil {
		return 0, err
	}
	return expiry, nil
}

func encodeOwner(s string) string {
	return strings.ReplaceAll(s, "-", "%2D")
}

func decodeOwner(s string) string {
	return strings.ReplaceAll(s, "%2D", "-")
}

func AcquireOwnerLock(ctx context.Context, store Backend, folder, owner string, expiry int64) (myKey string, err error) {
	creation := time.Now().Unix()
	var durStr string
	switch {
	case expiry == 0:
		durStr = "0"
	case expiry > creation:
		d := time.Duration(expiry-creation) * time.Second
		durStr = formatDuration(d)
	default:
		return "", fmt.Errorf("expiry must be in the future or 0 for permanent")
	}
	encodedOwner := encodeOwner(owner)
	newKey := fmt.Sprintf("%s%s-%d-%s.json", folder, encodedOwner, creation, durStr)
	proposal := OwnerEntry{Name: owner, ExpiryTime: expiry}
	data, err := json.Marshal(proposal)
	if err != nil {
		return "", fmt.Errorf("marshal proposal: %w", err)
	}

	if err := store.PutObject(ctx, newKey, data); err != nil {
		return "", fmt.Errorf("create proposal: %w", classifyErr(err, "put proposal"))
	}
	defer func() {
		if err != nil {
			_ = store.DeleteObject(ctx, newKey)
		}
	}()

	objects, err := store.ListObjects(ctx, folder)
	if err != nil {
		return "", fmt.Errorf("list proposals: %w", classifyErr(err, "list proposals"))
	}

	winner, _, _, _ := determineOwner(objects)
	if winner != newKey {
		return "", ErrLockConflict
	}

	myKey = newKey
	return myKey, nil
}

func (o *OwnerEntry) RemainingSeconds() int64 {
	if o.ExpiryTime == 0 {
		return math.MaxInt64
	}
	return o.ExpiryTime - time.Now().Unix()
}

func OwnerPrefix(volumeName string) string {
	return OwnerKeyspace + volumeName + "/"
}

func parseDuration(s string) (time.Duration, error) {
	if len(s) == 0 {
		return 0, fmt.Errorf("empty duration string")
	}
	unit := s[len(s)-1]
	numStr := s[:len(s)-1]
	num, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse duration number: %w", err)
	}
	switch unit {
	case 's':
		return time.Duration(num) * time.Second, nil
	case 'm':
		return time.Duration(num) * time.Minute, nil
	case 'h':
		return time.Duration(num) * time.Hour, nil
	default:
		return 0, fmt.Errorf("unknown duration unit %c, expected s, m, or h", unit)
	}
}

func formatDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}
	totalSeconds := int64(d.Seconds())
	if totalSeconds%3600 == 0 {
		return fmt.Sprintf("%dh", totalSeconds/3600)
	}
	if totalSeconds%60 == 0 {
		return fmt.Sprintf("%dm", totalSeconds/60)
	}
	return fmt.Sprintf("%ds", totalSeconds)
}

func ParseOwnerKey(key string) (volume, owner string, creation int64, expiry int64, err error) {
	if !strings.HasPrefix(key, OwnerKeyspace) {
		return "", "", 0, 0, fmt.Errorf("key does not start with owner prefix")
	}
	s := strings.TrimPrefix(key, OwnerKeyspace)
	idx := strings.LastIndexByte(s, '/')
	if idx < 0 {
		return "", "", 0, 0, fmt.Errorf("no volume/owner separator in owner key: %q", key)
	}
	volume = s[:idx]
	rest := s[idx+1:]
	if !strings.HasSuffix(rest, ".json") {
		return "", "", 0, 0, fmt.Errorf("owner key missing .json suffix: %q", key)
	}
	rest = rest[:len(rest)-5]

	// Format: <owner>-<creation>-<duration>
	lastDash := strings.LastIndexByte(rest, '-')
	if lastDash < 0 {
		return "", "", 0, 0, fmt.Errorf("no dash separator in owner key: %q", key)
	}
	prevDash := strings.LastIndexByte(rest[:lastDash], '-')
	if prevDash < 0 {
		return "", "", 0, 0, fmt.Errorf("missing creation timestamp in owner key: %q", key)
	}

	owner = decodeOwner(rest[:prevDash])
	creation, err = strconv.ParseInt(rest[prevDash+1:lastDash], 10, 64)
	if err != nil {
		return "", "", 0, 0, fmt.Errorf("parse creation from owner key: %w", err)
	}
	durationStr := rest[lastDash+1:]

	if durationStr == "0" {
		expiry = 0
	} else {
		d, durErr := parseDuration(durationStr)
		if durErr != nil {
			return "", "", 0, 0, fmt.Errorf("parse duration from owner key: %w", durErr)
		}
		expiry = creation + int64(d.Seconds())
	}

	return volume, owner, creation, expiry, nil
}

func determineOwner(objects []s3.Object) (firstKey string, firstOwner string, firstCreation int64, firstExpiry int64) {
	sort.Slice(objects, func(i, j int) bool {
		ti, tj := objects[i].ModificationCounter, objects[j].ModificationCounter
		if ti != nil && tj != nil && *ti != *tj {
			return *ti < *tj
		}
		if ti == nil && tj != nil {
			return false
		}
		if ti != nil && tj == nil {
			return true
		}

		_, _, ci, _, erri := ParseOwnerKey(*objects[i].Key)
		_, _, cj, _, errj := ParseOwnerKey(*objects[j].Key)
		if erri == nil && errj == nil && ci != cj {
			return ci < cj
		}

		return *objects[i].Key < *objects[j].Key
	})

	for i := 1; i < len(objects); i++ {
		if objects[i].ModificationCounter != nil && objects[i-1].ModificationCounter != nil &&
			*objects[i].ModificationCounter == *objects[i-1].ModificationCounter {
			log.WarnfDev("duplicate_modification_counter",
				"Duplicate modification counters can be caused by S3 backends with 1 second timestamp resolution, which is not suitable for locks",
				"count=%d entries=%s %s",
				*objects[i].ModificationCounter,
				*objects[i-1].Key,
				*objects[i].Key)
		}
	}

	now := time.Now().Unix()
	for _, obj := range objects {
		if obj.Key == nil {
			continue
		}
		_, owner, creation, expiry, err := ParseOwnerKey(*obj.Key)
		if err != nil {
			continue
		}
		if expiry > 0 && expiry <= now {
			continue
		}
		return *obj.Key, owner, creation, expiry
	}
	return "", "", 0, 0
}

func RemoveStaleObjects(ctx context.Context, store Backend, objects []s3.Object, ttl time.Duration) []s3.Object {
	now := time.Now()
	kept := make([]s3.Object, 0, len(objects))
	for _, obj := range objects {
		if obj.Key == nil {
			continue
		}
		_, _, _, expiry, err := ParseOwnerKey(*obj.Key)
		isPermanent := err == nil && expiry == 0
		if err == nil && expiry > now.Unix() {
			kept = append(kept, obj)
			continue
		}
		if !isPermanent && obj.ModificationCounter != nil {
			modTime := time.Unix(0, *obj.ModificationCounter)
			if now.Sub(modTime) > ttl {
				_ = store.DeleteObject(ctx, *obj.Key)
				continue
			}
		}
		kept = append(kept, obj)
	}
	return kept
}
