package tarconsumer

import (
	"context"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/fidelity/kraan/pkg/internal/testutils"
)

const (
	someFiles = "testdata/addons"
	empty     = "testdata/empty"
)

func TestTarConsumer(t *testing.T) {
	type testsData struct {
		name       string
		tarDataDir string
		expected   []byte
	}

	tests := []testsData{{
		name:       "return tar file",
		tarDataDir: someFiles,
		expected:   testutils.Compress(t, someFiles),
	}, {
		name:       "return empty tar file",
		tarDataDir: empty,
		expected:   testutils.Compress(t, empty),
	},
	}

	doTest := func(t *testing.T, test testsData) {
		httpClient, host, teardown := testutils.StartHTTPServer(t, test.tarDataDir)
		defer teardown()

		tarConsumer := NewTarConsumer(context.Background(), httpClient,
			fmt.Sprintf("http://%s/%s", host, test.tarDataDir))

		data, err := tarConsumer.GetTar(context.Background())

		assert.Nil(t, err)
		assert.Equal(t, data, test.expected)
		t.Logf("test: %s, successful", test.name)
	}

	for _, test := range tests {
		doTest(t, test)
	}
}

func TestTarUpack(t *testing.T) {
	type testsData struct {
		name       string
		tarDataDir string
		expected   error
	}

	tests := []testsData{{
		name:       "unpack tar file",
		tarDataDir: someFiles,
		expected:   nil,
	}, {
		name:       "unpack empty tar file",
		tarDataDir: empty,
		expected:   nil,
	},
	}

	doTest := func(t *testing.T, test testsData) {
		httpClient, host, teardown := testutils.StartHTTPServer(t, test.tarDataDir)
		defer teardown()

		tarConsumer := NewTarConsumer(context.Background(), httpClient,
			fmt.Sprintf("http://%s/%s", host, test.tarDataDir))

		data, err := tarConsumer.GetTar(context.Background())
		if err != nil {
			t.Fatalf(err.Error())
			return
		}

		dir, err := ioutil.TempDir("", "test-*")
		if err != nil {
			t.Fatalf(err.Error())
			return
		}
		err = UnpackTar(data, dir)
		assert.Equal(t, err, test.expected)
		t.Logf("test: %s, successful", test.name)
	}

	for _, test := range tests {
		doTest(t, test)
	}
}
