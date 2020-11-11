//Package utils contains utility functions used by multiple packages
package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	sourcev1 "github.com/fluxcd/source-controller/api/v1beta1"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
)

const (
	Me       = 3
	MyCaller = 4
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
func GetObjNamespaceName(obj k8sruntime.Object) (result []interface{}) {
	mobj, ok := (obj).(metav1.Object)
	if !ok {
		result = append(result, "namespace", "unavailable", "name", "unavailable")
		return result
	}
	result = append(result, "namespace", mobj.GetNamespace(), "name", mobj.GetName())
	return result
}

// GetObjKindNamespaceName gets object kind namespace and name for logging
func GetObjKindNamespaceName(obj k8sruntime.Object) (result []interface{}) {
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

// CallerInfo hold the function name and source file/line from which a call was made
type CallerInfo struct {
	FunctionName string
	SourceFile   string
	SourceLine   int
}

// Callers returns an array of strings containing the function name, source filename and line
// number for the caller of this function and its caller moving up the stack for as many levels as
// are available or the number of levels specified by the levels parameter.
// Set the short parameter to true to only return final element of Function and source file name.
func Callers(levels uint, short bool) ([]CallerInfo, error) {
	var callers []CallerInfo
	if levels == 0 {
		return callers, nil
	}
	// we get the callers as uintptrs
	fpcs := make([]uintptr, levels)

	// skip 1 levels to get to the caller of whoever called Callers()
	n := runtime.Callers(1, fpcs)
	if n == 0 {
		return nil, fmt.Errorf("caller not availalble")
	}

	frames := runtime.CallersFrames(fpcs)
	for {
		frame, more := frames.Next()
		if frame.Line == 0 {
			break
		}
		funcName := frame.Function
		sourceFile := frame.File
		lineNumber := frame.Line
		if short {
			funcName = filepath.Base(funcName)
			sourceFile = filepath.Base(sourceFile)
		}
		caller := CallerInfo{FunctionName: funcName, SourceFile: sourceFile, SourceLine: lineNumber}
		callers = append(callers, caller)
		if !more {
			break
		}
	}
	return callers, nil
}

// GetCaller returns the caller of GetCaller 'skip' levels back
// Set the short parameter to true to only return final element of Function and source file name
func GetCaller(skip uint, short bool) CallerInfo {
	callers, err := Callers(skip, short)
	if err != nil {
		return CallerInfo{FunctionName: "not available", SourceFile: "not available", SourceLine: 0}
	}
	if skip == 0 {
		return CallerInfo{FunctionName: "not available", SourceFile: "not available", SourceLine: 0}
	}
	if int(skip) > len(callers) {
		return CallerInfo{FunctionName: "not available", SourceFile: "not available", SourceLine: 0}
	}
	return callers[skip-1]
}

// Trace traces calls and exit for functions
func TraceCall(log logr.Logger) {
	callerInfo := GetCaller(MyCaller, true)
	log.V(4).Info("Entering function", "function", callerInfo.FunctionName, "source", callerInfo.SourceFile, "line", callerInfo.SourceLine)
}

// Trace traces calls and exit for functions
func TraceExit(log logr.Logger) {
	callerInfo := GetCaller(MyCaller, true)
	log.V(4).Info("Exiting function", "function", callerInfo.FunctionName, "source", callerInfo.SourceFile, "line", callerInfo.SourceLine)
}

// GetFunctionAndSource gets function name and source line for logging
func GetFunctionAndSource(skip uint) (result []interface{}) {
	callerInfo := GetCaller(skip, true)
	result = append(result, "function", callerInfo.FunctionName, "source", callerInfo.SourceFile, "line", callerInfo.SourceLine)
	return result
}
