package manifest

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
)

// Compressor provide functionality to generate a compressed manifest.
type Compressor interface {
	// Compress compresses the given manifests.
	Compress(ctx context.Context, manifests map[string][]byte) (*bytes.Buffer, error)
}

// ZipCompressor implements Compressor by compressing manifests into zip.
type ZipCompressor struct{}

// Ensure ZipCompressor implements Compressor.
var _ Compressor = (*ZipCompressor)(nil)

// Compress compresses the given manifests into zip.
func (*ZipCompressor) Compress(ctx context.Context, manifests map[string][]byte) (*bytes.Buffer, error) {
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)
	defer zipWriter.Close() //nolint:errcheck
	for filename, content := range manifests {
		select {
		case <-ctx.Done():
			return nil, errors.New("context cancelled")
		default:
		}

		fWriter, err := zipWriter.Create(filename)
		if err != nil {
			return nil, fmt.Errorf("generate zip for %s:%w", filename, err)
		}

		if _, err := fWriter.Write(content); err != nil {
			return nil, fmt.Errorf("write zip for %s:%w", filename, err)
		}
	}
	return buf, nil
}
