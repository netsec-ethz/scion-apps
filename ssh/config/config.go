// Copyright 2020 ETH Zurich
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"reflect"
	"regexp"
	"strings"
)

// Config is an interface representing a configuration file
type Config interface {
}

// UpdateFromString updates the given config from the single-line configuration string.
func UpdateFromString(conf Config, confOption string) error {
	split := regexp.MustCompile(`(.*?)\s*[\s=]\s*(.*)`).FindStringSubmatch(confOption)
	if len(split) < 3 {
		return fmt.Errorf("can't parse config file line: %s", confOption)
	}

	return Set(conf, split[1], split[2])
}

func cToString(valueA interface{}) string {
	bval, ok := valueA.(bool)
	if ok {
		if bval {
			return "yes"
		}
		return "no"
	}
	return fmt.Sprintf("%v", valueA)
}

// Set sets the given option on the given configuration file.
func Set(conf Config, name string, valueA interface{}) error {
	value := cToString(valueA)

	typeToSet, _ := reflect.TypeOf(conf).Elem().FieldByName(name)
	fieldToSet := reflect.ValueOf(conf).Elem().FieldByName(name)
	if !fieldToSet.IsValid() || !fieldToSet.CanSet() {
		return fmt.Errorf("unknown config option: %s %+v", name, fieldToSet)
	}

	checkRegexStr := fmt.Sprintf("^%s$", typeToSet.Tag.Get("regex"))
	// log.Printf("%s %v", checkRegexStr, typeToSet.Tag)
	checkRegex := regexp.MustCompile(checkRegexStr)
	if !checkRegex.MatchString(value) {
		return fmt.Errorf("value for option %s doesn't fit regex %s: %s", name, checkRegexStr, value)
	}

	val, add, err := parseConfigValue(strings.TrimSpace(value), fieldToSet.Type())
	if err != nil {
		return fmt.Errorf("can't parse config value for option %s: %s", name, value)
	}

	if add {
		fieldToSet.Set(reflect.Append(fieldToSet, val))
	} else {
		fieldToSet.Set(val)
	}

	return nil
}

// SetIfNot sets the given option on the given configuration file if and only if it is not equal to the last parameter.
func SetIfNot(conf Config, name string, value, not interface{}) (bool, error) {
	if cToString(value) == cToString(not) {
		return true, nil
	}
	return false, Set(conf, name, value)
}

// UpdateFromFile automatically reads a file and updates the configuration object from its contents.
func UpdateFromFile(conf Config, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	return UpdateFromReader(conf, file)
}

// UpdateFromReader takes a reader and updates the configuration object from its contents.
func UpdateFromReader(conf Config, reader io.Reader) error {
	lines := make([]string, 0)

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(text, "#") || len(text) == 0 {
			continue
		}
		lines = append(lines, text)
	}

	for i := len(lines) - 1; i >= 0; i-- {
		err := UpdateFromString(conf, lines[i])
		if err != nil {
			log.Printf("Error while updating config: %v", err)
		}
	}

	err := scanner.Err()
	if err != nil {
		return err
	}

	return nil
}

func parseConfigValue(confval string, tpye reflect.Type) (reflect.Value, bool, error) {
	switch tpye.Kind() {
	case reflect.Slice:
		val, _, err := parseConfigValue(confval, tpye.Elem())
		return val, true, err
	case reflect.String:
		return reflect.ValueOf(confval), false, nil
	default:
		return reflect.Zero(tpye), false, fmt.Errorf("Config field type not supported! %v", tpye)
	}
}
