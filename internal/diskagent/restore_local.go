package diskagent

import (
	"fmt"
	"io"
	"os"

	"github.com/klauspost/compress/zstd"
	"github.com/nikron173/datasaver/internal/archive"
)

type LocalSource struct {
	file          *os.File
	zstdReader    *zstd.Decoder
	archiveReader *archive.ArchiveReader
	// Нужен для чтения текущего файла
	limitedReader io.Reader
}

func NewLocalSource(archivePath string) (*LocalSource, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}

	zr, err := zstd.NewReader(f)
	if err != nil {
		f.Close()
		return nil, err
	}

	ar := archive.NewArchiveReader(zr.IOReadCloser())

	return &LocalSource{file: f, zstdReader: zr, archiveReader: ar}, nil
}

func (ls *LocalSource) NextFile() (*archive.FileHeader, string, error) {
	meta, path, err := ls.archiveReader.ReadFileMetadata()
	if err != nil {
		return nil, "", err
	}
	ls.limitedReader = io.LimitReader(ls.zstdReader, meta.Size)
	return meta, path, err
}

func (ls *LocalSource) ReadChunk(buf []byte) (int, error) {
	if ls.limitedReader == nil {
		return 0, fmt.Errorf("limited reader not initialization")
	}
	return ls.limitedReader.Read(buf)
}

func (ls *LocalSource) Close() error {
	if err := ls.archiveReader.Close(); err != nil {
		return err
	}
	if err := ls.file.Close(); err != nil {
		return err
	}
	return nil
}
