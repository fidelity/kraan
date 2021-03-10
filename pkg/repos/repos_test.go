package repos_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/fidelity/kraan/pkg/internal/testutils"
	"github.com/fidelity/kraan/pkg/repos"

	sourcev1 "github.com/fluxcd/source-controller/api/v1beta1"
	testlogr "github.com/go-logr/logr/testing"
)

const (
	testSrcRepo = "testdata/source-repo.yaml"
	testdataDir = "testdata/addons"
)

type repoTest struct {
	name      string
	repoYAML  string
	repoStore repos.Repos
	srcRepo   *sourcev1.GitRepository
	expected  interface{}
}

func newRepoTest(t *testing.T, name, repoYAML string, repoStore repos.Repos, expected interface{}) repoTest {
	test := repoTest{
		name:      name,
		repoYAML:  repoYAML,
		repoStore: repoStore,
		srcRepo:   getTestSourceRepo(t, repoYAML),
		expected:  expected,
	}
	return test
}

func getTestSourceRepo(t *testing.T, repoYAML string) *sourcev1.GitRepository {
	buffer, err := ioutil.ReadFile(repoYAML)
	if err != nil {
		t.Fatalf("error reading file from path '%s': %#v", repoYAML, err)
	}
	srcRepo := &sourcev1.GitRepository{}
	err = json.Unmarshal(buffer, srcRepo)
	if err != nil {
		t.Fatalf("error unmarshalling GitRepository YAML at '%s': %#v", repoYAML, err)
	}
	return srcRepo
}

func makeTempRootDir(t *testing.T) string {
	rootPath, err := ioutil.TempDir("", "test-*")
	if err != nil {
		t.Fatalf(err.Error())
	}
	return rootPath
}

func (r *repoTest) createTempRootDir(t *testing.T) {
	rootPath, err := ioutil.TempDir("", "test-*")
	if err != nil {
		t.Fatalf(err.Error())
	}
	r.repoStore.SetRootPath(rootPath)
}

func (r *repoTest) getRepo() repos.Repo {
	return r.repoStore.Add(r.srcRepo)
}

type fetchArtifactTest struct {
	repoTest
	ctx context.Context
}

func TestFetchArtifact(t *testing.T) {
	testRepos := repos.NewRepos(context.Background(), testlogr.TestLogger{T: t})

	newTest := func(ctx context.Context, t *testing.T, name, repoYAML string, testRepos repos.Repos, expected interface{}) fetchArtifactTest {
		return fetchArtifactTest{
			repoTest: newRepoTest(t, name, repoYAML, testRepos, expected),
			ctx:      ctx,
		}
	}

	tests := []fetchArtifactTest{
		newTest(context.Background(), t, "fetch tar and unpack", testSrcRepo, testRepos, nil),
	}

	doTest := func(t *testing.T, test fetchArtifactTest) {
		test.createTempRootDir(t)

		httpClient, host, teardown := testutils.StartHTTPServer(t, testdataDir)
		defer teardown()

		repoStore := test.repoStore

		repoStore.SetHTTPClient(httpClient)
		repoStore.SetHostName(host)

		r := test.getRepo()

		err := repos.FetchRepoArtifact(r, test.ctx)
		if !reflect.DeepEqual(err, test.expected) {
			t.Fatalf("error returned from repos.FetchArtifact did not match expected error:\nWanted: %#v\nGot: %#v", test.expected, err)
		}
		t.Logf("test: %s, successful", test.name)
	}

	for _, test := range tests {
		doTest(t, test)
	}
}

type syncRepoTest struct {
	repoTest
}

