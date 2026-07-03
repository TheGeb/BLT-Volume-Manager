package integration

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

type goPackage struct {
	ImportPath string   `json:"ImportPath"`
	Imports    []string `json:"Imports"`
}

func TestNoWebDriverDependency(t *testing.T) {
	webPkgs, err := listPackages("github.com/TheGeb/BLT-Volume-Manager/internal/web/...")
	if err != nil {
		t.Fatal(err)
	}

	for _, pkg := range webPkgs {
		for _, imp := range pkg.Imports {
			if strings.Contains(imp, "internal/driver") {
				t.Errorf("web package %q imports driver package %q", pkg.ImportPath, imp)
			}
		}
	}
}

func TestBackendOnlyInMetadata(t *testing.T) {
	backendPath := "github.com/TheGeb/BLT-Volume-Manager/internal/metadata/backend"
	allowedPrefixes := []string{
		"github.com/TheGeb/BLT-Volume-Manager/internal/metadata",
		"github.com/TheGeb/BLT-Volume-Manager/internal/cfg",
	}

	allPkgs, err := listPackages("github.com/TheGeb/BLT-Volume-Manager/internal/...")
	if err != nil {
		t.Fatal(err)
	}

	for _, pkg := range allPkgs {
		if isTestPkg(pkg.ImportPath) {
			continue
		}
		for _, imp := range pkg.Imports {
			if imp != backendPath {
				continue
			}
			allowed := false
			for _, prefix := range allowedPrefixes {
				if strings.HasPrefix(pkg.ImportPath, prefix) {
					allowed = true
					break
				}
			}
			if !allowed {
				t.Errorf("package %q imports %q; only %v may import the raw backend",
					pkg.ImportPath, backendPath, allowedPrefixes)
			}
		}
	}
}

func isTestPkg(path string) bool {
	return strings.HasSuffix(path, "_test") || strings.Contains(path, "_test/")
}

func TestNoDriverWebDependency(t *testing.T) {
	driverPkgs, err := listPackages("github.com/TheGeb/BLT-Volume-Manager/internal/driver/...")
	if err != nil {
		t.Fatal(err)
	}

	for _, pkg := range driverPkgs {
		for _, imp := range pkg.Imports {
			if strings.Contains(imp, "internal/web") {
				t.Errorf("driver package %q imports web package %q", pkg.ImportPath, imp)
			}
		}
	}
}

func listPackages(pattern string) ([]goPackage, error) {
	cmd := exec.Command("go", "list", "-json", pattern)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(strings.NewReader(string(output)))
	var pkgs []goPackage
	for dec.More() {
		var pkg goPackage
		if err := dec.Decode(&pkg); err != nil {
			return nil, err
		}
		pkgs = append(pkgs, pkg)
	}
	return pkgs, nil
}
