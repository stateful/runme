package env

import "fmt"

func ConvertMapEnv(mapEnv map[string]string) []string {
	result := make([]string, len(mapEnv))

	i := 0
	for k, v := range mapEnv {
		result[i] = fmt.Sprintf("%s=%s", k, v)
		i++
	}

	return result
}
