// Package ncparse 从 NC 程序文本里识别刀具调用。
//
// 用途：上传 NC 后把程序里用到的刀具抓出来，界面可以一键加进刀具补偿表，
// 省掉手工敲一遍，也避免敲错。
//
// 换刀的含义在车床和加工中心上不一样（加工中心的单独 T 只是备刀，
// 车床的 T 命令本身就是换刀），所以识别时按程序里有没有 M06 分流，
// 具体规则见 detectTools。
//
// 刻意**不识别程序号**：程序号由用户手工填写。现场的程序号写法五花八门
// （O1234 / 1234 / O01234 / Siemens 的 %_N_名称_MPF），机器判断不如人一眼看得准，
// 误报还会打断正常的上传流程。
//
// 解析策略偏保守：宁可少识别，不要乱识别。
// 识别不到就返回空，由界面提示用户手工确认，绝不猜。
package ncparse

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"cnccool/internal/domain"
)

const (
	// maxParseBytes 限制解析的文本量，避免超大文件拖慢上传响应。
	maxParseBytes = 2 << 20

	// controllerScanLines 推测数控系统时最多看开头这些行。
	controllerScanLines = 200

	// maxScanLines 扫描刀具时最多看这么多行。
	maxScanLines = 200000

	// maxTools 最多识别多少把刀。
	maxTools = 200
)

var (
	// FANUC 系程序的头：一行就是 O1234，可能前面带 %。
	// 现在只用来推测数控系统，不再当程序号使用。
	reFanucHeaderStrict = regexp.MustCompile(`^[%]?\s*[Oo:]\s*(\d{1,8})\s*(?:\(.*)?$`)
	reFanucHeaderLoose  = regexp.MustCompile(`^[%]?\s*[Oo]\s*(\d{1,8})\b`)
	// Siemens：%_N_名称_MPF / _SPF
	reSiemensHeader = regexp.MustCompile(`^%\s*_N_([A-Za-z0-9_\-]+?)_(MPF|SPF)\s*$`)

	// 刀具与刀补调用。
	//
	// 刻意不加尾部的 \b：T0101M03 这种紧挨着写的很常见，加了 \b 就匹配不到。
	// 前面用「行首或非字母」来限定：既能匹配 G43H01（H 前面是数字），
	// 又不会把注释里或单词内部的字母当成调用。
	reTool    = regexp.MustCompile(`(?i)(?:^|[^A-Za-z])T(\d{1,4})`)
	reOffsetD = regexp.MustCompile(`(?i)(?:^|[^A-Za-z])D(\d{1,4})`)
	reOffsetH = regexp.MustCompile(`(?i)(?:^|[^A-Za-z])H(\d{1,4})`)
	// 换刀指令 M06（也写成 M6 / M006）。后面必须是非数字，免得把 M60 当成 M6。
	reM06 = regexp.MustCompile(`(?i)(?:^|[^A-Za-z])M0*6(?:[^0-9]|$)`)

	// 注释：FANUC 用圆括号，西门子 / 海德汉用分号到行尾
	reParenComment = regexp.MustCompile(`\([^)]*\)`)
	reSemiComment  = regexp.MustCompile(`;.*$`)

	reSiemensMarker = regexp.MustCompile(`(?i)(_N_.*_(MPF|SPF))|^\s*(MSG|TRAORI)\s*\(|^\s*R\d+\s*=`)
)

// Parse 解析 NC 程序文本。
//
// 传入的应当是已经解码成 UTF-8 的文本；编码转换由 ncstore 负责。
func Parse(text string) domain.ParseResult {
	res := domain.ParseResult{
		Tools:    []domain.ToolUse{},
		Warnings: []string{},
	}

	if len(text) > maxParseBytes {
		text = text[:maxParseBytes]
		res.Warnings = append(res.Warnings,
			fmt.Sprintf("程序超过 %d MB，只解析了前面一部分", maxParseBytes>>20))
	}
	if strings.TrimSpace(text) == "" {
		res.Warnings = append(res.Warnings, "程序内容为空")
		return res
	}

	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	res.LineCount = len(lines)
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		res.LineCount--
	}

	res.Controller = detectController(lines)
	detectTools(lines, &res)

	if len(res.Tools) == 0 {
		res.Warnings = append(res.Warnings, "没有识别到刀具调用（T 号）")
	}
	return res
}

