package metadata

import "strings"

func (s *Store) ListRegisteredVolumes() ([]string, error) {
	objects, err := s.store.ListObjects(RegisteredVolumesPrefix)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, obj := range objects {
		if obj.Key == nil {
			continue
		}
		name := strings.TrimPrefix(*obj.Key, RegisteredVolumesPrefix)
		name = strings.TrimSuffix(name, ".json")
		if name != "" {
			names = append(names, name)
		}
	}
	return names, nil
}

func (s *Store) WriteRegisteredVolume(name string) error {
	return s.store.PutObject(RegisteredVolumesPrefix+name+".json", nil)
}

func (s *Store) DeleteRegisteredVolume(name string) error {
	return s.store.DeleteObject(RegisteredVolumesPrefix + name + ".json")
}