func TestSyncRepo(t *testing.T) {
	testRepos := repos.NewRepos(context.Background(), testlogr.TestLogger{T: t})

	newTest := func(name, path string, expected interface{}) syncRepoTest {
		return syncRepoTest{
			repoTest: newRepoTest(t, name, path, testRepos, expected),
		}
	}

	tests := []syncRepoTest{
		newTest("test syncRepo directory doesn't exist", testSrcRepo, nil),
	}

	doTest := func(t *testing.T, test syncRepoTest) {
		test.createTempRootDir(t)

		httpClient, host, teardown := testutils.StartHTTPServer(t, testdataDir)
		defer teardown()

		test.repoStore.SetHTTPClient(httpClient)
		test.repoStore.SetHostName(host)

		r := test.getRepo()

		err := r.SyncRepo()
		if !reflect.DeepEqual(err, test.expected) {
			t.Fatalf("error returned from %T.SyncRepo did not match expected error:\nWanted: %#v\nGot: %#v", r, test.expected, err)
		}
		t.Logf("test: %s, successful", test.name)
	}

	for _, test := range tests {
		doTest(t, test)
	}
}

type linkDataTest struct {
	repoTest
	layerPath    string
	sourcePath   string
	createTarget bool
	createSource bool
}

func (l linkDataTest) getExpected(t *testing.T) string {
	result, ok := l.repoTest.expected.(string)
	if !ok {
		t.Errorf("expected result is not a string: %#v", l.repoTest.expected)
	}
	return result
}

func (l linkDataTest) checkExpected(t *testing.T, repo repos.Repo, err error) {
	expected := l.getExpected(t)
	if expected == "" { //nolint:nestif // ok
		if err != nil {
			t.Fatalf("%T.LinkData returned an error: %#v", repo, err)
		}
	} else {
		if err != nil {
			errStr := err.Error()
			if !strings.Contains(errStr, expected) {
				t.Fatalf("expected error containing: '%s'\nGot: '%s'", expected, err.Error())
			}
		} else {
			t.Fatalf("expected error was not returned from %T.LinkData function: containing '%s'", repo, expected)
		}
	}
}

func TestLinkData(t *testing.T) {
	testRepos := repos.NewRepos(context.Background(), testlogr.TestLogger{T: t})

	newTest := func(name, path, layerPath, sourcePath string, createTarget, createSource bool, expected interface{}) linkDataTest {
		return linkDataTest{
			repoTest:     newRepoTest(t, name, path, testRepos, expected),
			layerPath:    layerPath,
			sourcePath:   sourcePath,
			createTarget: createTarget,
			createSource: createSource,
		}
	}

	tests := []linkDataTest{
		newTest("link existing to existing target", testSrcRepo, "layers/name/version", "addons/name", true, true, ""),
		newTest("link nonexisting to existing target", testSrcRepo, "layers/name/version", "addons/name", false, true, ""),
		newTest("link to non existing target", testSrcRepo, "layers/name/version", "addons/name", false, false, "failed, target directory does not exist"),
	}

	doTest := func(t *testing.T, test linkDataTest) {
		rootPath := makeTempRootDir(t)
		testRepos.SetRootPath(rootPath)

		httpClient, host, teardown := testutils.StartHTTPServer(t, testdataDir)
		defer teardown()

		repoStore := test.repoTest.repoStore

		repoStore.SetHTTPClient(httpClient)
		repoStore.SetHostName(host)

		r := test.getRepo()

		sourceDirPath := fmt.Sprintf("%s/%s", r.GetDataPath(), test.sourcePath)
		if test.createSource {
			t.Logf("creating source dir '%s'", sourceDirPath)
			if err := os.MkdirAll(sourceDirPath, os.ModePerm); err != nil {
				t.Fatalf("error creating test source path directory '%s': %#v", test.sourcePath, err)
				return
			}
		}
		layerDirPath := fmt.Sprintf("%s/%s", rootPath, test.layerPath)
		if test.createTarget {
			t.Logf("creating layer dir '%s'", layerDirPath)
			if err := os.MkdirAll(layerDirPath, os.ModePerm); err != nil {
				t.Fatalf("error creating target layer path directory '%s': %#v", layerDirPath, err)
			}
		}
		err := r.LinkData(layerDirPath, test.sourcePath)
		test.checkExpected(t, r, err)

		if e := os.RemoveAll(rootPath); e != nil {
			t.Fatalf("error removing test '%s' temp directory '%s': %#v", test.name, rootPath, e.Error())
		}
		repoStore.Delete(r.GetPath())
		t.Logf("test: %s, successful", test.name)
	}

	for _, test := range tests {
		doTest(t, test)
	}
}
