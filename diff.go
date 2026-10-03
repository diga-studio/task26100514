package scandiff

import "sort"

// groupKey 标识一个同组（路径、规则标识、指纹均相同）。
type groupKey struct {
	path        string
	ruleID      string
	fingerprint string
}

func keyOf(r Record) groupKey {
	return groupKey{path: r.Path, ruleID: r.RuleID, fingerprint: r.Fingerprint}
}

// diffRecords 对两次扫描的记录集合计算差分。
// 输入顺序任意，结果与输入顺序无关。
func diffRecords(oldRecords, newRecords []Record) *Diff {
	oldGroups := groupByKey(oldRecords)
	newGroups := groupByKey(newRecords)

	// 收集全部组键并排序，保证输出确定、与输入顺序无关。
	keys := make(map[groupKey]struct{}, len(oldGroups)+len(newGroups))
	for k := range oldGroups {
		keys[k] = struct{}{}
	}
	for k := range newGroups {
		keys[k] = struct{}{}
	}
	sortedKeys := make([]groupKey, 0, len(keys))
	for k := range keys {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Slice(sortedKeys, func(a, b int) bool {
		x, y := sortedKeys[a], sortedKeys[b]
		if x.path != y.path {
			return x.path < y.path
		}
		if x.ruleID != y.ruleID {
			return x.ruleID < y.ruleID
		}
		return x.fingerprint < y.fingerprint
	})

	diff := &Diff{}
	for _, k := range sortedKeys {
		olds := oldGroups[k]
		news := newGroups[k]
		oldLines := make([]int, len(olds))
		for i, r := range olds {
			oldLines[i] = r.Line
		}
		newLines := make([]int, len(news))
		for i, r := range news {
			newLines[i] = r.Line
		}

		matchedOld := make([]bool, len(olds))
		matchedNew := make([]bool, len(news))
		for _, p := range matchGroup(oldLines, newLines) {
			matchedOld[p[0]] = true
			matchedNew[p[1]] = true
			diff.Persisted = append(diff.Persisted, Pair{Old: olds[p[0]], New: news[p[1]]})
		}
		for i, r := range olds {
			if !matchedOld[i] {
				diff.Fixed = append(diff.Fixed, r)
			}
		}
		for i, r := range news {
			if !matchedNew[i] {
				diff.Added = append(diff.Added, r)
			}
		}
	}
	return diff
}

// groupByKey 按组键分组，每组内按行号升序。
// 同组同一行最多一条由 RegisterScan 校验，这里假定输入合法。
func groupByKey(records []Record) map[groupKey][]Record {
	groups := make(map[groupKey][]Record)
	for _, r := range records {
		k := keyOf(r)
		groups[k] = append(groups[k], r)
	}
	for _, g := range groups {
		sort.Slice(g, func(a, b int) bool { return g[a].Line < g[b].Line })
	}
	return groups
}
