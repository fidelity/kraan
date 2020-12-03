//Package repos provides an interface for processing repositories.
//go:generate mockgen -destination=../mocks/repos/mockRepos.go -package=mocks -source=repos.go . Repo,Repos
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

	sourcev1 "github.com/fluxcd/source-controller/api/v1beta1"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	"github.com/fidelity/kraan/pkg/common"
	"github.com/fidelity/kraan/pkg/internal/tarconsumer"
	"github.com/fidelity/kraan/pkg/logging"
)

var (
	DefaultRootPath = "/data"
	DefaultHostName = ""
	DefaultTimeOut  = 15 * time.Second
)

// Repos defines the interface for managing multiple instances of repository and revision data.
type Repos interface {
	Add(srcRepo *sourcev1.GitRepository) Repo
	Get(name string) Repo
	Delete(name string)
	List() map[string]Repo
	SetRootPath(path string)
	GetRootPath() string
	SetHostName(hostName string)
	SetTimeOut(timeOut time.Duration)
	SetHTTPClient(client *http.Client)
}

// reposData hold data about all repositories.
type reposData struct {
	repos        map[string]Repo
	ctx          context.Context
	log          logr.Logger
	rootPath     string
	hostName     string
	timeOut      time.Duration
	client       *http.Client
	Repos        `json:"-"`
	sync.RWMutex `json:"-"`
}

// NewRepos creates a repos object.
func NewRepos(ctx context.Context, log logr.Logger) Repos {
	logging.TraceCall(log)
	defer logging.TraceExit(log)
	return &reposData{
		repos:    make(map[string]Repo, 1),
		ctx:      ctx,
		log:      log,
		rootPath: DefaultRootPath,
		hostName: DefaultHostName,
		timeOut:  DefaultTimeOut,
		client:   &http.Client{},
	}
}

func PathKey(repo *sourcev1.GitRepository) string {
	return fmt.Sprintf("%s/%s", repo.GetNamespace(), repo.GetName())
}

func (r *reposData) SetRootPath(path string) {
	r.rootPath = path
}

func (r *reposData) GetRootPath() string {
	return r.rootPath
}

func (r *reposData) SetHostName(hostName string) {
	r.hostName = hostName
}

func (r *reposData) SetTimeOut(timeOut time.Duration) {
	r.timeOut = timeOut
}

func (r *reposData) SetHTTPClient(client *http.Client) {
	r.client = client
}

// List returns a map of repos keyed by repo label
func (r *reposData) List() map[string]Repo {
	logging.TraceCall(r.log)
	defer logging.TraceExit(r.log)
	r.RLock()
	defer r.RUnlock()
	return r.repos
}

// Get returns a worker or nil if not present
func (r *reposData) Get(name string) Repo {
	logging.TraceCall(r.log)
	defer logging.TraceExit(r.log)
	r.RLock()
	defer r.RUnlock()
	if repo, found := r.repos[name]; found {
		return repo
	}
	return nil
}

// Add adds a repo to the map of active repos
func (r *reposData) Add(repo *sourcev1.GitRepository) Repo {
	logging.TraceCall(r.log)
	defer logging.TraceExit(r.log)
	r.Lock()
	defer r.Unlock()
	key := PathKey(repo)
	rp, found := r.repos[key]
	if !found {
		r.repos[key] = r.newRepo(key, repo)
		return r.repos[key]
	}
	rp.SetGitRepo(repo, r.GetRootPath())
	return rp
}

// Delete deletes a repo from the map of active repos
func (r *reposData) Delete(name string) {
	logging.TraceCall(r.log)
	defer logging.TraceExit(r.log)
	r.Lock()
	defer r.Unlock()
	if repo, found := r.repos[name]; found {
		_ = repo.TidyAll() // nolint:errcheck // ok
		delete(r.repos, name)
	}
}

