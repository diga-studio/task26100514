package scandiff

import (
	"math/rand"
	"testing"
)

// totalShift 计算配对结果的绝对行号差之和。
func totalShift(oldLines, newLines []int, pairs [][2]int) int {
	total := 0
	for _, p := range pairs {
		total += abs(oldLines[p[0]] - newLines[p[1]])
	}
	return total
}

// TestMatchGroupGivenCases 覆盖题目指定的两个场景。
func TestMatchGroupGivenCases(t *testing.T) {
	// 旧 10、60，新 59、109：两对持续，总位移 98。
	pairs := matchGroup([]int{10, 60}, []int{59, 109})
	if len(pairs) != 2 {
		t.Fatalf("期望 2 对，得到 %d 对", len(pairs))
	}
	if got := totalShift([]int{10, 60}, []int{59, 109}, pairs); got != 98 {
		t.Fatalf("期望总位移 98，得到 %d", got)
	}

	// 旧 10、40、60，新 39、61：保留两对、总位移 2，旧第 10 行未配对。
	pairs = matchGroup([]int{10, 40, 60}, []int{39, 61})
	if len(pairs) != 2 {
		t.Fatalf("期望 2 对，得到 %d 对", len(pairs))
	}
	if got := totalShift([]int{10, 40, 60}, []int{39, 61}, pairs); got != 2 {
		t.Fatalf("期望总位移 2，得到 %d", got)
	}
	for _, p := range pairs {
		if p[0] == 0 {
			t.Fatalf("旧第 10 行不应参与配对")
		}
	}
}

// TestMatchGroupBoundary 验证 50 行可配、51 行不可配。
func TestMatchGroupBoundary(t *testing.T) {
	if pairs := matchGroup([]int{100}, []int{150}); len(pairs) != 1 {
		t.Fatalf("行号差 50 应可配对")
	}
	if pairs := matchGroup([]int{100}, []int{50}); len(pairs) != 1 {
		t.Fatalf("行号差 -50 应可配对")
	}
	if pairs := matchGroup([]int{100}, []int{151}); len(pairs) != 0 {
		t.Fatalf("行号差 51 不应配对")
	}
	if pairs := matchGroup([]int{100}, []int{49}); len(pairs) != 0 {
		t.Fatalf("行号差 -51 不应配对")
	}
}

// bruteForce 独立计算最优方案的 (数量, 总位移)：枚举所有不交叉保序配对。
func bruteForce(oldLines, newLines []int) (int, int) {
	best := solution{}
	var rec func(i, j, count, cost int)
	rec = func(i, j, count, cost int) {
		if i == len(oldLines) || j == len(newLines) {
			s := solution{count: count, cost: cost}
			if s.better(best) {
				best = s
			}
			return
		}
		// 跳过旧 i
		rec(i+1, j, count, cost)
		// 跳过新 j
		rec(i, j+1, count, cost)
		// 配对 i 与 j（保序，天然不交叉）
		if d := abs(oldLines[i] - newLines[j]); d <= maxLineShift {
			rec(i+1, j+1, count+1, cost+d)
		}
	}
	rec(0, 0, 0, 0)
	return best.count, best.cost
}

// TestMatchGroupAgainstBruteForce 小规模随机样本与暴力最优结果对拍。
func TestMatchGroupAgainstBruteForce(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	for trial := 0; trial < 500; trial++ {
		oldLines := randomLines(rng, rng.Intn(7))
		newLines := randomLines(rng, rng.Intn(7))
		pairs := matchGroup(oldLines, newLines)

		wantCount, wantCost := bruteForce(oldLines, newLines)
		if len(pairs) != wantCount {
			t.Fatalf("trial %d: old=%v new=%v 配对数 %d，暴力最优 %d",
				trial, oldLines, newLines, len(pairs), wantCount)
		}
		if got := totalShift(oldLines, newLines, pairs); got != wantCost {
			t.Fatalf("trial %d: old=%v new=%v 总位移 %d，暴力最优 %d",
				trial, oldLines, newLines, got, wantCost)
		}

		// 校验配对本身合法：位移上限、不交叉、每条命中最多一次。
		for k, p := range pairs {
			if abs(oldLines[p[0]]-newLines[p[1]]) > maxLineShift {
				t.Fatalf("trial %d: 配对超出位移上限", trial)
			}
			if k > 0 && (pairs[k-1][0] >= p[0] || pairs[k-1][1] >= p[1]) {
				t.Fatalf("trial %d: 配对交叉", trial)
			}
		}
	}
}

// randomLines 生成 n 个升序不重复的 1..200 行号。
func randomLines(rng *rand.Rand, n int) []int {
	used := make(map[int]bool)
	lines := make([]int, 0, n)
	for len(lines) < n {
		l := rng.Intn(200) + 1
		if !used[l] {
			used[l] = true
			lines = append(lines, l)
		}
	}
	for i := 1; i < len(lines); i++ {
		for j := i; j > 0 && lines[j-1] > lines[j]; j-- {
			lines[j-1], lines[j] = lines[j], lines[j-1]
		}
	}
	return lines
}
