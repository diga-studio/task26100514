package scandiff

import (
	"math/rand"
	"reflect"
	"sort"
	"testing"
)

func rec(path, rule, fp string, line int) Record {
	return Record{Path: path, RuleID: rule, Fingerprint: fp, Line: line}
}

// registerBoth 登记新旧两次扫描。
func registerBoth(t *testing.T, s *Store, oldRecords, newRecords []Record) {
	t.Helper()
	if err := s.RegisterScan("old", oldRecords); err != nil {
		t.Fatalf("登记旧扫描失败: %v", err)
	}
	if err := s.RegisterScan("new", newRecords); err != nil {
		t.Fatalf("登记新扫描失败: %v", err)
	}
}

// checkConservation 验证归类守恒：所有记录必须且只能归入一类。
func checkConservation(t *testing.T, d *Diff, oldCount, newCount int) {
	t.Helper()
	if got := len(d.Persisted)*2 + len(d.Fixed) + len(d.Added); got != oldCount+newCount {
		t.Fatalf("归类不守恒: 分类总数 %d，输入总数 %d", got, oldCount+newCount)
	}
	if len(d.Persisted)+len(d.Fixed) != oldCount {
		t.Fatalf("旧记录不守恒: %d + %d != %d", len(d.Persisted), len(d.Fixed), oldCount)
	}
	if len(d.Persisted)+len(d.Added) != newCount {
		t.Fatalf("新记录不守恒: %d + %d != %d", len(d.Persisted), len(d.Added), newCount)
	}
}

