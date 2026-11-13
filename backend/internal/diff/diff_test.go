package diff

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

func lines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}

// rebuild 由差异还原出左右两侧，用来验证差异是完整且自洽的。
func rebuild(r Result) (left, right []string) {
	for _, l := range r.Lines {
		switch l.Type {
		case "same":
			left, right = append(left, l.LeftText), append(right, l.RightText)
		case "change":
			left, right = append(left, l.LeftText), append(right, l.RightText)
		case "del":
			left = append(left, l.LeftText)
		case "add":
			right = append(right, l.RightText)
		}
	}
	return
}

func counts(r Result) (same, add, del, change int) {
	for _, l := range r.Lines {
		switch l.Type {
		case "same":
			same++
		case "add":
			add++
		case "del":
			del++
		case "change":
			change++
		}
	}
	return
}

func TestBasicCases(t *testing.T) {
	cases := []struct {
		name, l, r             string
		same, add, del, change int
	}{
		{"完全相同", "a,b,c", "a,b,c", 3, 0, 0, 0},
		{"两侧为空", "", "", 0, 0, 0, 0},
		{"左空", "", "a,b", 0, 2, 0, 0},
		{"右空", "a,b", "", 0, 0, 2, 0},
		{"中间改一行", "a,b,c", "a,X,c", 2, 0, 0, 1},
		{"末尾新增", "a,b", "a,b,c", 2, 1, 0, 0},
		{"开头删除", "a,b,c", "b,c", 2, 0, 1, 0},
		{"中间插入两行", "a,d", "a,b,c,d", 2, 2, 0, 0},
		{"整体不同", "a,b", "x,y,z", 0, 1, 0, 2},
		{"重复行", "a,a,a", "a,a", 2, 0, 1, 0},
	}
	for _, c := range cases {
		r := Compare(lines(c.l), lines(c.r))
		s, a, d, ch := counts(r)
		if s != c.same || a != c.add || d != c.del || ch != c.change {
			t.Errorf("%s: same/add/del/change = %d/%d/%d/%d，期望 %d/%d/%d/%d", c.name, s, a, d, ch, c.same, c.add, c.del, c.change)
		}
		if r.Truncated {
			t.Errorf("%s: 小改动不应降级", c.name)
		}
	}
}

func TestLineNumbers(t *testing.T) {
	r := Compare(lines("a,b,c,d"), lines("a,X,c,d,e"))
	want := []struct {
		typ    string
		ln, rn int // 0 表示 nil
	}{{"same", 1, 1}, {"change", 2, 2}, {"same", 3, 3}, {"same", 4, 4}, {"add", 0, 5}}
	if len(r.Lines) != len(want) {
		t.Fatalf("行数 %d，期望 %d", len(r.Lines), len(want))
	}
	for i, w := range want {
		l := r.Lines[i]
		gotL, gotR := 0, 0
		if l.LeftNo != nil {
			gotL = *l.LeftNo
		}
		if l.RightNo != nil {
			gotR = *l.RightNo
		}
		if l.Type != w.typ || gotL != w.ln || gotR != w.rn {
			t.Errorf("第 %d 行 = %s %d/%d，期望 %s %d/%d", i, l.Type, gotL, gotR, w.typ, w.ln, w.rn)
		}
	}
}

// 旧版"局部重同步"在窗口外重排时会给出非最小差异；Myers 必须找到最少的改动。
func TestFindsMinimalDiffForFarMovedBlock(t *testing.T) {
	var left, right []string
	for i := 0; i < 200; i++ {
		left = append(left, fmt.Sprintf("L%d", i))
	}
	right = append(right, left[100:]...) // 把前 100 行整体挪到最后
	right = append(right, left[:100]...)
	r := Compare(left, right)
	_, add, del, change := counts(r)
	if edits := add + del + 2*change; edits > 200 {
		t.Fatalf("编辑量 %d 超过最小值 100+100", edits)
	}
	gl, gr := rebuild(r)
	if strings.Join(gl, ",") != strings.Join(left, ",") || strings.Join(gr, ",") != strings.Join(right, ",") {
		t.Fatal("差异无法还原原文")
	}
}

// 随机文本：差异必须能还原两侧，且 same 行数等于最长公共子序列。
func TestRandomRoundTripAndOptimality(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	for iter := 0; iter < 300; iter++ {
		n, m := rng.Intn(25), rng.Intn(25)
		alpha := 1 + rng.Intn(5)
		gen := func(k int) []string {
			out := make([]string, k)
			for i := range out {
				out[i] = fmt.Sprintf("%c", 'a'+rng.Intn(alpha))
			}
			return out
		}
		left, right := gen(n), gen(m)
		r := Compare(left, right)
		gl, gr := rebuild(r)
		if strings.Join(gl, ",") != strings.Join(left, ",") || strings.Join(gr, ",") != strings.Join(right, ",") {
			t.Fatalf("还原失败: %v %v", left, right)
		}
		same, _, _, _ := counts(r)
		// change 行左右文本不同，所以公共行 = same；用 DP 求 LCS 对照
		if lcs := lcsLen(left, right); same != lcs {
			t.Fatalf("same=%d，LCS=%d: %v %v", same, lcs, left, right)
		}
	}
}

func lcsLen(a, b []string) int {
	dp := make([][]int, len(a)+1)
	for i := range dp {
		dp[i] = make([]int, len(b)+1)
	}
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else {
				dp[i][j] = max(dp[i-1][j], dp[i][j-1])
			}
		}
	}
	return dp[len(a)][len(b)]
}

func TestHugeChangeDegradesInsteadOfBlowingUp(t *testing.T) {
	var left, right []string
	for i := 0; i < 5000; i++ {
		left = append(left, fmt.Sprintf("L%d", i))
		right = append(right, fmt.Sprintf("R%d", i))
	}
	r := Compare(left, right)
	if !r.Truncated {
		t.Fatal("超大改动应降级为整块替换")
	}
	gl, gr := rebuild(r)
	if len(gl) != 5000 || len(gr) != 5000 {
		t.Fatal("降级后仍应保留全部内容")
	}
}

func TestLargeFileWithSmallEditIsFast(t *testing.T) {
	var left []string
	for i := 0; i < 100000; i++ {
		left = append(left, fmt.Sprintf("N%d G01 X%d", i, i))
	}
	right := append([]string(nil), left...)
	right[50000] = "CHANGED"
	r := Compare(left, right)
	_, a, d, c := counts(r)
	if r.Truncated || a != 0 || d != 0 || c != 1 {
		t.Fatalf("10 万行改 1 行应得到 1 处 change: %d/%d/%d truncated=%v", a, d, c, r.Truncated)
	}
}

func TestIdentical(t *testing.T) {
	if !Identical(lines("a,b"), lines("a,b")) || Identical(lines("a"), lines("a,b")) || Identical(lines("a,b"), lines("a,c")) {
		t.Fatal("Identical 判断错误")
	}
	if !Identical(nil, nil) {
		t.Fatal("两个空应相同")
	}
}
