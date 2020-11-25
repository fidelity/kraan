// Package for helper functions used in multiple packages
package common

import (
	"os"
)

// Following two functions copied from HelmController
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

// ContainsString tests for a string in a slice of strings
func ContainsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// RemoveString removes an element from a slice of strings if present
func RemoveString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}

// GetRuntimeNamespace returns the runtime namespace or empty string if environmental variable is not set.
func GetRuntimeNamespace() string {
	return os.Getenv("RUNTIME_NAMESPACE")
}

// GetSourceNamespace retruns the namespace provided or the default if it is not set
func GetSourceNamespace(nameSpace string) string {
	if len(nameSpace) > 0 {
		return nameSpace
	}
	return GetRuntimeNamespace()
}
