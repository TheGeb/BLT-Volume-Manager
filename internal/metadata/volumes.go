package metadata

import "strings"

func (s *Store) ListVolumeMarkers(prefix string) ([]string, error) {
	objects, err := s.store.ListObjects(prefix)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, obj := range objects {
		if obj.Key == nil {
			continue
		}
		name := strings.TrimPrefix(*obj.Key, prefix)
		name = strings.TrimSuffix(name, ".json")
		if name != "" {
			names = append(names, name)
		}
	}
	return names, nil
}

func (s *Store) WriteVolumeMarker(name string) error {
	return s.store.PutObject(VolumesPrefix+name+".json", nil)
}

func (s *Store) DeleteVolumeMarker(name string) error {
	return s.store.DeleteObject(VolumesPrefix + name + ".json")
}