// detectController 根据特征推测数控系统，认不准就返回空串。
//
// 这只是给用户一个参考值（界面填数控系统时不用自己想），
// 猜错没有后果，所以可以稍微积极一点；但仍然不做无根据的猜测。
func detectController(lines []string) string {
	for i := 0; i < min(len(lines), controllerScanLines); i++ {
		if reSiemensHeader.MatchString(strings.TrimSpace(lines[i])) {
			return "SIEMENS"
		}
	}
	for i := 0; i < min(len(lines), 2000); i++ {
		if reSiemensMarker.MatchString(lines[i]) {
			return "SIEMENS"
		}
	}
	// FANUC 系（含广数、三菱等）的程序一般以 O+数字 或 % 开头
	for i := 0; i < min(len(lines), controllerScanLines); i++ {
		raw := strings.TrimSpace(lines[i])
		if raw == "" {
			continue
		}
		if reFanucHeaderStrict.MatchString(raw) || reFanucHeaderLoose.MatchString(raw) {
			return "FANUC"
		}
	}
	return ""
}

// stripComments 去掉行内注释。
// 不去掉的话，注释里写的「(T01 D01 - 粗车)」会被当成刀具调用——
// 示例程序的第一行注释里正好就有 T01，这是实测踩过的。
func stripComments(line string) string {
	line = reParenComment.ReplaceAllString(line, " ")
	line = reSemiComment.ReplaceAllString(line, "")
	return line
}

// detectTools 扫描刀具调用。
//
// 换刀的含义在车床和加工中心上是不一样的，必须先分流：
//
//	程序里出现 M06  → 加工中心。**只有和 M06 同行的 T 才算真正换刀**；
//	                  单独的 T 是「备刀」（把刀转到待换位置），不算换到这把刀。
//	                  例：T1 M06 算 T1；单独一行 T2 不算。
//	程序里没有 M06 → 车床。T 命令本身就是换刀（T0101 直接换到 1 号刀），
//	                  所以所有 T 都要算。若也按 M06 规则处理，车床程序会一把刀都识别不出来。
//
// 刀补号的处理：
//   - T0101        → 刀号 01、刀补 D01（FANUC 车床的四位写法）
//   - T01 D01      → 同一行给出刀补
//   - G43 H01 / G41 D01 单独一行 → 补到当前这把刀上（铣床几乎都这么写，
//     所以刀补必须跨行跟随，而且这行没有 T 也可能没有 M06）
//
// 同一把刀只记一行，刀补取第一个识别到的；如果后面又出现了不同的刀补号，
// 记进警告让人工确认，不擅自覆盖也不静默丢弃。
func detectTools(lines []string, res *domain.ParseResult) {
	limit := min(len(lines), maxScanLines)

	useM06 := false
	for i := 0; i < limit; i++ {
		if reM06.MatchString(stripComments(lines[i])) {
			useM06 = true
			break
		}
	}

	index := make(map[string]int) // 归一化刀号 -> res.Tools 下标
	extra := make(map[string][]string)
	var skippedPrep []string
	currentKey := ""

	for i := 0; i < limit; i++ {
		code := stripComments(lines[i])
		if !strings.ContainsAny(code, "TtDdHhMm") {
			continue
		}

		tools := reTool.FindAllStringSubmatch(code, -1)
		offsets := findOffsets(code)

		// 没有 T 的行：可能只是刀补调用（G43 H01 / G41 D01），
		// 补到当前这把刀上——铣床的典型写法。
		if len(tools) == 0 {
			if currentKey != "" && len(offsets) > 0 {
				attachOffsets(res.Tools, index[currentKey], currentKey, offsets, extra)
			}
			continue
		}

		// 加工中心模式下，没有 M06 的 T 是备刀，不是换刀
		if useM06 && !reM06.MatchString(code) {
			for _, m := range tools {
				skippedPrep = appendUnique(skippedPrep, "T"+m[1])
			}
			continue
		}

		for _, m := range tools {
			digits := m[1]
			toolNo, suffixOffset := digits, ""
			// 四位数按 FANUC 车床惯例拆成 2+2：T0101 = 刀号 01、刀补 01
			if len(digits) == 4 {
				toolNo, suffixOffset = digits[:2], "D"+digits[2:]
			}

			key := normalizeToolKey(toolNo)
			idx, ok := index[key]
			if !ok {
				if len(res.Tools) >= maxTools {
					res.Warnings = append(res.Warnings,
						fmt.Sprintf("识别到的刀具超过 %d 把，只保留了前 %d 把", maxTools, maxTools))
					return
				}
				res.Tools = append(res.Tools, domain.ToolUse{
					Seq:    len(res.Tools) + 1,
					ToolNo: "T" + toolNo,
					LineNo: i + 1,
					Raw:    strings.TrimSpace(code),
				})
				idx = len(res.Tools) - 1
				index[key] = idx
			}

			if suffixOffset != "" {
				attachOffsets(res.Tools, idx, key, []string{suffixOffset}, extra)
			} else if len(offsets) > 0 {
				attachOffsets(res.Tools, idx, key, offsets, extra)
			}
			currentKey = key
		}
	}

	// 跳过了备刀就明确说出来，免得用户以为程序里的刀被漏识别了
	if len(skippedPrep) > 0 {
		res.Warnings = append(res.Warnings, fmt.Sprintf(
			"按加工中心规则跳过了 %d 处备刀调用（%s）：它们没有和 M06 同行，不算真正换刀",
			len(skippedPrep), strings.Join(skippedPrep, "、")))
	}

	// 同一把刀出现多个不同刀补号时，明确提示而不是悄悄丢掉
	for key, list := range extra {
		if len(list) == 0 {
			continue
		}
		idx := index[key]
		res.Warnings = append(res.Warnings, fmt.Sprintf(
			"%s 出现了多个刀补号（%s 和 %s），已取第一个，请人工确认",
			res.Tools[idx].ToolNo, res.Tools[idx].OffsetNo, strings.Join(list, "、")))
	}
}

