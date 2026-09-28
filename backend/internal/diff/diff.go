// Package diff 提供数控程序的逐行比较。
//
// 这里没有采用完整的 Myers/LCS 算法，而是使用「局部重同步」策略：
// 逐行对比，遇到不一致时在前后各 50 行的窗口内寻找下一个公共行作为同步点。
//
// 理由：标准 LCS 在 3000 行程序上需要 3000×3000 的表格（几十 MB 内存），
// 而数控程序的修改绝大多数是局部改动（改个进给、加两行、改把刀），
// 局部重同步的效果与标准 diff 几乎一致，代码量和内存占用却小一个数量级，
// 后期维护时任何人也都能看懂。
package diff

import "cnccool/internal/domain"

// syncWindow 是寻找同步点时的搜索窗口（行）。
const syncWindow = 50

// Compare 返回两段文本的逐行差异。
func Compare(left, right []string) []domain.DiffLine {
	out := make([]domain.DiffLine, 0, len(left)+len(right))
	i, j := 0, 0

	for i < len(left) && j < len(right) {
		if left[i] == right[j] {
			out = append(out, domain.DiffLine{
				Type: "same", LeftNo: intPtr(i + 1), RightNo: intPtr(j + 1),
				LeftText: left[i], RightText: right[j],
			})
			i++
			j++
			continue
		}
		// 当前位置不一致：找一个最近的公共行重新对齐
		si, sj, ok := findSync(left, right, i, j)
		if !ok {
			break // 后面再没有公共行，交给收尾逻辑处理
		}
		out = appendBlock(out, left[i:si], i, right[j:sj], j)
		i, j = si, sj
	}

	// 收尾：任一侧还有剩余内容，整段作为增/删处理
	if i < len(left) || j < len(right) {
		out = appendBlock(out, left[i:], i, right[j:], j)
	}
	return out
}

// findSync 在 [i, i+window) × [j, j+window) 范围内寻找代价最小的公共行。
// 代价 = 两侧各需要跳过的行数之和，取最小者即为最自然的对齐点。
func findSync(left, right []string, i, j int) (int, int, bool) {
	bestI, bestJ, bestCost := -1, -1, 1<<30
	maxI := min(i+syncWindow, len(left))
	maxJ := min(j+syncWindow, len(right))

	for a := i; a < maxI; a++ {
		for b := j; b < maxJ; b++ {
			if left[a] != right[b] {
				continue
			}
			if cost := (a - i) + (b - j); cost < bestCost {
				bestCost, bestI, bestJ = cost, a, b
			}
		}
	}
	if bestI < 0 {
		return 0, 0, false
	}
	return bestI, bestJ, true
}

// appendBlock 输出一段「不匹配区」：
// 先按顺序把删除行与新增行两两配对成 change，多出来的部分再分别记为 del / add。
func appendBlock(out []domain.DiffLine, dels []string, delStart int, adds []string, addStart int) []domain.DiffLine {
	paired := min(len(dels), len(adds))

	for k := 0; k < paired; k++ {
		out = append(out, domain.DiffLine{
			Type: "change",
			LeftNo: intPtr(delStart + k + 1), RightNo: intPtr(addStart + k + 1),
			LeftText: dels[k], RightText: adds[k],
		})
	}
	for k := paired; k < len(dels); k++ {
		out = append(out, domain.DiffLine{
			Type: "del", LeftNo: intPtr(delStart + k + 1), LeftText: dels[k],
		})
	}
	for k := paired; k < len(adds); k++ {
		out = append(out, domain.DiffLine{
			Type: "add", RightNo: intPtr(addStart + k + 1), RightText: adds[k],
		})
	}
	return out
}

func intPtr(v int) *int { return &v }
