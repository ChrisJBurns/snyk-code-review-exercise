package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"

	"github.com/Masterminds/semver/v3"
	"github.com/gorilla/mux"
)

func New() http.Handler {
	router := mux.NewRouter()
	router.Handle("/package/{package}/{version}", http.HandlerFunc(packageHandler))
	return router
}

type npmPackageMetaResponse struct {
	Versions map[string]npmPackageResponse `json:"versions"`
}

type npmPackageResponse struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Dependencies map[string]string `json:"dependencies"`
}

type NpmPackageVersion struct {
	Name         string                        `json:"name"`
	Version      string                        `json:"version"`
	Dependencies map[string]*NpmPackageVersion `json:"dependencies"`
}

func packageHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	pkgName := vars["package"]
	pkgVersion := vars["version"]

	rootPkg := &NpmPackageVersion{Name: pkgName, Dependencies: map[string]*NpmPackageVersion{}}
	// depCache := map[string]*NpmPackageVersion{}
	var depCacheSync sync.Map
	var wg sync.WaitGroup
	if err := fetchAndResolveDependencies(&depCacheSync, rootPkg, pkgVersion, &wg); err != nil {
		println(err.Error())
		w.WriteHeader(500)
		return
	}

	wg.Wait()

	stringified, err := json.MarshalIndent(rootPkg, "", "  ")
	if err != nil {
		println(err.Error())
		w.WriteHeader(500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)

	// Ignoring ResponseWriter errors
	_, _ = w.Write(stringified)
}

func fetchAndResolveDependencies(depCacheSync *sync.Map, pkg *NpmPackageVersion, versionConstraint string, wg *sync.WaitGroup) error {
	if cachedDep, ok := depCacheSync.Load(pkg.Name + "-" + versionConstraint); ok {
		fmt.Printf("dep found: %v \n", pkg.Name+"-"+versionConstraint)
		p := cachedDep.(*NpmPackageVersion)
		pkg.Version = p.Version
		pkg.Name = p.Name
		pkg.Dependencies = p.Dependencies
		return nil
	}

	npmPkg, err := fetch(pkg, versionConstraint)
	if err != nil {
		return err
	}

	wg.Add(1)
	go resolveDependencies(depCacheSync, pkg, npmPkg, wg)
	return nil
}

func resolveDependencies(depCacheSync *sync.Map, pkg *NpmPackageVersion, npmPkg npmPackageResponse, wg *sync.WaitGroup) error {
	defer wg.Done()
	depCacheSync.Store(npmPkg.Name+"-"+npmPkg.Version, &NpmPackageVersion{
		Name:         npmPkg.Name,
		Version:      npmPkg.Version,
		Dependencies: make(map[string]*NpmPackageVersion),
	},
	)

	for dependencyName, dependencyVersionConstraint := range npmPkg.Dependencies {
		dep := &NpmPackageVersion{Name: dependencyName, Dependencies: map[string]*NpmPackageVersion{}}
		pkg.Dependencies[dependencyName] = dep
		if err := fetchAndResolveDependencies(depCacheSync, dep, dependencyVersionConstraint, wg); err != nil {
			return err
		}

		if depCacheEntry, ok := depCacheSync.Load(npmPkg.Name + "-" + npmPkg.Version); ok {
			depCacheEntryDependencies := depCacheEntry.(*NpmPackageVersion)
			depCacheEntryDependencies.Dependencies[dep.Name] = dep
		}
	}

	return nil
}

func fetch(pkg *NpmPackageVersion, versionConstraint string) (npmPackageResponse, error) {
	pkgMeta, err := fetchPackageMeta(pkg.Name)
	if err != nil {
		return npmPackageResponse{}, err
	}

	concreteVersion, err := highestCompatibleVersion(versionConstraint, pkgMeta)
	if err != nil {
		return npmPackageResponse{}, err
	}
	pkg.Version = concreteVersion

	npmPkg, err := fetchPackage(pkg.Name, pkg.Version)
	if err != nil {
		return npmPackageResponse{}, err
	}
	return *npmPkg, nil
}

func highestCompatibleVersion(constraintStr string, versions *npmPackageMetaResponse) (string, error) {
	constraint, err := semver.NewConstraint(constraintStr)
	if err != nil {
		return "", err
	}
	filtered := filterCompatibleVersions(constraint, versions)
	sort.Sort(filtered)
	if len(filtered) == 0 {
		return "", errors.New("no compatible versions found")
	}
	return filtered[len(filtered)-1].String(), nil
}

func filterCompatibleVersions(constraint *semver.Constraints, pkgMeta *npmPackageMetaResponse) semver.Collection {
	var compatible semver.Collection
	for version := range pkgMeta.Versions {
		semVer, err := semver.NewVersion(version)
		if err != nil {
			continue
		}
		if constraint.Check(semVer) {
			compatible = append(compatible, semVer)
		}
	}

	return compatible
}

func fetchPackage(name, version string) (*npmPackageResponse, error) {
	resp, err := http.Get(fmt.Sprintf("https://registry.npmjs.org/%s/%s", name, version))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var parsed npmPackageResponse
	_ = json.Unmarshal(body, &parsed)
	return &parsed, nil
}

func fetchPackageMeta(p string) (*npmPackageMetaResponse, error) {
	resp, err := http.Get(fmt.Sprintf("https://registry.npmjs.org/%s", p))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var parsed npmPackageMetaResponse
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		return nil, err
	}

	return &parsed, nil
}
