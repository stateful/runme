package env

import (
	"fmt"
	"strings"
)

var SessionKeys = []string{"RUNME_SESSION", "RUNME_SERVER_ADDR", "RUNME_TLS_DIR"}

func ConvertMapEnv(mapEnv map[string]string) []string {
	result := make([]string, len(mapEnv))

	i := 0
	for k, v := range mapEnv {
		result[i] = fmt.Sprintf("%s=%s", k, v)
		i++
	}

	return result
}

func CleaSessionVars(envs []string) []string {
	var cleanedEnv []string

	for _, env := range envs {
		shouldKeep := true
		for _, key := range SessionKeys {
			if strings.Contains(env, key) {
				shouldKeep = false
				break
			}
		}
		if shouldKeep {
			cleanedEnv = append(cleanedEnv, env)
		}
	}

	return cleanedEnv
}
