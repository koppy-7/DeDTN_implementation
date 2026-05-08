package pbft

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// LoadDotEnv loads key=value pairs from a .env file and sets them as process environment variables.
// Lines beginning with # and blank lines are ignored.
func LoadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		idx := strings.Index(line, "=")
		if idx <= 0 {
			continue
		}

		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])
		if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
			if unquoted, err := strconv.Unquote(value); err == nil {
				value = unquoted
			}
		}
		os.Setenv(key, value)
	}

	return scanner.Err()
}

// EnvString returns the environment variable value if set, otherwise the provided default.
func EnvString(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}

// EnvUint64 returns a parsed uint64 from the environment or the provided default.
func EnvUint64(key string, def uint64) uint64 {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if parsed, err := strconv.ParseUint(v, 10, 64); err == nil {
			return parsed
		}
	}
	return def
}
