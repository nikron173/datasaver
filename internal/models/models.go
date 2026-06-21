package models

// Структура заголовка файла для нашего бинарного формата
type FileHeader struct {
	PathSize int32  // 4 байта под длину пути файла
	Size     int64  // 8 байт под размер самого файла
	Mode     uint32 // 4 байта под права доступа Linux (permissions)
}

func NewFileHeader(pathSize int32, size int64, mode uint32) FileHeader {
	return FileHeader{pathSize, size, mode}
}
