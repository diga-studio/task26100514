package scandiff

import (
	"errors"
	"path/filepath"
	"testing"
)

func openTemp(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "scans.json")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return s, path
}

func TestRegisterAndCompareAcrossReopen(t *testing.T) {
	s, path := openTemp(t)
	// 故意先登记"新"扫描，验证比较方向由调用方指定而非登记顺序。
	if err := s.RegisterScan("scan-new", hitsAt(59, 109)); err != nil {
		t.Fatalf("register new: %v", err)
	}
	if err := s.RegisterScan("scan-old", hitsAt(10, 60)); err != nil {
		t.Fatalf("register old: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// 重新打开后仍可比较。
	s2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()
	cmp, err := s2.Compare("scan-old", "scan-new")
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	ongoing, fixed, added := cmp.Counts()
	if ongoing != 2 || fixed != 0 || added != 0 {
		t.Fatalf("got ongoing=%d fixed=%d added=%d, want 2/0/0", ongoing, fixed, added)
	}
	totalShift := 0
	for _, h := range cmp.Hits {
		if h.Category != Ongoing {
			t.Fatalf("unexpected category %v", h.Category)
		}
		totalShift += absDiff(h.OldLine, h.NewLine)
	}
	if totalShift != 98 {
		t.Fatalf("total shift = %d, want 98", totalShift)
	}
}

func TestCompareFixedAndAdded(t *testing.T) {
	s, _ := openTemp(t)
	defer s.Close()
	oldHits := []HitRecord{
		{Path: "a.go", RuleID: "R1@v1", Fingerprint: "fp1", Line: 10},
		{Path: "a.go", RuleID: "R1@v1", Fingerprint: "fp1", Line: 40},
		{Path: "a.go", RuleID: "R1@v1", Fingerprint: "fp1", Line: 60},
		{Path: "b.go", RuleID: "R2@v2", Fingerprint: "fp2", Line: 5},
	}
	newHits := []HitRecord{
		{Path: "a.go", RuleID: "R1@v1", Fingerprint: "fp1", Line: 39},
		{Path: "a.go", RuleID: "R1@v1", Fingerprint: "fp1", Line: 61},
		{Path: "b.go", RuleID: "R2@v2", Fingerprint: "fp2", Line: 500}, // 位移超 50，不配对
		{Path: "c.go", RuleID: "R3@v1", Fingerprint: "fp3", Line: 1},   // 全新组
	}
	if err := s.RegisterScan("old", oldHits); err != nil {
		t.Fatal(err)
	}
	if err := s.RegisterScan("new", newHits); err != nil {
		t.Fatal(err)
	}
	cmp, err := s.Compare("old", "new")
	if err != nil {
		t.Fatal(err)
	}
	ongoing, fixed, added := cmp.Counts()
	if ongoing != 2 || fixed != 2 || added != 2 {
		t.Fatalf("got ongoing=%d fixed=%d added=%d, want 2/2/2", ongoing, fixed, added)
	}
	// 归类守恒：每条输入记录恰好出现一次。
	if len(cmp.Hits) != len(oldHits)+len(newHits)-ongoing {
		t.Fatalf("conservation violated: %d classified hits", len(cmp.Hits))
	}
	if got := ongoing*2 + fixed + added; got != len(oldHits)+len(newHits) {
		t.Fatalf("conservation: %d != %d", got, len(oldHits)+len(newHits))
	}
	// 旧第 10 行必须报告为已修复。
	foundFixed10 := false
	for _, h := range cmp.Hits {
		if h.Category == Fixed && h.Line == 10 && h.Fingerprint == "fp1" {
			foundFixed10 = true
		}
	}
	if !foundFixed10 {
		t.Fatal("old line 10 must be reported as fixed")
	}
}

func TestCompareDuplicateFingerprintsNotCollapsed(t *testing.T) {
	s, _ := openTemp(t)
	defer s.Close()
	// 同组同一指纹的 3 次命中，两侧各 3 条，必须全部保留。
	if err := s.RegisterScan("old", hitsAt(10, 20, 30)); err != nil {
		t.Fatal(err)
	}
	if err := s.RegisterScan("new", hitsAt(11, 21, 31)); err != nil {
		t.Fatal(err)
	}
	cmp, err := s.Compare("old", "new")
	if err != nil {
		t.Fatal(err)
	}
	ongoing, fixed, added := cmp.Counts()
	if ongoing != 3 || fixed != 0 || added != 0 {
		t.Fatalf("got ongoing=%d fixed=%d added=%d, want 3/0/0", ongoing, fixed, added)
	}
}

func TestCompareInputOrderIndependentAcrossReopen(t *testing.T) {
	mkHits := func(lines []int, seed int64) []HitRecord {
		hits := hitsAt(lines...)
		// 简单确定性乱序
		for i := range hits {
			j := int(seed+int64(i)) % len(hits)
			hits[i], hits[j] = hits[j], hits[i]
		}
		return hits
	}
	oldLines := []int{10, 40, 60, 90, 130}
	newLines := []int{39, 61, 88, 200}

	s1, _ := openTemp(t)
	defer s1.Close()
	s1.RegisterScan("old", mkHits(oldLines, 1))
	s1.RegisterScan("new", mkHits(newLines, 2))
	cmp1, err := s1.Compare("old", "new")
	if err != nil {
		t.Fatal(err)
	}

	s2, path2 := openTemp(t)
	s2.RegisterScan("old", mkHits(oldLines, 3))
	s2.RegisterScan("new", mkHits(newLines, 4))
	s2.Close()
	// 重新打开后再比较，结果也必须一致。
	s3, err := Open(path2)
	if err != nil {
		t.Fatal(err)
	}
	defer s3.Close()
	cmp2, err := s3.Compare("old", "new")
	if err != nil {
		t.Fatal(err)
	}

	if len(cmp1.Hits) != len(cmp2.Hits) {
		t.Fatalf("hit count differs: %d vs %d", len(cmp1.Hits), len(cmp2.Hits))
	}
	for i := range cmp1.Hits {
		if cmp1.Hits[i] != cmp2.Hits[i] {
			t.Fatalf("hit %d differs: %+v vs %+v", i, cmp1.Hits[i], cmp2.Hits[i])
		}
	}
}

func TestRegisterDuplicateAndUnknownScan(t *testing.T) {
	s, _ := openTemp(t)
	defer s.Close()
	if err := s.RegisterScan("a", hitsAt(1)); err != nil {
		t.Fatal(err)
	}
	if err := s.RegisterScan("a", hitsAt(2)); !errors.Is(err, ErrScanExists) {
		t.Fatalf("want ErrScanExists, got %v", err)
	}
	if _, err := s.Compare("a", "missing"); !errors.Is(err, ErrScanNotFound) {
		t.Fatalf("want ErrScanNotFound, got %v", err)
	}
	if _, err := s.Compare("missing", "a"); !errors.Is(err, ErrScanNotFound) {
		t.Fatalf("want ErrScanNotFound, got %v", err)
	}
}
