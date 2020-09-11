//Package layers provides an interface for processing AddonsLayers.
//go:generate mockgen -destination=mockLayers.go -package=layers -source=layers.go . Layer
package layers

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/fluxcd/pkg/untar"
	sourcev1 "github.com/fluxcd/source-controller/api/v1alpha1"
	"github.com/go-logr/logr"

	kraanv1alpha1 "github.com/fidelity/kraan/pkg/api/v1alpha1"
)

// MaxConditions is the maximum number of condtions to retain.
var (
	RootPath = "/data"
	ReposData = reposData{repos: map[string]Repo{}}
)

func init() {
	path, set := os.LookupEnv("DATA_PATH")
	if set {
		RootPath = path
	}
}

// List returns a map of workers keyed by cluster-entity name
func (r *reposData) List() map[string]Repo {
	r.RLock()
	defer r.RUnlock()
	return r.repos
}

// Get returns a worker or nil if not present
func (r *reposData) Get(name string) Repo {
	r.RLock()
	defer r.RUnlock()
	if repo, found := r.repos[name]; found {
		return repo
	}
	return nil
}

// Add adds a worker for an entity returning the new worker or existing one if already present
func (r *reposData) Add(name string, repo Repo) Repo {
	r.Lock()
	defer r.Unlock()
	if repo, found := r.repos[name]; found {
		return repo
	}
	r.repos[name] = &repoData{}
	return r.repos[name]
}

// Delete deletes a worker from the map of active workers
func (r *reposData) Delete(name string) {
	r.Lock()
	defer r.Unlock()
	if _, found := r.repos[name]; found {
		delete(r.repos, name)
	}
}

// Repos defines the interface for managing multiple instances of repository and revision data.
type Repos interface {
	Add(name string, repo Repo) Repo
	Get(name string) Repo
	Delete(name string)
	List() map[string]Repo
}

// reposData hold data about all repositories.
type reposData struct {
	repos      map[string]Repo
	ctx        context.Context
	log        logr.Logger
	Repos      `json:"-"`
	sync.RWMutex `json:"-"`
}

// Repo defines the interface for managing repository and revision data.
type Repo interface {
	GetName() string
	LinkData() error
	SyncRepo() error
}

// repoData hold data about a repository.
type repoData struct {
	repo       *sourcev1.GitRepository
	ctx        context.Context
	log        logr.Logger
	Repo       `json:"-"`
	sync.RWMutex `json:"-"`
}

// NewRepo creates a layer object.
func NewRepo(ctx context.Context, log logr.Logger, repo *sourcev1.GitRepository) Repo {
	r := newRepo(ctx, log, repo)
	ReposData.Add(repo.Name, r)
	return r
}

// NewRepo creates a layer object.
func newRepo(ctx context.Context, log logr.Logger, repo *sourcev1.GitRepository) Repo {
	r := &repoData{
		ctx:  ctx,
		log:  log,
		repo: repo,
	}
	return r
}


func LinkData(addon *kraanv1alpha1.AddonsLayer, dataPath string) error {
	addonsPath := GetSourcePath(addon)
	addonsData := fmt.Sprintf("%s/%s", dataPath, addon.Spec.Source.Path)

	info, err := os.Stat(addonsData)
	if os.IsNotExist(err) {
		//reconciler.Log.Error(err, fmt.Sprintf("addons layer: %s, directory path not found in repository data", addon.Name))
		return err
	}
	if err != nil {
		//reconciler.Log.Error(err, fmt.Sprintf("failed to stat addons Data directory: %s", addonsData))
		return err
	}
	if !info.IsDir() {
		err := fmt.Errorf("addons Data path: %s, is not a directory", addonsData)
		//reconciler.Log.Error(err, "invalid path")
		return err
	}
	if err := os.Link(addonsPath, addonsData); err != nil {
		//reconciler.Log.Error(err, fmt.Sprintf("unable link to new data for addonsLayers: %s", addon.Name))
		return err
	}
	return nil
}

/*
Copyright 2020 The Flux CD contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

func SyncRepo() error {
	// set timeout for the reconciliation
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	log.Info("New revision detected", "revision", repository.Status.Artifact.Revision)

	dataPath := fmt.Sprintf("%s/repos/%s/%s", RootPath, repository.Name, repository.Status.Artifact.Revision)
	err := os.MkdirAll(dataPath, os.ModePerm)
	if err != nil {
		return "", fmt.Errorf("failed to create dir, error: %w", err)
	}

	// download and extract artifact
	summary, err := fetchArtifact(ctx, repository, dataPath)
	if err != nil {
		return "", err
	}
	log.Info("fetched artifact", "summary", summary)
	// list artifact content
	files, err := ioutil.ReadDir(dataPath)
	if err != nil {
		return "", fmt.Errorf("faild to list files, error: %w", err)
	}
	for _, file := range files {
		log.Info("unpacked", "file", file)
	}
	return dataPath, nil
}

func fetchArtifact(ctx context.Context, repository *sourcev1.GitRepository, dir string) (string, error) {
	if repository.Status.Artifact == nil {
		return "", fmt.Errorf("repository %s does not containt an artifact", repository.Name)
	}

	url := repository.Status.Artifact.URL

	// for local run:
	// kubectl -n gitops-system port-forward svc/source-controller 8080:80
	// export SOURCE_HOST=localhost:8080
	if hostname := os.Getenv("SOURCE_HOST"); hostname != "" {
		url = fmt.Sprintf("http://%s/gitrepository/%s/%s/latest.tar.gz", hostname, repository.Namespace, repository.Name)
	}

	// download the tarball
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request, error: %w", err)
	}

	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		return "", fmt.Errorf("failed to download artifact from %s, error: %w", url, err)
	}
	defer resp.Body.Close()

	// check response
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("faild to download artifact, status: %s", resp.Status)
	}

	// extract
	summary, err := untar.Untar(resp.Body, dir)
	if err != nil {
		return "", fmt.Errorf("faild to untar artifact, error: %w", err)
	}

	return summary, nil
}