// Repo defines the interface for managing repository and revision data.
type Repo interface {
	GetSourceName() string
	GetSourceNameSpace() string
	SyncRepo() error
	IsSynced() bool
	TidyRepo() error
	TidyAll() error
	AddUser(name string)
	RemoveUser(namer string) bool
	LinkData(layerPath, sourcePath string) error
	GetGitRepo() *sourcev1.GitRepository
	GetPath() string
	GetDataPath() string
	GetLoadPath() string
	SetHostName(hostName string)
	SetGitRepo(src *sourcev1.GitRepository, rootPath string)
	SetHTTPClient(client *http.Client)
	SetTarConsumer(tarConsumer tarconsumer.TarConsumer)
	fetchArtifact(ctx context.Context) error
}

// repoData hold data about a repository.
type repoData struct {
	ctx          context.Context
	log          logr.Logger
	client       *http.Client
	hostName     string
	dataPath     string
	loadPath     string
	path         string
	repo         *sourcev1.GitRepository
	tarConsumer  tarconsumer.TarConsumer
	Repo         `json:"-"`
	sync.RWMutex `json:"-"`
	syncLock     sync.RWMutex
	users        []string
}

// newRepo creates a repo.
func (r *reposData) newRepo(path string, sourceRepo *sourcev1.GitRepository) Repo {
	logging.TraceCall(r.log)
	defer logging.TraceExit(r.log)
	url := "not set"
	revision := "none"
	if sourceRepo.Status.Artifact != nil {
		url = sourceRepo.Status.Artifact.URL
		revision = sourceRepo.Status.Artifact.Revision
	}
	repo := &repoData{
		ctx:         r.ctx,
		log:         r.log,
		client:      r.client,
		hostName:    r.hostName,
		dataPath:    fmt.Sprintf("%s/%s/%s", r.rootPath, path, revision),
		loadPath:    fmt.Sprintf("%s/load/%s/%s", r.rootPath, path, revision),
		path:        path,
		repo:        sourceRepo,
		tarConsumer: tarconsumer.NewTarConsumer(r.ctx, r.client, url),
		users:       []string{},
	}
	return repo
}

func (r *repoData) GetGitRepo() *sourcev1.GitRepository {
	r.RLock()
	defer r.RUnlock()
	return r.repo
}

func (r *repoData) GetPath() string {
	return r.path
}

func (r *repoData) GetDataPath() string {
	return r.dataPath
}

func (r *repoData) GetLoadPath() string {
	return r.loadPath
}

func (r *repoData) SetHostName(hostName string) {
	r.hostName = hostName
}

func (r *repoData) SetGitRepo(src *sourcev1.GitRepository, rootPath string) {
	logging.TraceCall(r.log)
	defer logging.TraceExit(r.log)
	r.syncLock.Lock()
	defer r.syncLock.Unlock()
	r.repo = src
	revision := "none"
	if r.repo.Status.Artifact != nil {
		revision = r.repo.Status.Artifact.Revision
	}
	r.dataPath = fmt.Sprintf("%s/%s/%s", rootPath, PathKey(src), revision)
	r.loadPath = fmt.Sprintf("%s/load/%s/%s", rootPath, PathKey(src), revision)
}

func (r *repoData) SetHTTPClient(client *http.Client) {
	r.client = client
}

