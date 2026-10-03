package scandiff

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// fileFormat 是落盘文件的顶层结构。
type fileFormat struct {
	Version int                    `json:"version"`
	Scans   map[string][]HitRecord `json:"scans"`
}

const fileVersion = 1

// loadScans 从文件读取全部扫描；文件不存在时返回空集。
func loadScans(path string) (map[string][]HitRecord, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string][]HitRecord{}, nil
	}
	if err != nil {
		return nil, err
	}
	var f fileFormat
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("scandiff: corrupt store file %s: %w", path, err)
	}
	if f.Scans == nil {
		f.Scans = map[string][]HitRecord{}
	}
	return f.Scans, nil
}

// saveScans 把全部扫描原子地写入文件（先写临时文件再重命名）。
func saveScans(path string, scans map[string][]HitRecord) error {
	data, err := json.MarshalIndent(fileFormat{Version: fileVersion, Scans: scans}, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".scandiff-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}
