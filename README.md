# scandiff

历史扫描差分库（Go，纯本地文件持久化，无外部依赖）。由上层直接调用，
用于比较同一项目任意两次扫描，区分**持续**、**已修复**、**新增**问题。

## 能力

- `Open(dir)` 打开/创建库；数据保存在 `dir/scans.json`，关闭重开后仍可比较。
- `RegisterScan(id, records)` 登记一次完整扫描；登记后不可修改，重复 id 报错。
- `Compare(oldID, newID)` 比较两次扫描，新旧由调用方显式指定，与登记顺序无关。

## 记录模型

```go
type Record struct {
    Path        string // 文件路径
    RuleID      string // 含版本的规则标识
    Fingerprint string // 命中文本指纹（调用方计算）
    Line        int    // 从 1 开始的行号
}
```

路径、规则标识、指纹均相同的记录属于同组；同组同一行最多一条；输入顺序任意。

## 配对规则

- 配对只发生在同组内，且两端行号差绝对值 ≤ 50（含 50）。
- 每条命中最多参与一次配对；同组配对不交叉（保序）。
- 每组先最大化配对数量，再最小化全部配对的绝对行号差之和；
  仍有多解时按固定规则选择，输入顺序变化或重开库不改变结果。
- 已配对 = 持续（`Diff.Persisted`），未配对旧记录 = 已修复（`Diff.Fixed`），
  未配对的新记录 = 新增（`Diff.Added`）。相同指纹的多次命中各自独立归类，不折叠。

## 用法

```go
s, _ := scandiff.Open("/path/to/db")
defer s.Close()

s.RegisterScan("scan-2026-09-01", oldRecords)
s.RegisterScan("scan-2026-10-01", newRecords)

d, _ := s.Compare("scan-2026-09-01", "scan-2026-10-01")
for _, p := range d.Persisted { /* 持续：p.Old -> p.New */ }
for _, r := range d.Fixed     { /* 已修复 */ }
for _, r := range d.Added     { /* 新增 */ }
```

## 测试

```sh
go test ./...
```

覆盖：题目给定场景（总位移 98 与总位移 2）、50/51 行边界、
小规模随机样本与暴力最优对拍、归类守恒、输入顺序无关、重开库持久化、
登记不可变与非法输入校验。