// TestCompareGivenCases 通过 Store 端到端复现题目场景。
func TestCompareGivenCases(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	mk := func(lines ...int) []Record {
		rs := make([]Record, len(lines))
		for i, l := range lines {
			rs[i] = rec("a.go", "RULE-1@v2", "fp", l)
		}
		return rs
	}
	mk2 := func(lines ...int) []Record {
		rs := make([]Record, len(lines))
		for i, l := range lines {
			rs[i] = rec("a.go", "RULE-1@v2", "fp2", l)
		}
		return rs
	}
	// 场景一（组 fp）：旧 10、60，新 59、109 → 两对，总位移 98。
	// 场景二（组 fp2）：旧 10、40、60，新 39、61 → 两对，总位移 2，旧 10 已修复。
	registerBoth(t, s, append(mk(10, 60), mk2(10, 40, 60)...),
		append(mk(59, 109), mk2(39, 61)...))
	d, err := s.Compare("old", "new")
	if err != nil {
		t.Fatal(err)
	}
	checkConservation(t, d, 5, 4)
	if len(d.Persisted) != 4 || len(d.Fixed) != 1 || len(d.Added) != 0 {
		t.Fatalf("配对=%d 修复=%d 新增=%d，期望 4/1/0",
			len(d.Persisted), len(d.Fixed), len(d.Added))
	}
	shiftByGroup := map[string]int{}
	for _, p := range d.Persisted {
		shiftByGroup[p.Old.Fingerprint] += abs(p.Old.Line - p.New.Line)
	}
	if shiftByGroup["fp"] != 98 {
		t.Fatalf("场景一期望总位移 98，得到 %d", shiftByGroup["fp"])
	}
	if shiftByGroup["fp2"] != 2 {
		t.Fatalf("场景二期望总位移 2，得到 %d", shiftByGroup["fp2"])
	}
	if d.Fixed[0].Fingerprint != "fp2" || d.Fixed[0].Line != 10 {
		t.Fatalf("组 fp2 旧第 10 行应报告为已修复，得到 %+v", d.Fixed[0])
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestReopenAndCallerSpecifiedOrder 关闭重开后仍可比较，且新旧由调用方指定。
func TestReopenAndCallerSpecifiedOrder(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	// 故意按“新先旧后”的登记顺序，验证比较不依赖登记顺序。
	registerBoth(t, s, []Record{rec("a.go", "R@v1", "f", 10)}, nil)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	s2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	d, err := s2.Compare("old", "new")
	if err != nil {
		t.Fatal(err)
	}
	checkConservation(t, d, 1, 0)
	if len(d.Fixed) != 1 || d.Fixed[0].Line != 10 {
		t.Fatalf("重开后旧记录应为已修复: %+v", d.Fixed)
	}
	// 反向比较：同一记录变为新增。
	d2, err := s2.Compare("new", "old")
	if err != nil {
		t.Fatal(err)
	}
	if len(d2.Added) != 1 || d2.Added[0].Line != 10 {
		t.Fatalf("反向比较应为新增: %+v", d2.Added)
	}
}

// TestRegisterImmutability 登记后不可修改，非法输入被拒绝。
func TestRegisterImmutability(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RegisterScan("s1", []Record{rec("a", "R", "f", 1)}); err != nil {
		t.Fatal(err)
	}
	if err := s.RegisterScan("s1", nil); err == nil {
		t.Fatal("重复登记应报错")
	}
	if err := s.RegisterScan("", nil); err == nil {
		t.Fatal("空 id 应报错")
	}
	if err := s.RegisterScan("bad-line", []Record{rec("a", "R", "f", 0)}); err == nil {
		t.Fatal("行号 0 应报错")
	}
	dup := []Record{rec("a", "R", "f", 5), rec("a", "R", "f", 5)}
	if err := s.RegisterScan("dup", dup); err == nil {
		t.Fatal("同组同行多条应报错")
	}
	// 失败的登记不应污染库。
	if _, err := s.Compare("dup", "s1"); err == nil {
		t.Fatal("非法扫描不应被登记")
	}
	if _, err := s.Compare("s1", "missing"); err == nil {
		t.Fatal("不存在的扫描应报错")
	}
}

// TestInputOrderIndependence 输入顺序变化与重开库都不改变结果。
func TestInputOrderIndependence(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	oldRecords := []Record{
		rec("a.go", "R@v1", "f1", 10), rec("a.go", "R@v1", "f1", 60),
		rec("a.go", "R@v1", "f2", 5), rec("b.go", "R@v2", "f3", 100),
		rec("b.go", "R@v2", "f3", 130),
	}
	newRecords := []Record{
		rec("a.go", "R@v1", "f1", 59), rec("a.go", "R@v1", "f1", 109),
		rec("a.go", "R@v1", "f2", 5), rec("b.go", "R@v2", "f3", 90),
	}

	run := func(shuffle bool) *Diff {
		o := append([]Record(nil), oldRecords...)
		n := append([]Record(nil), newRecords...)
		if shuffle {
			rng.Shuffle(len(o), func(i, j int) { o[i], o[j] = o[j], o[i] })
			rng.Shuffle(len(n), func(i, j int) { n[i], n[j] = n[j], n[i] })
		}
		s, err := Open(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		registerBoth(t, s, o, n)
		d, err := s.Compare("old", "new")
		if err != nil {
			t.Fatal(err)
		}
		checkConservation(t, d, len(o), len(n))
		return d
	}

	base := run(false)
	// 同指纹多次命中不得折叠：f1 组两行都配对，共 4 对。
	if len(base.Persisted) != 4 || len(base.Fixed) != 1 || len(base.Added) != 0 {
		t.Fatalf("配对=%d 修复=%d 新增=%d，期望 4/1/0",
			len(base.Persisted), len(base.Fixed), len(base.Added))
	}
	for i := 0; i < 20; i++ {
		if got := run(true); !reflect.DeepEqual(normalize(base), normalize(got)) {
			t.Fatalf("第 %d 次乱序结果与基准不一致", i)
		}
	}
}

// normalize 把 Diff 排序成可比较的规范形式。
func normalize(d *Diff) *Diff {
	out := &Diff{}
	out.Persisted = append(out.Persisted, d.Persisted...)
	out.Fixed = append(out.Fixed, d.Fixed...)
	out.Added = append(out.Added, d.Added...)
	less := func(rs []Record) func(i, j int) bool {
		return func(i, j int) bool {
			if rs[i].Path != rs[j].Path {
				return rs[i].Path < rs[j].Path
			}
			if rs[i].RuleID != rs[j].RuleID {
				return rs[i].RuleID < rs[j].RuleID
			}
			if rs[i].Fingerprint != rs[j].Fingerprint {
				return rs[i].Fingerprint < rs[j].Fingerprint
			}
			return rs[i].Line < rs[j].Line
		}
	}
	sort.Slice(out.Persisted, func(i, j int) bool {
		a, b := out.Persisted[i], out.Persisted[j]
		if a.Old.Line != b.Old.Line {
			return a.Old.Line < b.Old.Line
		}
		return a.New.Line < b.New.Line
	})
	sort.Slice(out.Fixed, less(out.Fixed))
	sort.Slice(out.Added, less(out.Added))
	return out
}
