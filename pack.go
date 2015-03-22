package library

import (
	"bytes"
	"compress/gzip"
	"io"
)

//size of buffer when zipping/unzipping data
const PackBufferSize = 4096

func pack(in io.Reader) (io.Reader, error) {
	ign := make([]byte, PackBufferSize)
	buf := new(bytes.Buffer)
	gzw := gzip.NewWriter(buf)
	reader := io.TeeReader(in, gzw)
	for {
		if _, err := reader.Read(ign); err != nil {
			if err != io.EOF {
				return nil, err
			}
			break
		}
	}
	//close gzip writer
	if err := gzw.Close(); err != nil {
		return nil, err
	}
	return buf, nil
}

func unpack(in io.Reader) (io.Reader, error) {
	ign := make([]byte, PackBufferSize)
	buf := new(bytes.Buffer)
	gz, err := gzip.NewReader(in)
	if err != nil {
		return nil, err
	}
	reader := io.TeeReader(gz, buf)
	for {
		if _, err := reader.Read(ign); err != nil {
			if err != io.EOF {
				return nil, err
			}
			break
		}
	}
	//close gzip reader
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf, nil
}
