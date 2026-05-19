package platform

import (
	"fmt"
	"io"
	"mime/multipart"
	"path"
	"strings"
	"time"
)

func BuildObjectKey(prefix, fileName string) string {
	ts := time.Now().UTC().Format("20060102/150405")
	safeName := strings.ReplaceAll(fileName, " ", "_")
	return path.Join(prefix, ts, fmt.Sprintf("%d_%s", time.Now().UnixNano(), safeName))
}
func ReadMultipartFile(header *multipart.FileHeader) ([]byte, error) {
	file, err := header.Open()
	if err != nil {
		return nil, fmt.Errorf("open multipart file: %w", err)
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("read multipart file: %w", err)
	}
	return data, nil
}
