// Package diff 提供数控程序的逐行比较，使用 Myers O(ND) 算法。
//
// 先裁掉公共前后缀（数控程序的修改绝大多数是局部的，这一步最省），
// 中间部分再跑 Myers。改动量超过 maxEdits 时放弃求最小差异，
// 把中间部分整块当作替换，保证超大改动也不会拖垮服务。
package diff

import "cnc-manager/internal/domain"

// maxEdits 是 Myers 允许探索的最大编辑距离；轨迹内存约 maxEdits² 级。
const maxEdits = 2000

type opKind int

const (
	opSame opKind = iota
	opDel
	opAdd
)

// Result 是比较结果。
type Result struct {
	Lines     []domain.DiffLine
	Truncated bool
}

// Compare 返回两段文本的逐行差异。相邻的删除与新增会配对成 change。
func Compare(left, right []string) Result {
	// 1. 裁公共前缀与后缀
	pre := 0
	for pre < len(left) && pre < len(right) && left[pre] == right[pre] {
		pre++
	}
	suf := 0
	for suf < len(left)-pre && suf < len(right)-pre && left[len(left)-1-suf] == right[len(right)-1-suf] {
		suf++
	}
	midL, midR := left[pre:len(left)-suf], right[pre:len(right)-suf]

	ops, ok := myers(midL, midR)
	truncated := !ok
	if !ok {
		ops = replaceAll(len(midL), len(midR))
	}

	out := make([]domain.DiffLine, 0, len(left)+len(right))
	for i := 0; i < pre; i++ {
		out = append(out, same(left, right, i, i))
	}
	i, j := pre, pre
	var dels, adds []int
	flush := func() {
		out = appendBlock(out, left, right, dels, adds)
		dels, adds = dels[:0], adds[:0]
	}
	for _, op := range ops {
		switch op {
		case opSame:
			flush()
			out = append(out, same(left, right, i, j))
			i++
			j++
		case opDel:
			dels = append(dels, i)
			i++
		case opAdd:
			adds = append(adds, j)
			j++
		}
	}
	flush()
	for k := 0; k < suf; k++ {
		out = append(out, same(left, right, i+k, j+k))
	}
	return Result{Lines: out, Truncated: truncated}
}

// Identical 判断两段文本是否逐行相同。
func Identical(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func replaceAll(n, m int) []opKind {
	ops := make([]opKind, 0, n+m)
	for i := 0; i < n; i++ {
		ops = append(ops, opDel)
	}
	for j := 0; j < m; j++ {
		ops = append(ops, opAdd)
	}
	return ops
}

// myers 求 a→b 的最短编辑脚本；编辑距离超过 maxEdits 时返回 ok=false。
func myers(a, b []string) ([]opKind, bool) {
	n, m := len(a), len(b)
	if n == 0 && m == 0 {
		return nil, true
	}
	if n == 0 || m == 0 {
		return replaceAll(n, m), true
	}
	max := n + m
	limit := min(max, maxEdits)
	offset := limit + 1
	v := make([]int, 2*limit+3)
	var trace [][]int

	for d := 0; d <= limit; d++ {
		snap := make([]int, 2*d+1)
		for k := -d; k <= d; k += 2 {
			var x int
			if k == -d || (k != d && v[offset+k-1] < v[offset+k+1]) {
				x = v[offset+k+1]
			} else {
				x = v[offset+k-1] + 1
			}
			y := x - k
			for x < n && y < m && a[x] == b[y] {
				x++
				y++
			}
			v[offset+k] = x
			if x >= n && y >= m {
				copy(snap, v[offset-d:offset+d+1])
				trace = append(trace, snap)
				return backtrack(trace, n, m), true
			}
		}
		copy(snap, v[offset-d:offset+d+1])
		trace = append(trace, snap)
	}
	return nil, false
}

// backtrack 由每轮 v 的快照回溯出编辑脚本（逆序生成后翻转）。
func backtrack(trace [][]int, n, m int) []opKind {
	x, y := n, m
	var rev []opKind
	for d := len(trace) - 1; d > 0; d-- {
		prev := trace[d-1] // 下标 = k + (d-1)
		at := func(k int) int { return prev[k+(d-1)] }
		k := x - y
		var pk int
		if k == -d || (k != d && at(k-1) < at(k+1)) {
			pk = k + 1
		} else {
			pk = k - 1
		}
		px := at(pk)
		py := px - pk
		for x > px && y > py { // 对角线：相同行
			rev = append(rev, opSame)
			x--
			y--
		}
		if x == px {
			rev = append(rev, opAdd)
		} else {
			rev = append(rev, opDel)
		}
		x, y = px, py
	}
	for x > 0 && y > 0 { // d=0：开头的公共部分
		rev = append(rev, opSame)
		x--
		y--
	}
	for l, r := 0, len(rev)-1; l < r; l, r = l+1, r-1 {
		rev[l], rev[r] = rev[r], rev[l]
	}
	return rev
}

func same(left, right []string, i, j int) domain.DiffLine {
	return domain.DiffLine{Type: "same", LeftNo: ptr(i + 1), RightNo: ptr(j + 1), LeftText: left[i], RightText: right[j]}
}

// appendBlock 输出一段不匹配区：先两两配对成 change，多出的记为 del / add。
func appendBlock(out []domain.DiffLine, left, right []string, dels, adds []int) []domain.DiffLine {
	paired := min(len(dels), len(adds))
	for k := 0; k < paired; k++ {
		out = append(out, domain.DiffLine{Type: "change",
			LeftNo: ptr(dels[k] + 1), RightNo: ptr(adds[k] + 1),
			LeftText: left[dels[k]], RightText: right[adds[k]]})
	}
	for _, i := range dels[paired:] {
		out = append(out, domain.DiffLine{Type: "del", LeftNo: ptr(i + 1), LeftText: left[i]})
	}
	for _, j := range adds[paired:] {
		out = append(out, domain.DiffLine{Type: "add", RightNo: ptr(j + 1), RightText: right[j]})
	}
	return out
}

func ptr(v int) *int { return &v }
