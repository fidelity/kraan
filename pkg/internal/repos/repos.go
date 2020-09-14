//Package repos provides an interface for processing repositories.
//go:generate mockgen -destination=../mocks/mockrepo/mockRepos.go -package=mockrepos -source=repos.go . Repo Repos
package repos

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fluxcd/pkg/untar"
	sourcev1 "github.com/fluxcd/source-controller/api/v1alpha1"
	"github.com/go-logr/logr"

	"github.com/fidelity/kraan/pkg/internal/utils"
)

var (
	RootPath = "/data"
	HostName = ""
	TimeOut  = 15 * time.Second
)

// Repos defines the interface for managing multiple instances of repository and revision data.
type Repos interface {
	Add(srcRepo *sourcev1.GitRepository) Repo
	Get(name string) Repo
	Delete(name string)
	List() map[string]Repo
}

// reposData hold data about all repositories.
type reposData struct {
	repos        map[string]Repo
	ctx          context.Context
	log          logr.Logger
	Repos        `json:"-"`
	sync.RWMutex `json:"-"`
}

// NewRepos creates a repos object.
func NewRepos(ctx context.Context, log logr.Logger) Repos {
	r := &reposData{
		ctx: ctx,
		log: log,
	}
	r.repos = map[string]Repo{}
	return r
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
func (r *reposData) Add(srcRepo *sourcev1.GitRepository) Repo {
	r.Lock()
	defer r.Unlock()
	name := fmt.Sprintf("%s/%s/%s", srcRepo.GetNamespace(), srcRepo.GetName(), srcRepo.GetArtifact().Revision)
	if _, found := r.repos[name]; !found {
		r.repos[name] = newRepo(r.ctx, r.log, srcRepo)
	}
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

// Repo defines the interface for managing repository and revision data.
type Repo interface {
	GetName() string
	GetSourceName() string
	GetSourceNameSpace() string
	SyncRepo() error
	GetDataPath() string
	LinkData(layerPath, sourcePath string) error
}

// repoData hold data about a repository.
type repoData struct {
	repo         *sourcev1.GitRepository
	ctx          context.Context
	log          logr.Logger
	Repo         `json:"-"`
	sync.RWMutex `json:"-"`
}

// newRepo creates a repo.
func newRepo(ctx context.Context, log logr.Logger, repo *sourcev1.GitRepository) Repo {
	r := &repoData{
		ctx:  ctx,
		log:  log,
		repo: repo,
	}
	return r
}

func (r *repoData) getDataPath() string {
	return fmt.Sprintf("%s/%s/%s/%s", RootPath, r.repo.GetNamespace(), r.repo.GetName(), r.repo.GetArtifact().Revision)
}

func (r *repoData) GetDataPath() string {
	r.RLock()
	defer r.RUnlock()
	return r.getDataPath()
}

func (r *repoData) GetName() string {
	r.RLock()
	defer r.RUnlock()
	return fmt.Sprintf("%s/%s/%s", r.repo.GetNamespace(), r.repo.GetName(), r.repo.GetArtifact().Revision)
}

func (r *repoData) GetSourceName() string {
	r.RLock()
	defer r.RUnlock()
	return r.repo.GetName()
}

func (r *repoData) GetSourceNameSpace() string {
	r.RLock()
	defer r.RUnlock()
	return r.repo.GetNamespace()
}

func (r *repoData) LinkData(layerPath, sourcePath string) error {
	r.Lock()
	defer r.Unlock()
	addonsPath := fmt.Sprintf("%s/%s", r.getDataPath(), sourcePath)
	if err := utils.IsExistingDir(addonsPath); err != nil {
		return err
	}
	layerPathParts := strings.Split(layerPath, "/")
	layerPathDir := strings.Join(layerPathParts[:len(layerPathParts)-1], "/")

	if err := os.MkdirAll(layerPathDir, os.ModePerm); err != nil {
		return err
	}
	if _, err := os.Lstat(layerPath); err == nil {
		if e := os.Remove(layerPath); e != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := os.Symlink(addonsPath, layerPath); err != nil {
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

func (r *repoData) SyncRepo() error {
	r.Lock()
	defer r.Unlock()
	ctx, cancel := context.WithTimeout(r.ctx, TimeOut)
	defer cancel()

	r.log.Info("New revision detected", "revision", r.repo.Status.Artifact.Revision)

	dataPath := fmt.Sprintf("%s/repos/%s/%s", RootPath, r.repo.Name, r.repo.Status.Artifact.Revision)
	if _, err := os.Stat(dataPath); os.IsNotExist(err) {
		if e := os.RemoveAll(dataPath); e != nil {
			return fmt.Errorf("failed to remove dir, error: %w", e)
		}
	} else if err != nil {
		return err
	}
	if err := os.MkdirAll(dataPath, os.ModePerm); err != nil {
		return err
	}

	// download and extract artifact
	summary, err := r.fetchArtifact(ctx)
	if err != nil {
		return err
	}
	r.log.Info("fetched artifact", "summary", summary)
	// list artifact content
	files, err := ioutil.ReadDir(dataPath)
	if err != nil {
		return fmt.Errorf("faild to list files, error: %w", err)
	}
	for _, file := range files {
		r.log.Info("unpacked", "file", file)
	}
	return nil
}

func (r *repoData) fetchArtifact(ctx context.Context) (string, error) {
	if r.repo.Status.Artifact == nil {
		return "", fmt.Errorf("repository %s does not containt an artifact", r.repo.Name)
	}

	url := r.repo.Status.Artifact.URL

	if HostName != "" {
		url = fmt.Sprintf("http://%s/gitrepository/%s/%s/latest.tar.gz", HostName, r.repo.Namespace, r.repo.Name)
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
	summary, err := untar.Untar(resp.Body, r.getDataPath())
	if err != nil {
		return "", fmt.Errorf("faild to untar artifact, error: %w", err)
	}

	return summary, nil
}