// appendUnique 追加但不重复，保持出现顺序。
func appendUnique(list []string, v string) []string {
	for _, x := range list {
		if x == v {
			return list
		}
	}
	return append(list, v)
}

// findOffsets 取出一行里所有的 D / H 刀补调用，按出现顺序。
func findOffsets(code string) []string {
	type hit struct {
		pos int
		val string
	}
	var hits []hit
	for _, m := range reOffsetD.FindAllStringSubmatchIndex(code, -1) {
		hits = append(hits, hit{m[0], "D" + code[m[2]:m[3]]})
	}
	for _, m := range reOffsetH.FindAllStringSubmatchIndex(code, -1) {
		hits = append(hits, hit{m[0], "H" + code[m[2]:m[3]]})
	}
	// 按在行内出现的先后排序，保证取到的是"第一个"
	for i := 1; i < len(hits); i++ {
		for j := i; j > 0 && hits[j].pos < hits[j-1].pos; j-- {
			hits[j], hits[j-1] = hits[j-1], hits[j]
		}
	}
	out := make([]string, 0, len(hits))
	for _, h := range hits {
		out = append(out, h.val)
	}
	return out
}

// attachOffsets 把刀补号挂到某把刀上。
// 已经有刀补号且不同时，记进 extra 由调用方汇总提示。
func attachOffsets(tools []domain.ToolUse, idx int, key string, offsets []string, extra map[string][]string) {
	for _, off := range offsets {
		if off == "" {
			continue
		}
		if tools[idx].OffsetNo == "" {
			tools[idx].OffsetNo = off
			continue
		}
		if normalizeToolKey(tools[idx].OffsetNo) == normalizeToolKey(off) {
			continue
		}
		dup := false
		for _, e := range extra[key] {
			if normalizeToolKey(e) == normalizeToolKey(off) {
				dup = true
				break
			}
		}
		if !dup {
			extra[key] = append(extra[key], off)
		}
	}
}

// normalizeToolKey 只用于判断「是不是同一个」：T1 和 T01 指同一把刀，
// 但输出时保留程序里的原始写法，免得和正文对不上。
//
// 输入可能带前缀（T01 / D01 / H01），也可能不带（01）——
// 刀号在内部是按纯数字传递的，所以两种都要认。
func normalizeToolKey(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	if s == "" {
		return ""
	}

	prefix := ""
	if s[0] < '0' || s[0] > '9' {
		prefix = s[:1]
		s = s[1:]
	}

	n, err := strconv.Atoi(s)
	if err != nil {
		return prefix + s
	}
	return prefix + strconv.Itoa(n)
}
