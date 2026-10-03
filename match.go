package scandiff

import "sort"

// maxLineShift 是配对允许的最大行号差（含）。
const maxLineShift = 50

// dpCell 是动态规划的一个状态值：pairs 为配对数，cost 为绝对行号差之和。
type dpCell struct {
	pairs int
	cost  int
}

// better 比较两个状态：配对数大者优；并列时代价小者优。
func better(a, b dpCell) bool {
	if a.pairs != b.pairs {
		return a.pairs > b.pairs
	}
	return a.cost < b.cost
}

func absDiff(a, b int) int {
	if a > b {
		return a - b
	}
	return b - a
}

// matchGroup 对同组内的旧、新记录求最优配对：
// 配对数最大，并列时绝对行号差之和最小；配对不交叉（两端各自按行号升序后顺序一致）。
// 返回与 oldHits 等长的切片，oldPair[i] 是 oldHits[i] 配到的新记录下标，-1 表示未配对。
// 结果与输入顺序无关：函数内部先按行号排序（同组同一行最多一条，排序为全序）。
func matchGroup(oldHits, newHits []HitRecord) []int {
	oldIdx := sortedIndexes(oldHits)
	newIdx := sortedIndexes(newHits)

	n, m := len(oldIdx), len(newIdx)
	// dp[i][j]：旧前 i 条与新前 j 条的最优状态。
	dp := make([][]dpCell, n+1)
	for i := range dp {
		dp[i] = make([]dpCell, m+1)
	}
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			best := dp[i-1][j]
			if better(dp[i][j-1], best) {
				best = dp[i][j-1]
			}
			d := absDiff(oldHits[oldIdx[i-1]].Line, newHits[newIdx[j-1]].Line)
			if d <= maxLineShift {
				cand := dpCell{pairs: dp[i-1][j-1].pairs + 1, cost: dp[i-1][j-1].cost + d}
				if better(cand, best) {
					best = cand
				}
			}
			dp[i][j] = best
		}
	}

	oldPair := make([]int, n)
	for i := range oldPair {
		oldPair[i] = -1
	}
	// 回溯重建配对；分支优先级固定，保证结果确定。
	for i, j := n, m; i > 0 && j > 0; {
		cur := dp[i][j]
		d := absDiff(oldHits[oldIdx[i-1]].Line, newHits[newIdx[j-1]].Line)
		if d <= maxLineShift {
			cand := dpCell{pairs: dp[i-1][j-1].pairs + 1, cost: dp[i-1][j-1].cost + d}
			if cand == cur {
				oldPair[oldIdx[i-1]] = newIdx[j-1]
				i--
				j--
				continue
			}
		}
		if dp[i-1][j] == cur {
			i--
			continue
		}
		j--
	}
	return oldPair
}

// sortedIndexes 返回按下述全序排序后的下标序列：行号升序。
// 同组同一行最多一条记录，因此行号即构成全序，排序结果与输入顺序无关。
func sortedIndexes(hits []HitRecord) []int {
	idx := make([]int, len(hits))
	for i := range idx {
		idx[i] = i
	}
	sort.Slice(idx, func(a, b int) bool {
		return hits[idx[a]].Line < hits[idx[b]].Line
	})
	return idx
}
