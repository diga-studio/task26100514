# scandiff — 历史扫描差分库

Go 库，由上层直接调用。登记完整扫描结果到本地文件，并比较同一项目任意两次扫描，
区分**新增（added）**、**已修复（fixed）**与**持续存在（ongoing）**的问题。

## 使用

```go
s, _ := scandiff.Open("scans.json") // 本地文件持久化，不存在则创建
defer s.Close()

// 登记完整扫描（不可重复登记同一 id；登记后不可修改，立即落盘）
s.RegisterScan("2026-09-01", []scandiff.HitRecord{
    {Path: "a.go", RuleID: "SQLi@v2", Fingerprint: "fp-abc", Line: 10},
    {Path: "a.go", RuleID: "SQLi@v2", Fingerprint: "fp-abc", Line: 60},
})
s.RegisterScan("2026-10-01", []scandiff.HitRecord{
    {Path: "a.go", RuleID: "SQLi@v2", Fingerprint: "fp-abc", Line: 59},
    {Path: "a.go", RuleID: "SQLi@v2", Fingerprint: "fp-abc", Line: 109},
})

// 旧/新扫描由调用方指定，与登记顺序无关；关闭重开后仍可比较
cmp, _ := s.Compare("2026-09-01", "2026-10-01")
ongoing, fixed, added := cmp.Counts() // 2, 0, 0
for _, h := range cmp.Hits {
    // h.Category: Ongoing / Fixed / Added
    // Ongoing 时 h.OldLine、h.NewLine 分别为两侧行号
}
```

## 配对规则

- 分组：`Path`、`RuleID`（含版本）、`Fingerprint` 三者均相同的记录为一组，配对只发生在同组内。
- 同组同一行最多一条记录；同一指纹的多次命中各自独立参与配对，不会被折叠。
- 配对条件：两端行号差绝对值 ≤ 50（含 50）；每条命中最多配对一次；配对不交叉。
- 每组的最优目标：先最大化配对数量，再最小化全部配对的绝对行号差之和
  （同组内按行号排序后的动态规划，等价于带权非交叉匹配）。
- 结果确定：与输入顺序无关，关闭重开库后比较结果一致。

## 测试

```sh
go test ./...
```

覆盖：最优配对（含暴力枚举对照的随机样本）、50 行可配 / 51 行不可配边界、
归类守恒（每条记录恰好归入一类）、输入顺序无关、跨重开持久化、
重复登记与未知扫描的错误处理。
