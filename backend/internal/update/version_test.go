package update

import "testing"

func TestParseVersion(t *testing.T) {
	cases := []struct {
		in   string
		want []int
		ok   bool
	}{
		{"v0.03", []int{0, 3}, true},
		{"v0.3", []int{0, 3}, true},
		{"0.03", []int{0, 3}, true},
		{"  v1.2.3  ", []int{1, 2, 3}, true},
		{"V2.0", []int{2, 0}, true},
		{"v1.2.3-rc1", []int{1, 2, 3}, true},
		{"v1.2.3+build5", []int{1, 2, 3}, true},
		{"v1", []int{1}, true},
		{"", nil, false},
		{"v", nil, false},
		{"v1.x", nil, false},
		{"latest", nil, false},
		{"-rc1", nil, false},
	}
	for _, c := range cases {
		got, ok := ParseVersion(c.in)
		if ok != c.ok {
			t.Errorf("ParseVersion(%q) ok = %v，期望 %v", c.in, ok, c.ok)
			continue
		}
		if !ok {
			continue
		}
		if len(got) != len(c.want) {
			t.Errorf("ParseVersion(%q) = %v，期望 %v", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("ParseVersion(%q) = %v，期望 %v", c.in, got, c.want)
				break
			}
		}
	}
}

func TestCompareVersion(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		// 这一组是最容易写错的地方：按字符串比的话 v0.10 < v0.9
		{"v0.03", "v0.04", -1},
		{"v0.04", "v0.03", 1},
		{"v0.03", "v0.03", 0},
		{"v0.9", "v0.10", -1},
		{"v0.10", "v0.9", 1},
		{"v1.0", "v0.99.99", 1},
		// 段数不同按补零算
		{"v1.2", "v1.2.0", 0},
		{"v1.2.1", "v1.2", 1},
		{"v2", "v1.9.9", 1},
		{"v0.03", "0.03", 0},
		{"v1.2.3-rc1", "v1.2.3", 0},
		// 解析不了的一律当作相同：宁可漏报，也不误报
		{"unknown", "v1.0", 0},
		{"v1.0", "unknown", 0},
		{"", "v1.0", 0},
	}
	for _, c := range cases {
		if got := CompareVersion(c.a, c.b); got != c.want {
			t.Errorf("CompareVersion(%q, %q) = %d，期望 %d", c.a, c.b, got, c.want)
		}
	}
}
