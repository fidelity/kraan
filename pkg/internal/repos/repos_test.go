package repos_test

import (
	"context"
	"encoding/json"
	"io/ioutil"
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
		repos.SetTarConsumer(r, test.ctx, httpClient, r.GetGitRepo().GetArtifact().URL)
		err = repos.FetchArtifact(test.ctx, r)
		assert.Equal(t, test.expected, err)
		t.Logf("test: %s, successful", test.name)
	}

	for _, test := range tests {
		doTest(t, test)
	}
}
