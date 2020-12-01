//Package logging contains logging related functions used by multiple packages
package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"

	sourcev1 "github.com/fluxcd/source-controller/api/v1beta1"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"

	kraanv1alpha1 "github.com/fidelity/kraan/api/v1alpha1"
)

const (
	Me       = 3
	MyCaller = 4
)

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

// GetLayer gets layer, returning object name for AddonsLayer.kraan.io objects or owner name for others
func GetLayer(obj k8sruntime.Object) string {
	gvk := obj.GetObjectKind().GroupVersionKind()
	kind := fmt.Sprintf("%s.%s", gvk.Kind, gvk.Group)
	mobj, ok := (obj).(metav1.Object)
	if !ok {
		return "layer not available"
	}
	if kind == "AddonsLayer.kraan.io" {
		return mobj.GetName()
	}
	owners := mobj.GetOwnerReferences()
	for _, owner := range owners {
		if owner.Kind == "AddonsLayer" && owner.APIVersion == "kraan.io/v1alpha1" {
			return owner.Name
		}
	}
	return "layer not available"
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

// GetLayerInfo gets AddonsLayer details for logging
func GetLayerInfo(src *kraanv1alpha1.AddonsLayer) (result []interface{}) {
	result = append(result, "kind", LayerKind())
	result = append(result, GetObjNamespaceName(src)...)
	result = append(result, "status", src.Status.State)
	return result
}

// GitRepoSourceKind returns the gitrepository kind
func GitRepoSourceKind() string {
	return fmt.Sprintf("%s.%s", sourcev1.GitRepositoryKind, sourcev1.GroupVersion)
}

// LayerKind returns the AddonsLayer kind
func LayerKind() string {
	return fmt.Sprintf("%s.%s", kraanv1alpha1.AddonsLayerKind, kraanv1alpha1.GroupVersion)
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

// CallerStr returns the caller's function, source file and line number as a string
func CallerStr(skip uint) string {
	callerInfo := GetCaller(skip+1, true)
	return fmt.Sprintf("%s - %s(%d)", callerInfo.FunctionName, callerInfo.SourceFile, callerInfo.SourceLine)
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
