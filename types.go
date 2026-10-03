// Package scandiff 提供历史扫描差分能力：登记完整扫描结果，
// 并比较同一项目任意两次扫描，区分新增、已修复与持续存在的问题。
package scandiff

// HitRecord 是调用方提供的一条命中记录。
type HitRecord struct {
	Path        string // 文件路径
	RuleID      string // 含版本的规则标识
	Fingerprint string // 命中文本指纹（由调用方计算）
	Line        int    // 从 1 开始的行号
}

// groupKey 用于把记录分组：路径、规则标识、指纹均相同的记录属于同组。
type groupKey struct {
	Path        string
	RuleID      string
	Fingerprint string
}

func keyOf(h HitRecord) groupKey {
	return groupKey{Path: h.Path, RuleID: h.RuleID, Fingerprint: h.Fingerprint}
}

// Category 表示比较结果中一条记录的归类。
type Category int

const (
	Ongoing Category = iota // 持续存在（已配对）
	Fixed                   // 已修复（仅存在于旧扫描）
	Added                   // 新增（仅存在于新扫描）
)

func (c Category) String() string {
	switch c {
	case Ongoing:
		return "ongoing"
	case Fixed:
		return "fixed"
	case Added:
		return "added"
	}
	return "unknown"
}

// ClassifiedHit 是一条已归类的记录。OldLine/NewLine 仅在持续问题中同时有效。
type ClassifiedHit struct {
	HitRecord
	Category Category
	OldLine  int // 持续问题：旧扫描中的行号；其余为 0
	NewLine  int // 持续问题：新扫描中的行号；其余为 0
}

// Comparison 是两次扫描的比较结果。
type Comparison struct {
	OldScanID string
	NewScanID string
	Hits      []ClassifiedHit
}

// Counts 返回各类归类的记录数。
func (c *Comparison) Counts() (ongoing, fixed, added int) {
	for _, h := range c.Hits {
		switch h.Category {
		case Ongoing:
			ongoing++
		case Fixed:
			fixed++
		case Added:
			added++
		}
	}
	return
}
