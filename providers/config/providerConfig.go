// Package config provides functions for reading and parsing the provider credentials json file.
// It cleans nonstandard json features (comments and trailing commas), as well as replaces environment variable placeholders with
// their environment variable equivalents. To reference an environment variable in your json file, simply use values in this format:
//    "key"="$ENV_VAR_NAME"
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/DisposaBoy/JsonConfigReader"
	"github.com/TomOnTime/utfutil"
)

// LoadProviderConfigs will open or execute the specified file name, and parse its contents. It will replace environment variables it finds if any value matches $[A-Za-z_-0-9]+
func LoadProviderConfigs(fname string) (map[string]map[string]string, error) {
	var results = map[string]map[string]string{}

	var dat []byte
	var err error

	if strings.HasPrefix(fname, "!") {
		dat, err = executeCredsFile(strings.TrimPrefix(fname, "!"))
		if err != nil {
			return nil, err
		}
	} else if isExecutable(fname) {
		dat, err = executeCredsFile(fname)
		if err != nil {
			return nil, err
		}
	} else {
		// no executable bit found nor marked as executable so read it in
		dat, err = readCredsFile(fname)
		if err != nil {
			return nil, err
		}
	}

	s := string(dat)
	r := JsonConfigReader.New(strings.NewReader(s))
	err = json.NewDecoder(r).Decode(&results)
	if err != nil {
		return nil, fmt.Errorf("failed parsing provider credentials file %v: %v", fname, err)
	}
	if err = replaceEnvVars(results); err != nil {
		return nil, err
	}
	return results, nil
}

func isExecutable(filename string) bool {
	if stat, statErr := os.Stat(filename); statErr == nil {
		if mode := stat.Mode(); mode&0111 == 0111 {
			return true
		}
	}
	return false
}

func readCredsFile(filename string) ([]byte, error) {
	dat, err := utfutil.ReadFile(filename, utfutil.POSIX)
	if err != nil {
		// no creds file is ok. Bind requires nothing for example. Individual providers will error if things not found.
		if os.IsNotExist(err) {
			fmt.Printf("INFO: Config file %q does not exist. Skipping.\n", filename)
			return []byte{}, nil
		}
		return nil, fmt.Errorf("failed reading provider credentials file %v: %v", filename, err)
	}
	return dat, nil
}

func executeCredsFile(filename string) ([]byte, error) {
	cmd := filename
	if !strings.HasPrefix(filename, "/") {
		// if the path doesn't start with `/` make sure we aren't relying on $PATH.
		cmd = strings.Join([]string{".", filename}, string(filepath.Separator))
	}
	out, err := exec.Command(cmd).Output()
	return out, err
}

func replaceEnvVars(m map[string]map[string]string) error {
	for _, keys := range m {
		for k, v := range keys {
			if strings.HasPrefix(v, "$") {
				env := v[1:]
				newVal := os.Getenv(env)
				keys[k] = newVal
			}
		}
	}
	return nil
}
