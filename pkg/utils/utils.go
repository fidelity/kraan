//Package utils contains utility functions used by multiple packages
package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	sourcev1 "github.com/fluxcd/source-controller/api/v1beta1"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func CompareAsJSON(one, two interface{}) bool {
	if one == nil && two == nil {
		return true
	}
	jsonOne, err := ToJSON(one)
	if err != nil {
		return false
	}

	jsonTwo, err := ToJSON(two)
	if err != nil {
		return false
	}
	return jsonOne == jsonTwo
}

// LogJSON is used log an item in JSON format.
func LogJSON(data interface{}) string {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err.Error()
	}
	var prettyJSON bytes.Buffer
	err = json.Indent(&prettyJSON, jsonData, "", "  ")
	if err != nil {
		return err.Error()
	}
	return prettyJSON.String()
}

// ToJSON is used to convert a data structure into JSON format.
func ToJSON(data interface{}) (string, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", errors.WithMessage(err, "failed to marshal json")
	}
	var prettyJSON bytes.Buffer
	err = json.Indent(&prettyJSON, jsonData, "", "\t")
	if err != nil {
		return "", errors.WithMessage(err, "failed indent json")
	}
	return prettyJSON.String(), nil
}

// GetObjNamespaceName gets object namespace and name for logging
func GetObjNamespaceName(obj runtime.Object) (result []interface{}) {
	mobj, ok := (obj).(metav1.Object)
	if !ok {
		result = append(result, "namespace", "unavailable", "name", "unavailable")
		return result
	}
	result = append(result, "namespace", mobj.GetNamespace(), "name", mobj.GetName())
	return result
}

// GetObjKindNamespaceName gets object kind namespace and name for logging
func GetObjKindNamespaceName(obj runtime.Object) (result []interface{}) {
	gvk := obj.GetObjectKind().GroupVersionKind()
	result = append(result, "kind", fmt.Sprintf("%s.%s", gvk.Kind, gvk.Group))
	result = append(result, GetObjNamespaceName(obj)...)
	return result
}

// GetGitRepoInfo gets GitRepository details for logging
func GetGitRepoInfo(srcRepo *sourcev1.GitRepository) (result []interface{}) {
	result = append(result, "kind", GitRepoSourceKind())
	result = append(result, GetObjNamespaceName(srcRepo)...)
	result = append(result, "generation", srcRepo.Generation, "observed", srcRepo.Status.ObservedGeneration)
	if srcRepo.Status.Artifact != nil {
		result = append(result, "revision", srcRepo.Status.Artifact.Revision)
	}
	return result
}

// GitRepoSourceKind returns the gitrepository kind
func GitRepoSourceKind() string {
	return fmt.Sprintf("%s.%s", sourcev1.GitRepositoryKind, sourcev1.GroupVersion)
}

// RemoveIfExists removes a directory if it exists
func RemoveIfExists(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	if e := os.RemoveAll(path); e != nil {
		return errors.Wrap(e, "failed to remove directory")
	}
	return nil
}

// RemoveRecreateDir removes a directory if it exists and then recreates it
func RemoveRecreateDir(path string) error {
	if e := RemoveIfExists(path); e != nil {
		return errors.WithMessage(e, "failed to remove directory before recreate")
	}
	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		return errors.Wrapf(err, "failed to make directory: %s", path)
	}
	return nil
}
