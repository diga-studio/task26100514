package scandiff

import (
	"errors"
	"fmt"
	"sort"
)

// ErrScanExists 表示重复登记同一扫描标识。
var ErrScanExists = errors.New("scandiff: scan already registered")

// ErrScanNotFound 表示比较的扫描未登记。
var ErrScanNotFound = errors.New("scandiff: scan not found")

// Store 是扫描差分库，数据保存在本地文件中。
// 不是并发安全的；调用方需自行串行化访问。
type Store struct {
	path  string
	scans map[string][]HitRecord
}

// Open 打开（必要时创建）位于 path 的差分库。
func Open(path string) (*Store, error) {
	scans, err := loadScans(path)
	if err != nil {
		return nil, err
	}
	return &Store{path: path, scans: scans}, nil
}

// Close 关闭库，保证数据已落盘。
func (s *Store) Close() error {
	return saveScans(s.path, s.scans)
}

// RegisterScan 登记一次完整扫描。登记后记录不可修改，立即落盘。
func (s *Store) RegisterScan(id string, hits []HitRecord) error {
	if _, ok := s.scans[id]; ok {
		return fmt.Errorf("%w: %q", ErrScanExists, id)
	}
	stored := make([]HitRecord, len(hits))
	copy(stored, hits)
	s.scans[id] = stored
	return saveScans(s.path, s.scans)
}

// Compare 比较两次已登记的扫描，旧扫描与新扫描由调用方指定，与登记顺序无关。
func (s *Store) Compare(oldID, newID string) (*Comparison, error) {
	oldHits, ok := s.scans[oldID]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrScanNotFound, oldID)
	}
	newHits, ok := s.scans[newID]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrScanNotFound, newID)
	}

	oldGroups := groupByKey(oldHits)
	newGroups := groupByKey(newHits)

	keys := make([]groupKey, 0, len(oldGroups)+len(newGroups))
	seen := make(map[groupKey]bool, len(oldGroups)+len(newGroups))
	for k := range oldGroups {
		seen[k] = true
		keys = append(keys, k)
	}
	for k := range newGroups {
		if !seen[k] {
			keys = append(keys, k)
		}
	}
	sort.Slice(keys, func(i, j int) bool { return lessKey(keys[i], keys[j]) })

	cmp := &Comparison{OldScanID: oldID, NewScanID: newID}
	for _, k := range keys {
		og := oldGroups[k]
		ng := newGroups[k] // 不存在时为 nil，matchGroup 自然处理
		oldPair := matchGroup(og, ng)
		newPaired := make([]bool, len(ng))
		for i, j := range oldPair {
			if j >= 0 {
				newPaired[j] = true
				cmp.Hits = append(cmp.Hits, ClassifiedHit{
					HitRecord: HitRecord{Path: k.Path, RuleID: k.RuleID, Fingerprint: k.Fingerprint, Line: og[i].Line},
					Category:  Ongoing,
					OldLine:   og[i].Line,
					NewLine:   ng[j].Line,
				})
			} else {
				cmp.Hits = append(cmp.Hits, ClassifiedHit{
					HitRecord: HitRecord{Path: k.Path, RuleID: k.RuleID, Fingerprint: k.Fingerprint, Line: og[i].Line},
					Category:  Fixed,
				})
			}
		}
		for j, h := range ng {
			if !newPaired[j] {
				cmp.Hits = append(cmp.Hits, ClassifiedHit{
					HitRecord: HitRecord{Path: k.Path, RuleID: k.RuleID, Fingerprint: k.Fingerprint, Line: h.Line},
					Category:  Added,
				})
			}
		}
	}
	// 输出顺序确定化：与输入顺序及遍历顺序无关。
	sort.SliceStable(cmp.Hits, func(i, j int) bool {
		a, b := cmp.Hits[i], cmp.Hits[j]
		if a.Category != b.Category {
			return a.Category < b.Category
		}
		if lessKey(keyOf(a.HitRecord), keyOf(b.HitRecord)) {
			return true
		}
		if lessKey(keyOf(b.HitRecord), keyOf(a.HitRecord)) {
			return false
		}
		if a.OldLine != b.OldLine {
			return a.OldLine < b.OldLine
		}
		return a.Line < b.Line
	})
	return cmp, nil
}

// groupByKey 按组键分组，保持组内原有相对顺序。
func groupByKey(hits []HitRecord) map[groupKey][]HitRecord {
	groups := make(map[groupKey][]HitRecord)
	for _, h := range hits {
		k := keyOf(h)
		groups[k] = append(groups[k], h)
	}
	return groups
}

func lessKey(a, b groupKey) bool {
	if a.Path != b.Path {
		return a.Path < b.Path
	}
	if a.RuleID != b.RuleID {
		return a.RuleID < b.RuleID
	}
	return a.Fingerprint < b.Fingerprint
}
