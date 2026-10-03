// Package scandiff 提供历史扫描登记与两次扫描之间的差分比较能力。
package scandiff

// Record 是一条调用方提供的命中记录。
// Path、RuleID、Fingerprint 均相同的记录属于同组。
type Record struct {
	Path        string // 文件路径
	RuleID      string // 含版本的规则标识
	Fingerprint string // 命中文本指纹（调用方计算）
	Line        int    // 从 1 开始的行号
}

// Pair 表示一对已配对的持续问题。
type Pair struct {
	Old Record
	New Record
}

// Diff 是两次扫描的比较结果。所有记录必须且只能归入其中一类。
type Diff struct {
	Persisted []Pair   // 持续问题（已配对）
	Fixed     []Record // 已修复（未配对的旧记录）
	Added     []Record // 新增（未配对的新记录）
}
