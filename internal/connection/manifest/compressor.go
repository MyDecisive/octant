package manifest

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"context"
	"errors"
	"fmt"
	"hash/crc32"
	"time"
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

// NewZipCompressor returns a new instance of ZipCompressor.
func NewZipCompressor() *ZipCompressor {
	return &ZipCompressor{}
}

// Compress compresses the given manifests into zip.
func (*ZipCompressor) Compress(ctx context.Context, manifests map[string][]byte) (*bytes.Buffer, error) {
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	for filename, content := range manifests {
		select {
		case <-ctx.Done():
			return nil, errors.New("context cancelled")
		default:
		}

		// Manually compress the content in memory first
		var compressedBuf bytes.Buffer
		flateWriter, createErr := flate.NewWriter(&compressedBuf, flate.DefaultCompression)
		if createErr != nil {
			return nil, fmt.Errorf("create flate writer for %s:%w", filename, createErr)
		}
		if _, writeErr := flateWriter.Write(content); writeErr != nil {
			return nil, fmt.Errorf("flate write for %s:%w", filename, writeErr)
		}
		if closeErr := flateWriter.Close(); closeErr != nil {
			return nil, fmt.Errorf("flate close for %s:%w", filename, closeErr)
		}

		compressedContent := compressedBuf.Bytes()

		const utf8Flag = 0x800 // utf-8 filenames flag
		// Now we know the exact compressed size. Build the header.
		const versionTwo = uint16(20) // zip spec v2.0, widely compatible
		header := &zip.FileHeader{
			Name:               filename,
			Method:             zip.Deflate,
			UncompressedSize64: uint64(len(content)),
			CompressedSize64:   uint64(len(compressedContent)),
			CRC32:              crc32.ChecksumIEEE(content),
			CreatorVersion:     versionTwo,
			ReaderVersion:      versionTwo,
			Flags:              utf8Flag,
		}

		// SetModTime converts time.Now() into the legacy MS-DOS uint16 fields
		// that CreateRaw actually writes to the byte stream.
		// Uses deprecated `SetModTime` due to https://github.com/golang/go/issues/76741
		header.SetModTime(time.Now()) // nolint:staticcheck

		// SetMode establishes standard read/write file permissions (ExternalAttrs)
		// which Windows Explorer relies on to know it's a standard file.
		const rwPermissions = 0o644
		header.SetMode(rwPermissions)

		// Use CreateRaw to inject the pre-compressed bytes directly
		fWriter, err := zipWriter.CreateRaw(header)
		if err != nil {
			return nil, fmt.Errorf("generate zip header for %s:%w", filename, err)
		}

		if _, err := fWriter.Write(compressedContent); err != nil {
			return nil, fmt.Errorf("write compressed zip data for %s:%w", filename, err)
		}
	}

	if closeErr := zipWriter.Close(); closeErr != nil {
		return nil, fmt.Errorf("close zip writer: %w", closeErr)
	}

	return buf, nil
}
