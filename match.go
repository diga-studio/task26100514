package scandiff

// maxLineShift 配对允许的最大行号差（含）。
const maxLineShift = 50

// solution 是一个可行配对方案的评价指标。
type solution struct {
	count int // 配对数量
	cost  int // 全部配对的绝对行号差之和
}

// better 比较两个方案：先比数量，数量相同比代价。相等视为不更优，
// 配合固定的转移优先级保证结果确定。
func (a solution) better(b solution) bool {
	if a.count != b.count {
		return a.count > b.count
	}
	return a.cost < b.cost
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// matchGroup 在同组内计算最优配对，返回 (旧索引, 新索引) 对。
// oldLines、newLines 必须各自升序且无重复。
// 配对约束：|old-new| <= maxLineShift，每条命中最多配对一次，配对不交叉。
// 目标：配对数量最大；数量最大时总位移最小。
// 非交叉约束下最优解可由保序 DP 求得：任何交叉配对都可交换为不交叉且不劣的方案。
func matchGroup(oldLines, newLines []int) [][2]int {
	n, m := len(oldLines), len(newLines)
	if n == 0 || m == 0 {
		return nil
	}

	// dp[i][j] = 用 oldLines[:i] 与 newLines[:j] 能得到的最优方案。
	dp := make([][]solution, n+1)
	for i := range dp {
		dp[i] = make([]solution, m+1)
	}
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			best := dp[i-1][j]
			if s := dp[i][j-1]; s.better(best) {
				best = s
			}
			if abs(oldLines[i-1]-newLines[j-1]) <= maxLineShift {
				s := dp[i-1][j-1]
				s.count++
				s.cost += abs(oldLines[i-1] - newLines[j-1])
				if s.better(best) {
					best = s
				}
			}
			dp[i][j] = best
		}
	}

	// 回溯：转移优先级固定（跳过旧 > 跳过新 > 配对），
	// 严格更优才替换，因此回溯时按同样顺序检查即可复现唯一选择。
	var pairs [][2]int
	i, j := n, m
	for i > 0 && j > 0 {
		cur := dp[i][j]
		if dp[i-1][j] == cur {
			i--
			continue
		}
		if dp[i][j-1] == cur {
			j--
			continue
		}
		pairs = append(pairs, [2]int{i - 1, j - 1})
		i--
		j--
	}
	for l, r := 0, len(pairs)-1; l < r; l, r = l+1, r-1 {
		pairs[l], pairs[r] = pairs[r], pairs[l]
	}
	return pairs
}