func (r *repoData) SetTarConsumer(tarConsumer tarconsumer.TarConsumer) {
	r.tarConsumer = tarConsumer
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

func (r *repoData) TidyRepo() error {
	r.syncLock.Lock()
	defer r.syncLock.Unlock()
	if r.repo.Status.Artifact == nil {
		return fmt.Errorf("repository %s does not contain an artifact", r.path)
	}
	dataPathParts := strings.Split(r.GetDataPath(), "/")
	dataPathDir := strings.Join(dataPathParts[:len(dataPathParts)-1], "/")
	revision := dataPathParts[len(dataPathParts)-1]

	if err := r.removeDirs(dataPathDir, revision); err != nil {
		return errors.WithMessagef(err, "%s - failed to remove previous revisions", logging.CallerStr(logging.Me))
	}

	dataPathDir = strings.Join(dataPathParts[:len(dataPathParts)-2], "/")
	branch := dataPathParts[len(dataPathParts)-2]

	if err := r.removeDirs(dataPathDir, branch); err != nil {
		return errors.WithMessagef(err, "%s - failed to remove previous branches/tags", logging.CallerStr(logging.Me))
	}
	return nil
}

func (r *repoData) TidyAll() error {
	return r.tidyAll()
}

func (r *repoData) tidyAll() error {
	dataPathParts := strings.Split(r.GetDataPath(), "/")
	dirName := strings.Join(dataPathParts[:len(dataPathParts)-1], "/")

	if err := os.RemoveAll(dirName); err != nil {
		return errors.Wrapf(err, "%s - failed to remove directory: %s", logging.CallerStr(logging.Me), dirName)
	}
	return nil
}

func (r *repoData) RemoveUser(name string) bool {
	r.syncLock.Lock()
	defer r.syncLock.Unlock()
	r.users = common.RemoveString(r.users, name)
	return len(r.users) == 0
}

func (r *repoData) AddUser(name string) {
	r.syncLock.Lock()
	defer r.syncLock.Unlock()
	if !common.ContainsString(r.users, name) {
		r.users = append(r.users, name)
	}
}

func (r *repoData) removeDirs(path, exclude string) error {
	r.log.V(2).Info("processing directory",
		append(logging.GetGitRepoInfo(r.repo), append(logging.GetFunctionAndSource(logging.MyCaller), "path", path, "exclude", exclude)...)...)
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return errors.Wrapf(err, "%s -failed to read directory", logging.CallerStr(logging.Me))
	}

	for _, f := range files {
		if f.IsDir() {
			if f.Name() != exclude {
				dirName := fmt.Sprintf("%s/%s", path, f.Name())
				r.log.V(1).Info("removing directory", append(logging.GetGitRepoInfo(r.repo),
					append(logging.GetFunctionAndSource(logging.MyCaller), "path", dirName)...)...)
				if e := os.RemoveAll(dirName); e != nil {
					return errors.Wrapf(e, "%s - failed to remove directory: %s", logging.CallerStr(logging.Me), dirName)
				}
			}
		}
	}
	return nil
}

