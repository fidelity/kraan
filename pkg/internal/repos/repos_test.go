package repos_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	sourcev1 "github.com/fluxcd/source-controller/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/fidelity/kraan/pkg/internal/repos"
	"github.com/fidelity/kraan/pkg/internal/testutils"
)

const (
	testSrcRepo = "testdata/source-repo.yaml"
	testdataDir = "testdata/addons"
)

func getSrcRepo(filePath string) (*sourcev1.GitRepository, error) {
	buffer, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	srcRepo := &sourcev1.GitRepository{}
	err = json.Unmarshal(buffer, srcRepo)
	if err != nil {
		return nil, err
	}
	return srcRepo, nil
}

func TestFetchArtifact(t *testing.T) {
	type testsData struct {
		name        string
		ctx         context.Context
		srcRepoName string
		expected    error
	}

	tests := []testsData{{
		name:        "fetch tar and unpack",
		ctx:         context.Background(),
		srcRepoName: testSrcRepo,
		expected:    nil,
	},
	}

	doTest := func(t *testing.T, test testsData) {
		srcRepo, err := getSrcRepo(test.srcRepoName)
		if err != nil {
			t.Fatalf(err.Error())
			return
		}
		r := repos.NewRepo(context.Background(), log.Log, srcRepo)
		repos.RootPath, err = ioutil.TempDir("", "test-*")
		if err != nil {
			t.Fatalf(err.Error())
			return
		}
		httpClient, host, teardown := testutils.StartHTTPServer(t, testdataDir)
		defer teardown()
		repos.HostName = host
		repos.SetTarConsumer(r, test.ctx, httpClient, repos.GetGitRepo(r).GetArtifact().URL)
		err = repos.FetchArtifact(test.ctx, r)
		assert.Equal(t, test.expected, err)
		t.Logf("test: %s, successful", test.name)
	}

	for _, test := range tests {
		doTest(t, test)
	}
}

func TestSyncRepo(t *testing.T) {
	type testsData struct {
		name        string
		srcRepoName string
		expected    error
	}

	tests := []testsData{{
		name:        "test syncRepo directory doesn't exist",
		srcRepoName: testSrcRepo,
		expected:    nil,
	},
	}

	doTest := func(t *testing.T, test testsData) {
		srcRepo, err := getSrcRepo(test.srcRepoName)
		if err != nil {
			t.Fatalf(err.Error())
			return
		}
		r := repos.NewRepo(context.Background(), log.Log, srcRepo)
		repos.RootPath, err = ioutil.TempDir("", "test-*")
		if err != nil {
			t.Fatalf(err.Error())
			return
		}
		httpClient, host, teardown := testutils.StartHTTPServer(t, testdataDir)
		defer teardown()
		repos.HostName = host
		repos.SetTarConsumer(r, context.Background(), httpClient, repos.GetGitRepo(r).GetArtifact().URL)
		err = r.SyncRepo()
		assert.Equal(t, test.expected, err)
		t.Logf("test: %s, successful", test.name)
	}

	for _, test := range tests {
		doTest(t, test)
	}
}

func TestLinkData(t *testing.T) { //nolint:funlen,gocyclo // ok
	type testsData struct {
		name         string
		srcRepoName  string
		layerPath    string
		sourcePath   string
		createTarget bool
		createSource bool
		expected     string
	}
	var err error
	repos.RootPath, err = ioutil.TempDir("", "test-*")
	if err != nil {
		t.Fatalf(err.Error())
		return
	}

	tests := []testsData{{
		name:         "link existing to existing target",
		srcRepoName:  testSrcRepo,
		layerPath:    fmt.Sprintf("%s/layers/name/version", repos.RootPath),
		sourcePath:   "addons/name",
		createSource: true,
		createTarget: true,
		expected:     "",
	}, {
		name:         "link nonexisting to existing target",
		srcRepoName:  testSrcRepo,
		layerPath:    fmt.Sprintf("%s/layers/name/version", repos.RootPath),
		sourcePath:   "addons/name",
		createSource: true,
		createTarget: false,
		expected:     "",
	}, {
		name:         "link to non existing target",
		srcRepoName:  testSrcRepo,
		layerPath:    fmt.Sprintf("%s/layers/name/version", repos.RootPath),
		sourcePath:   "addons/name",
		createSource: false,
		createTarget: false,
		expected:     "failed, target directory does not exit",
	},
	}

	doTest := func(t *testing.T, test testsData) {
		srcRepo, err := getSrcRepo(test.srcRepoName)
		if err != nil {
			t.Fatalf(err.Error())
			return
		}
		r := repos.NewRepo(context.Background(), log.Log, srcRepo)
		if test.createSource {
			if e := os.MkdirAll(fmt.Sprintf("%s/%s", repos.GetDataPath(repos.GetGitRepo(r)), test.sourcePath), os.ModePerm); e != nil {
				t.Fatalf(err.Error())
				return
			}
		}
		if test.createTarget {
			if e := os.MkdirAll(test.layerPath, os.ModePerm); e != nil {
				t.Fatalf(err.Error())
				return
			}
		}
		err = r.LinkData(test.layerPath, test.sourcePath)
		if test.expected == "" {
			if !assert.Nil(t, err) {
				return
			}
		} else {
			if !strings.Contains(err.Error(), test.expected) {
				t.Fatalf("expected error containing: %s\nGot: %s", test.expected, err.Error())
				return
			}
		}

		if e := os.RemoveAll(repos.RootPath); e != nil {
			t.Fatalf(e.Error())
			return
		}
		t.Logf("test: %s, successful", test.name)
	}

	for _, test := range tests {
		doTest(t, test)
	}
}
