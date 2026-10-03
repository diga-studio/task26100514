package scandiff

import (
	"math/rand"
	"sort"
	"testing"
)

func hitsAt(lines ...int) []HitRecord {
	hits := make([]HitRecord, len(lines))
	for i, ln := range lines {
		hits[i] = HitRecord{Path: "a.go", RuleID: "R1@v1", Fingerprint: "fp", Line: ln}
	}
	return hits
}

// pairStats 汇总配对结果：配对数与总位移。
func pairStats(oldHits, newHits []HitRecord, oldPair []int) (pairs, shift int) {
	for i, j := range oldPair {
		if j >= 0 {
			pairs++
			shift += absDiff(oldHits[i].Line, newHits[j].Line)
		}
	}
	return
}

func TestMatchGroup_TwoPairsLargeShift(t *testing.T) {
	oldHits := hitsAt(10, 60)
	newHits := hitsAt(59, 109)
	pairs, shift := pairStats(oldHits, newHits, matchGroup(oldHits, newHits))
	if pairs != 2 || shift != 98 {
		t.Fatalf("got pairs=%d shift=%d, want pairs=2 shift=98", pairs, shift)
	}
}

func TestMatchGroup_PrefersMinimalShift(t *testing.T) {
	oldHits := hitsAt(10, 40, 60)
	newHits := hitsAt(39, 61)
	oldPair := matchGroup(oldHits, newHits)
	pairs, shift := pairStats(oldHits, newHits, oldPair)
	if pairs != 2 || shift != 2 {
		t.Fatalf("got pairs=%d shift=%d, want pairs=2 shift=2", pairs, shift)
	}
	if oldPair[0] != -1 {
		t.Fatalf("old line 10 should be unpaired (fixed), got pair to new index %d", oldPair[0])
	}
}

func TestMatchGroup_ShiftBoundary(t *testing.T) {
	oldHits := hitsAt(100)
	if p := matchGroup(oldHits, hitsAt(150))[0]; p != 0 {
		t.Fatalf("shift 50 must pair, got %d", p)
	}
	if p := matchGroup(oldHits, hitsAt(151))[0]; p != -1 {
		t.Fatalf("shift 51 must not pair, got %d", p)
	}
	if p := matchGroup(oldHits, hitsAt(50))[0]; p != 0 {
		t.Fatalf("shift 50 downward must pair, got %d", p)
	}
}

// bruteForce 独立计算最优配对：枚举所有非交叉匹配，返回 (配对数, 总位移)。
func bruteForce(oldLines, newLines []int) (bestPairs, bestCost int) {
	sort.Ints(oldLines)
	sort.Ints(newLines)
	var rec func(i, j, pairs, cost int)
	rec = func(i, j, pairs, cost int) {
		if i == len(oldLines) || j == len(newLines) {
			if pairs > bestPairs || (pairs == bestPairs && cost < bestCost) {
				bestPairs, bestCost = pairs, cost
			}
			return
		}
		// 不配对旧 i
		rec(i+1, j, pairs, cost)
		// 不配对新 j
		rec(i, j+1, pairs, cost)
		// 配对 i 与 j（非交叉：两端均有序，配对后同时前进）
		if d := absDiff(oldLines[i], newLines[j]); d <= maxLineShift {
			rec(i+1, j+1, pairs+1, cost+d)
		}
	}
	rec(0, 0, 0, 0)
	return
}

func TestMatchGroup_MatchesBruteForce(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	for trial := 0; trial < 300; trial++ {
		n := rng.Intn(6)
		m := rng.Intn(6)
		oldLines := randLines(rng, n)
		newLines := randLines(rng, m)
		oldHits := hitsAt(oldLines...)
		newHits := hitsAt(newLines...)
		pairs, shift := pairStats(oldHits, newHits, matchGroup(oldHits, newHits))
		wantPairs, wantShift := bruteForce(oldLines, newLines)
		if pairs != wantPairs || shift != wantShift {
			t.Fatalf("old=%v new=%v: got pairs=%d shift=%d, brute force wants pairs=%d shift=%d",
				oldLines, newLines, pairs, shift, wantPairs, wantShift)
		}
	}
}

// randLines 生成互不相同的升序行号（同组同一行最多一条）。
func randLines(rng *rand.Rand, n int) []int {
	set := map[int]bool{}
	lines := make([]int, 0, n)
	for len(lines) < n {
		ln := 1 + rng.Intn(200)
		if !set[ln] {
			set[ln] = true
			lines = append(lines, ln)
		}
	}
	sort.Ints(lines)
	return lines
}

func TestMatchGroup_InputOrderIndependent(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for trial := 0; trial < 100; trial++ {
		oldLines := randLines(rng, 1+rng.Intn(5))
		newLines := randLines(rng, 1+rng.Intn(5))
		base := matchGroup(hitsAt(oldLines...), hitsAt(newLines...))
		// 打乱输入顺序后结果必须一致
		shuffledOld := shuffleHits(rng, hitsAt(oldLines...))
		shuffledNew := shuffleHits(rng, hitsAt(newLines...))
		got := matchGroup(shuffledOld, shuffledNew)
		gotPairs := map[[2]int]bool{}
		for i, j := range got {
			if j >= 0 {
				gotPairs[[2]int{shuffledOld[i].Line, shuffledNew[j].Line}] = true
			}
		}
		basePairs := map[[2]int]bool{}
		for i, j := range base {
			if j >= 0 {
				basePairs[[2]int{oldLines[i], newLines[j]}] = true
			}
		}
		if len(gotPairs) != len(basePairs) {
			t.Fatalf("pair count changed after shuffle: %v vs %v", gotPairs, basePairs)
		}
		for p := range basePairs {
			if !gotPairs[p] {
				t.Fatalf("pair %v missing after shuffle: got %v", p, gotPairs)
			}
		}
	}
}

func shuffleHits(rng *rand.Rand, hits []HitRecord) []HitRecord {
	out := make([]HitRecord, len(hits))
	copy(out, hits)
	rng.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}
