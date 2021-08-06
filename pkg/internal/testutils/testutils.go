package testutils

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// ToJSON is used to convert a data structure into JSON format.
func ToJSON(data interface{}) (string, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	var prettyJSON bytes.Buffer
	err = json.Indent(&prettyJSON, jsonData, "", "\t")
	if err != nil {
		return "", err
	}
	return prettyJSON.String(), nil
}

func Compress(t *testing.T, srcDir string) []byte {
	var buf bytes.Buffer
	zr := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zr)

	walkFunc := func(file string, fi os.FileInfo, err error) error { // nolint:staticcheck // ok
		// generate tar header
		var header *tar.Header
		header, err = tar.FileInfoHeader(fi, file) // nolint:staticcheck // ok
		if err != nil {
			return err
		}

		// must provide real name
		// (see https://golang.org/src/archive/tar/common.go?#L626)
		header.Name = filepath.ToSlash(file)

		// write header
		if err = tw.WriteHeader(header); err != nil {
			return err
		}
		// if not a dir, write file content
		if !fi.IsDir() {
			var data *os.File
			data, err = os.Open(file)
			if err != nil {
				return err
			}
			if _, err = io.Copy(tw, data); err != nil {
				return err
			}
		}
		return err
	}
	// walk through every file in the folder
	e := filepath.Walk(srcDir, walkFunc)
	if e != nil {
		t.Fatalf(e.Error())
		return nil
	}
	// produce tar
	if err := tw.Close(); err != nil {
		t.Fatalf(err.Error())
		return nil
	}
	// produce gzip
	if err := zr.Close(); err != nil {
		t.Fatalf(err.Error())
		return nil
	}
	//
	return buf.Bytes()
}

func StartHTTPServer(t *testing.T, testDir string) (*http.Client, string, func()) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n, err := w.Write(Compress(t, testDir))
		if err != nil {
			t.Fatalf("failed to write tar data, %s", err.Error())
		}
		if n <= 0 {
			t.Fatalf("empty tar data")
		}
	})
	s := httptest.NewServer(h)

	cli := &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, network, _ string) (net.Conn, error) {
				return net.Dial(network, s.Listener.Addr().String())
			},
		},
	}

	return cli, s.Listener.Addr().String(), s.Close
}
