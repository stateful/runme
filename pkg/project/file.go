package project

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
)

func GetRelativePath(cwd, path string) string {
	relPath, err := filepath.Rel(cwd, path)
	if err != nil {
		relPath = path
	}
	return relPath
}

func readMarkdown(source string) ([]byte, error) {
	var (
		data []byte
		err  error
	)

	if source == "-" {
		data, err = io.ReadAll(os.Stdin)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read from stdin")
		}
	} else if strings.HasPrefix(source, "https://") {
		client := http.Client{
			Timeout: time.Second * 5,
		}
		resp, err := client.Get(source)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get a file %q", source)
		}
		defer func() { _ = resp.Body.Close() }()
		data, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read body")
		}
	} else {
		data, err = os.ReadFile(source)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read from file %q", source)
		}
	}

	return data, nil
}

func writeMarkdown(destination string, data []byte) error {
	if destination == "-" {
		_, err := os.Stdout.Write(data)
		return errors.Wrap(err, "failed to write to stdout")
	}
	if strings.HasPrefix(destination, "https://") {
		return errors.New("cannot write to HTTPS location")
	}
	err := os.WriteFile(destination, data, 0o600)
	return errors.Wrapf(err, "failed to write data to %q", destination)
}
