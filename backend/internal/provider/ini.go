package provider

import (
	"bufio"
	"os"
	"strings"
)

// iniFile mirrors the subset of PHP's parse_ini_file output the Unraid state
// files rely on: [section] blocks with key=value pairs. Unraid writes single
// quoted values, so quotes are stripped.
type iniFile map[string]map[string]string

func parseINI(path string) (iniFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	out := iniFile{}
	section := ""
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.Trim(strings.TrimSpace(line[1:len(line)-1]), `"'`)
			if _, ok := out[section]; !ok {
				out[section] = map[string]string{}
			}
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if _, exists := out[section]; !exists {
			out[section] = map[string]string{}
		}
		out[section][key] = value
	}
	if err := scanner.Err(); err != nil {
		return out, err
	}
	return out, nil
}

func (i iniFile) section(name string) map[string]string {
	if i == nil {
		return nil
	}
	return i[name]
}

func (i iniFile) get(section, key string) string {
	if s, ok := i[section]; ok {
		return s[key]
	}
	return ""
}
