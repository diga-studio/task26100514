package scandiff

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// storeFileName 是库目录中保存扫描数据的文件名。
const storeFileName = "scans.json"

// scanData 是一次已登记扫描的持久化形式。
type scanData struct {
	ID      string   `json:"id"`
	Records []Record `json:"records"`
}

// storeFile 是磁盘文件的顶层结构。
type storeFile struct {
	Scans []scanData `json:"scans"`
}

// Store 是扫描记录库，数据保存在本地目录中。
// 登记后的扫描不可修改；关闭并重新打开后仍可比较。
type Store struct {
	dir   string
	scans map[string][]Record
	order []string // 登记顺序，仅用于持久化的稳定输出
}

// Open 打开（必要时创建）位于 dir 的扫描记录库。
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("scandiff: 创建库目录失败: %w", err)
	}
	s := &Store{dir: dir, scans: make(map[string][]Record)}

	path := filepath.Join(dir, storeFileName)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scandiff: 读取库文件失败: %w", err)
	}
	var f storeFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("scandiff: 解析库文件失败: %w", err)
	}
	for _, sd := range f.Scans {
		if _, dup := s.scans[sd.ID]; dup {
			return nil, fmt.Errorf("scandiff: 库文件中扫描 %q 重复", sd.ID)
		}
		if err := validateRecords(sd.Records); err != nil {
			return nil, fmt.Errorf("scandiff: 库文件中扫描 %q 非法: %w", sd.ID, err)
		}
		s.scans[sd.ID] = sd.Records
		s.order = append(s.order, sd.ID)
	}
	return s, nil
}

// Close 关闭库。数据在登记时已持久化，Close 不做额外操作。
func (s *Store) Close() error {
	return nil
}

// RegisterScan 登记一次完整扫描。登记后记录不可修改；
// 重复登记同一 id 或记录非法时返回错误，且不会改变库内容。
func (s *Store) RegisterScan(id string, records []Record) error {
	if id == "" {
		return errors.New("scandiff: 扫描 id 不能为空")
	}
	if _, exists := s.scans[id]; exists {
		return fmt.Errorf("scandiff: 扫描 %q 已存在，登记后不可修改", id)
	}
	if err := validateRecords(records); err != nil {
		return fmt.Errorf("scandiff: 扫描 %q 记录非法: %w", id, err)
	}

	copied := make([]Record, len(records))
	copy(copied, records)
	s.scans[id] = copied
	s.order = append(s.order, id)
	if err := s.save(); err != nil {
		delete(s.scans, id)
		s.order = s.order[:len(s.order)-1]
		return err
	}
	return nil
}

// Compare 比较两次扫描，oldID 为旧扫描，newID 为新扫描，均由调用方指定。
func (s *Store) Compare(oldID, newID string) (*Diff, error) {
	oldRecords, ok := s.scans[oldID]
	if !ok {
		return nil, fmt.Errorf("scandiff: 旧扫描 %q 不存在", oldID)
	}
	newRecords, ok := s.scans[newID]
	if !ok {
		return nil, fmt.Errorf("scandiff: 新扫描 %q 不存在", newID)
	}
	return diffRecords(oldRecords, newRecords), nil
}

// validateRecords 校验一次扫描的记录：行号从 1 开始，同组同一行最多一条。
func validateRecords(records []Record) error {
	type lineKey struct {
		groupKey
		line int
	}
	seen := make(map[lineKey]struct{}, len(records))
	for i, r := range records {
		if r.Line < 1 {
			return fmt.Errorf("第 %d 条记录行号 %d 小于 1", i, r.Line)
		}
		k := lineKey{groupKey: keyOf(r), line: r.Line}
		if _, dup := seen[k]; dup {
			return fmt.Errorf("同组 (path=%q, rule=%q, fingerprint=%q) 行 %d 出现多条记录",
				r.Path, r.RuleID, r.Fingerprint, r.Line)
		}
		seen[k] = struct{}{}
	}
	return nil
}

// save 把整个库原子地写入磁盘（先写临时文件再重命名）。
func (s *Store) save() error {
	f := storeFile{Scans: make([]scanData, 0, len(s.order))}
	for _, id := range s.order {
		records := s.scans[id]
		sorted := make([]Record, len(records))
		copy(sorted, records)
		sort.Slice(sorted, func(a, b int) bool {
			x, y := sorted[a], sorted[b]
			if x.Path != y.Path {
				return x.Path < y.Path
			}
			if x.RuleID != y.RuleID {
				return x.RuleID < y.RuleID
			}
			if x.Fingerprint != y.Fingerprint {
				return x.Fingerprint < y.Fingerprint
			}
			return x.Line < y.Line
		})
		f.Scans = append(f.Scans, scanData{ID: id, Records: sorted})
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("scandiff: 序列化失败: %w", err)
	}
	tmp := filepath.Join(s.dir, storeFileName+".tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("scandiff: 写入临时文件失败: %w", err)
	}
	if err := os.Rename(tmp, filepath.Join(s.dir, storeFileName)); err != nil {
		return fmt.Errorf("scandiff: 替换库文件失败: %w", err)
	}
	return nil
}
