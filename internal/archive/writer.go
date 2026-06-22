package archive

import (
	"encoding/binary"
	"io"
)

// ArchiveWriter отвечает за правильную сериализацию файлов в бинарный поток СРК
type ArchiveWriter struct {
	writer io.WriteCloser
}

// NewArchiveWriter создает новый экземпляр упаковщика
func NewArchiveWriter(w io.WriteCloser) *ArchiveWriter {
	return &ArchiveWriter{writer: w}
}

// WriteFile упаковывает метаданные файла в поток (BigEndian)
func (aw *ArchiveWriter) WriteFileMetadata(path string, size int64, mode uint32) error {
	pathSize := int32(len(path))

	if err := binary.Write(aw.writer, binary.BigEndian, pathSize); err != nil {
		return err
	}
	if err := binary.Write(aw.writer, binary.BigEndian, size); err != nil {
		return err
	}
	if err := binary.Write(aw.writer, binary.BigEndian, mode); err != nil {
		return err
	}
	if _, err := aw.writer.Write([]byte(path)); err != nil {
		return err
	}
	return nil
}

// WriteDataBlock записывает сырой кусок данных (чанк) файла в поток
func (aw *ArchiveWriter) WriteDataBlock(data []byte) error {
	_, err := aw.writer.Write(data)
	return err
}

func (aw *ArchiveWriter) Close() error {
	return aw.writer.Close()
}
