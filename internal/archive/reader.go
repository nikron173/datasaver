package archive

import (
	"encoding/binary"
	"io"
	"log/slog"
)

type ArchiveReader struct {
	reader io.ReadCloser
}

func NewArchiveReader(r io.ReadCloser) *ArchiveReader {
	return &ArchiveReader{reader: r}
}

// ReadFile читает метаданные файла в поток (BigEndian)
func (ar *ArchiveReader) ReadFileMetadata() (*FileHeader, string, error) {
	header := FileHeader{}

	if err := binary.Read(ar.reader, binary.BigEndian, &header.PathSize); err != nil {
		return nil, "", err
	}
	if err := binary.Read(ar.reader, binary.BigEndian, &header.Size); err != nil {
		return nil, "", err
	}
	if err := binary.Read(ar.reader, binary.BigEndian, &header.Mode); err != nil {
		return nil, "", err
	}

	originalPathBuf := make([]byte, header.PathSize)
	if _, err := io.ReadFull(ar.reader, originalPathBuf); err != nil {
		slog.Error("error read archive", slog.String("err", err.Error()))
		return nil, "", err
	}

	return &header, string(originalPathBuf), nil
}

// ReadDataBlock читает сырой кусок данных (чанк) файла в поток
func (ar *ArchiveReader) ReadDataBlock(data []byte) (int, error) {
	return ar.reader.Read(data)
}

func (ar *ArchiveReader) Close() error {
	return ar.reader.Close()
}
