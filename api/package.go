package api

import (
	"errors"
	"fmt"
	"net/http"
)

type Package struct{}

type NpmPackageMetaResponse struct {
	Versions map[string]NpmPackageResponse `json:"versions"`
}

type NpmPackageResponse struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Dependencies map[string]string `json:"dependencies"`
}

// FetchMeta gets all metadata information about a `pkg`. Versions of dependencies are returned
func (p *Package) FetchMeta(pkg string) (*NpmPackageMetaResponse, error) {
	resp, err := http.Get(fmt.Sprintf("https://registry.npmjs.org/%s", pkg))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == 404 {
		return nil, &ErrorResponse{StatusCode: resp.StatusCode, Err: errors.New("package not found")}
	}
	defer resp.Body.Close()

	var parsed NpmPackageMetaResponse
	converter.Unmarshall(resp.Body, &parsed)
	return &parsed, nil
}

// FetchPackage will return the package based on the package name and version provided
func (p *Package) FetchPackage(name, version string) (*NpmPackageResponse, error) {
	resp, err := http.Get(fmt.Sprintf("https://registry.npmjs.org/%s/%s", name, version))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == 404 {
		return nil, &ErrorResponse{StatusCode: resp.StatusCode, Err: errors.New("package not found")}
	}

	defer resp.Body.Close()

	var parsed NpmPackageResponse
	err = converter.Unmarshall(resp.Body, &parsed)
	if err != nil {
		return nil, err
	}

	return &parsed, nil
}
