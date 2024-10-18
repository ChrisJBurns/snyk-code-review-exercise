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

type SafeMap struct {
	mu sync.RWMutex
	m  map[string]*NpmPackageVersion
}

func (sm *SafeMap) Load(key string) (*NpmPackageVersion, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	val, ok := sm.m[key]
	return val, ok
}
func (sm *SafeMap) Store(key string, value *NpmPackageVersion) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.m == nil {
		sm.m = make(map[string]*NpmPackageVersion)
	}
	sm.m[key] = value
}

func (sm *SafeMap) StoreDependency(npmPkg npmPackageResponse, depName string, dep *NpmPackageVersion) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	key := npmPkg.Name + "-" + npmPkg.Version

	// initialise map if nil
	if sm.m == nil {
		sm.m = make(map[string]*NpmPackageVersion)
	}

	// if package does not exists in map for `key`, add it and it's dependency package.
	// if package does exist in map for `key`, add it's dependency package
	if _, ok := sm.m[key]; !ok {
		sm.m[key] = &NpmPackageVersion{
			Name:         npmPkg.Name,
			Version:      npmPkg.Version,
			Dependencies: make(map[string]*NpmPackageVersion),
		}
		sm.m[key].Dependencies[depName] = dep
	} else {
		sm.m[key].Dependencies[depName] = dep
	}
}

func packageHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	pkgName := vars["package"]
	pkgVersion := vars["version"]

	rootPkg := &NpmPackageVersion{Name: pkgName, Dependencies: map[string]*NpmPackageVersion{}}
	var safeMapCache SafeMap
	var wg sync.WaitGroup
	if err := fetchAndResolveDependencies(&safeMapCache, rootPkg, pkgVersion, &wg); err != nil {
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

func fetchAndResolveDependencies(safeMapCache *SafeMap, pkg *NpmPackageVersion, versionConstraint string, wg *sync.WaitGroup) error {
	if cachedDep, ok := safeMapCache.Load(pkg.Name + "-" + versionConstraint); ok {
		fmt.Printf("dep found: %v \n", pkg.Name+"-"+versionConstraint)
		pkg.Version = cachedDep.Version
		pkg.Name = cachedDep.Name
		pkg.Dependencies = cachedDep.Dependencies
		return nil
	}

	npmPkg, err := fetch(pkg, versionConstraint)
	if err != nil {
		return err
	}

	wg.Add(1)
	go resolveDependencies(safeMapCache, pkg, npmPkg, wg)
	return nil
}

func resolveDependencies(safeMapCache *SafeMap, pkg *NpmPackageVersion, npmPkg npmPackageResponse, wg *sync.WaitGroup) error {
	defer wg.Done()

	key := npmPkg.Name + "-" + npmPkg.Version
	if _, ok := safeMapCache.Load(key); !ok {
		safeMapCache.Store(key, &NpmPackageVersion{
			Name:         npmPkg.Name,
			Version:      npmPkg.Version,
			Dependencies: make(map[string]*NpmPackageVersion),
		})

		for depName, depVersionConstraint := range npmPkg.Dependencies {
			dep := &NpmPackageVersion{Name: depName, Dependencies: map[string]*NpmPackageVersion{}}
			pkg.Dependencies[depName] = dep
			if err := fetchAndResolveDependencies(safeMapCache, dep, depVersionConstraint, wg); err != nil {
				return err
			}
			safeMapCache.StoreDependency(npmPkg, dep.Name, dep)
		}
		return nil
	}
	return nil

	// for depName, depVersionConstraint := range npmPkg.Dependencies {
	// 	dep := &NpmPackageVersion{Name: depName, Dependencies: map[string]*NpmPackageVersion{}}
	// 	pkg.Dependencies[depName] = dep
	// 	if err := fetchAndResolveDependencies(safeMapCache, dep, depVersionConstraint, wg); err != nil {
	// 		return err
	// 	}
	// 	safeMapCache.StoreDependency(npmPkg, dep.Name, dep)
	// }
	// return nil
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
