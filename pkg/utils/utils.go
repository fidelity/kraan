package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

func IsExistingDir(dataPath string) error {
	info, err := os.Stat(dataPath)
	if os.IsNotExist(err) {
		return err
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("addons Data path: %s, is not a directory", dataPath)
	}

	return nil
}

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

// CompareAsJSON compares two interfaces by converting them to json and comparing json text
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