func (r *repoData) LinkData(layerPath, sourcePath string) error {
	logging.TraceCall(r.log)
	defer logging.TraceExit(r.log)
	r.Lock()
	defer r.Unlock()
	addonsPath := fmt.Sprintf("%s/%s", r.GetDataPath(), sourcePath)
	if err := isExistingDir(addonsPath); err != nil {
		return errors.Wrapf(err, "%s - failed, target directory does not exist", logging.CallerStr(logging.Me))
	}
	layerPathParts := strings.Split(layerPath, "/")
	layerPathDir := strings.Join(layerPathParts[:len(layerPathParts)-1], "/")

	if err := os.MkdirAll(layerPathDir, os.ModePerm); err != nil {
		return errors.Wrapf(err, "%s - failed to make directory: %s", logging.CallerStr(logging.Me), layerPathDir)
	}
	if _, err := os.Lstat(layerPath); err == nil {
		if e := os.RemoveAll(layerPath); e != nil {
			return errors.Wrapf(err, "%s - failed to remove link: %s", logging.CallerStr(logging.Me), layerPath)
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := os.Symlink(addonsPath, layerPath); err != nil {
		return errors.Wrapf(err, "%s - failed to create link: %s", logging.CallerStr(logging.Me), layerPath)
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

// removeIfExists removes a directory if it exists
func removeIfExists(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	if e := os.RemoveAll(path); e != nil {
		return errors.Wrapf(e, "%s - failed to remove directory", logging.CallerStr(logging.Me))
	}
	return nil
}

// removeRecreateDir removes a directory if it exists and then recreates it
func removeRecreateDir(path string) error {
	if e := removeIfExists(path); e != nil {
		return errors.WithMessagef(e, "%s - failed to remove directory before recreate", logging.CallerStr(logging.Me))
	}
	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		return errors.Wrapf(err, "%s - failed to make directory: %s", logging.CallerStr(logging.Me), path)
	}
	return nil
}

func isExistingDir(dataPath string) error {
	info, err := os.Stat(dataPath)
	if !os.IsNotExist(err) {
		return errors.Wrapf(err, "%s - failed to stat: %s", logging.CallerStr(logging.Me), dataPath)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("addons Data path: %s, is not a directory", dataPath)
	}

	return nil
}

func (r *repoData) IsSynced() bool {
	if err := isExistingDir(r.dataPath); err == nil {
		r.log.V(1).Info("Revision is synced", append(logging.GetGitRepoInfo(r.repo), logging.GetFunctionAndSource(logging.MyCaller)...)...)
		return false
	}
	r.log.V(1).Info("Revision not synced", append(logging.GetGitRepoInfo(r.repo), logging.GetFunctionAndSource(logging.MyCaller)...)...)
	return true
}

func (r *repoData) SyncRepo() error {
	logging.TraceCall(r.log)
	defer logging.TraceExit(r.log)
	r.syncLock.Lock()
	defer r.syncLock.Unlock()

	if r.repo.Status.Artifact == nil {
		return fmt.Errorf("repository %s does not contain an artifact", r.path)
	}
	if err := isExistingDir(r.dataPath); err == nil {
		r.log.V(1).Info("Revision already synced", append(logging.GetGitRepoInfo(r.repo), logging.GetFunctionAndSource(logging.MyCaller)...)...)
		return nil
	}
	r.log.V(1).Info("New revision detected", append(logging.GetGitRepoInfo(r.repo), logging.GetFunctionAndSource(logging.MyCaller)...)...)

	if err := removeRecreateDir(r.loadPath); err != nil {
		return errors.WithMessagef(err, "%s - failed to remove and recreate load directory", logging.CallerStr(logging.Me))
	}

	ctx, cancel := context.WithTimeout(r.ctx, DefaultTimeOut)
	defer cancel()

	// download and extract artifact
	if err := r.fetchArtifact(ctx); err != nil {
		return errors.Wrapf(err, "%s - failed to fetch repository tar file from source controller", logging.CallerStr(logging.Me))
	}

	if err := os.MkdirAll(r.dataPath, os.ModePerm); err != nil {
		return errors.Wrapf(err, "%s - failed to make directory: %s", logging.CallerStr(logging.Me), r.dataPath)
	}

	r.Lock()
	defer r.Unlock()
	if e := os.RemoveAll(r.dataPath); e != nil {
		return errors.Wrapf(e, "%s - failed to remove data path: %s", logging.CallerStr(logging.Me), r.dataPath)
	}
	if err := os.Rename(r.loadPath, r.dataPath); err != nil {
		return errors.Wrapf(err, "%s - failed to rename load path: %s", logging.CallerStr(logging.Me), r.loadPath)
	}
	r.log.V(1).Info("synced repo", append(logging.GetGitRepoInfo(r.repo), logging.GetFunctionAndSource(logging.MyCaller)...)...)
	return nil
}

func (r *repoData) fetchArtifact(ctx context.Context) error {
	logging.TraceCall(r.log)
	defer logging.TraceExit(r.log)
	repo := r.repo
	if repo.Status.Artifact == nil {
		return fmt.Errorf("repository %s does not contain an artifact", r.path)
	}

	url := repo.Status.Artifact.URL

	if r.hostName != "" {
		url = fmt.Sprintf("http://%s/gitrepository/%s/%s/latest.tar.gz", r.hostName, repo.Namespace, repo.Name)
	}

	r.tarConsumer.SetURL(url)

	tar, err := r.tarConsumer.GetTar(ctx)
	if err != nil {
		return errors.WithMessagef(err, "%s - failed to download artifact from %s", logging.CallerStr(logging.Me), url)
	}
	// Debugging for unzip error
	r.log.V(2).Info("tar data", append(logging.GetGitRepoInfo(r.repo), append(logging.GetFunctionAndSource(logging.MyCaller), "length", len(tar))...)...)

	if err := tarconsumer.UnpackTar(tar, r.GetLoadPath()); err != nil {
		return errors.WithMessagef(err, "%s - failed to untar artifact", logging.CallerStr(logging.Me))
	}

	return nil
}
