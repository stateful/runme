package testdata

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"io"
	"os"
)

//go:embed users_1m.json.gzip
var Users1MGzip []byte

func UngzipToFile(data []byte, file string) (n int64, err error) {
	reader := bytes.NewReader(data)
	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		return
	}
	defer func() {
		cerr := gzipReader.Close()
		if err == nil {
			err = cerr
		}
	}()
	f, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer func() {
		cerr := f.Close()
		if err == nil {
			err = cerr
		}
	}()
	return io.Copy(f, gzipReader)
}
