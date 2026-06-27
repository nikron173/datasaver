package diskagent

import (
	"os"

	"github.com/klauspost/compress/zstd"
	"github.com/nikron173/datasaver/internal/archive"
)

type LocalSink struct {
	file          *os.File
	zstdWriter    *zstd.Encoder
	archiveWriter *archive.ArchiveWriter
}

func NewLocalSink(archivePath string) (*LocalSink, error) {
	f, err := os.Create(archivePath)
	if err != nil {
		return nil, err
	}

	zw, err := zstd.NewWriter(f)
	if err != nil {
		f.Close()
		return nil, err
	}

	aw := archive.NewArchiveWriter(zw)

	return &LocalSink{file: f, zstdWriter: zw, archiveWriter: aw}, nil
}

func (ls *LocalSink) WriteFileMetadata(meta archive.FileHeader, path string) error {
	return ls.archiveWriter.WriteFileMetadata(path, meta.Size, meta.Mode)
}

func (ls *LocalSink) WriteChunk(data []byte) error {
	return ls.archiveWriter.WriteDataBlock(data)
}

func (ls *LocalSink) Close() error {
	if err := ls.archiveWriter.Close(); err != nil {
		return err
	}
	if err := ls.file.Close(); err != nil {
		return err
	}
	return nil
}
