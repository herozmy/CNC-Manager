// Package update 负责两件事：问 GitHub 上有没有新版本，以及把离线包装上去。
//
// 为什么放在后端而不是前端：
//   - 前端直连 api.github.com 会撞上跨域；
//   - 后端可以做缓存，避免每次刷新页面都去问一遍（GitHub 对未认证请求限流很紧）；
//   - 车间网络可能要走代理，后端处理比浏览器可控。
package update

import (
	"strconv"
	"strings"
)

// ParseVersion 把 "v0.03"、"1.2.3"、"v1.2.3-rc1" 这样的版本号拆成数字段。
//
// 解析不了就返回 ok=false，调用方据此放弃比较。
// 宁可漏报「有新版本」，也不能因为版本号写法怪就误报——那会让用户白白折腾一趟。
func ParseVersion(s string) ([]int, bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "v")
	s = strings.TrimPrefix(s, "V")

	// 去掉预发布/构建后缀：v1.2.3-rc1 -> 1.2.3
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	if s == "" {
		return nil, false
	}

	parts := strings.Split(s, ".")
	nums := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || n < 0 {
			return nil, false
		}
		nums = append(nums, n)
	}
	return nums, true
}

// CompareVersion 比较两个版本号：a<b 返回 -1，a==b 返回 0，a>b 返回 1。
//
// 段数不一样时（v1.2 与 v1.2.0）按补零处理，两者相等。
// 任一边解析不了就返回 0（当作相同），这样界面上最多是不提示，不会误提示。
func CompareVersion(a, b string) int {
	na, oka := ParseVersion(a)
	nb, okb := ParseVersion(b)
	if !oka || !okb {
		return 0
	}

	n := len(na)
	if len(nb) > n {
		n = len(nb)
	}
	for i := 0; i < n; i++ {
		var x, y int
		if i < len(na) {
			x = na[i]
		}
		if i < len(nb) {
			y = nb[i]
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}
