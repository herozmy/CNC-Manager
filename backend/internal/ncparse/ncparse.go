// Package ncparse 从 NC 程序文本里识别程序号、刀具调用等信息。
//
// 为什么需要它：
//   现场最容易犯、又最难自己发现的错误就是「把 O1235 传到了 O1234 下面」。
//   文件名可能被改过、U 盘里可能拿错，但程序正文第一行的程序号才是机床真正要执行的。
//   上传时读一遍正文并和记录里的程序号对一下，这类错误当场就能拦住。
//
// 顺带把程序里调用的刀具抓出来，省掉在刀具补偿表里手工敲一遍。
//
// 解析策略偏保守：宁可少识别，不要乱识别。
// 识别不到就返回空，由调用方提示用户手工确认，绝不猜。
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

	// programNoScanLines 只在文件开头这些行里找程序号。
	// 数控程序的程序号必须写在最前面；往后找只会在注释里翻出假货。
	programNoScanLines = 30

	// maxScanLines 扫描刀具时最多看这么多行。
	maxScanLines = 200000

	// maxTools 最多识别多少把刀。
	maxTools = 200
)

var (
	// FANUC / 广数 / 三菱等：一行就是程序号，可能前面带 %
	reProgramNoStrict = regexp.MustCompile(`^[%]?\s*[Oo:]\s*(\d{1,8})\s*(?:\(.*)?$`)
	// 同上的宽松版：行首有 O1234，后面还跟着别的东西
	reProgramNoLoose = regexp.MustCompile(`^[%]?\s*[Oo]\s*(\d{1,8})\b`)
	// Siemens：%_N_名称_MPF / _SPF
	reSiemensProgramNo = regexp.MustCompile(`^%\s*_N_([A-Za-z0-9_\-]+?)_(MPF|SPF)\s*$`)
	// 西门子 / 海德汉风格的 NAME 声明
	reNameDecl = regexp.MustCompile(`(?i)^;?\s*NAME\s*[:=]\s*([A-Za-z0-9_\-]+)\s*$`)

	// 刀具与刀补调用。
	//
	// 刻意不加尾部的 \b：T0101M03 这种紧挨着写的很常见，加了 \b 就匹配不到。
	// 前面用「行首或非字母」来限定：既能匹配 G43H01（H 前面是数字），
	// 又不会把注释里或单词内部的字母当成调用。
	reTool    = regexp.MustCompile(`(?i)(?:^|[^A-Za-z])T(\d{1,4})`)
	reOffsetD = regexp.MustCompile(`(?i)(?:^|[^A-Za-z])D(\d{1,4})`)
	reOffsetH = regexp.MustCompile(`(?i)(?:^|[^A-Za-z])H(\d{1,4})`)

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

	detectProgramNo(lines, &res)
	detectController(lines, &res)
	detectTools(lines, &res)

	if res.ProgramNo == "" {
		res.Warnings = append(res.Warnings,
			"没有识别到程序号。数控程序一般在第一行写 O 加数字（如 O1234），Siemens 是 %_N_名称_MPF")
	}
	if len(res.Tools) == 0 {
		res.Warnings = append(res.Warnings, "没有识别到刀具调用（T 号）")
	}
	return res
}

// detectProgramNo 在文件开头若干行里找程序号。
func detectProgramNo(lines []string, res *domain.ParseResult) {
	limit := min(len(lines), programNoScanLines)
	for i := 0; i < limit; i++ {
		raw := strings.TrimSpace(lines[i])
		if raw == "" {
			continue
		}

		if m := reSiemensProgramNo.FindStringSubmatch(raw); m != nil {
			res.ProgramNoRaw = raw
			res.ProgramNo = m[1]
			return
		}
		if m := reProgramNoStrict.FindStringSubmatch(raw); m != nil {
			res.ProgramNoRaw = raw
			res.ProgramNo = "O" + m[1]
			return
		}
		if m := reProgramNoLoose.FindStringSubmatch(raw); m != nil {
			res.ProgramNoRaw = raw
			res.ProgramNo = "O" + m[1]
			return
		}
		if m := reNameDecl.FindStringSubmatch(raw); m != nil {
			res.ProgramNoRaw = raw
			res.ProgramNo = m[1]
			return
		}
	}
}

// detectController 根据特征推测数控系统，认不准就留空。
func detectController(lines []string, res *domain.ParseResult) {
	for i := 0; i < min(len(lines), 200); i++ {
		if reSiemensProgramNo.MatchString(strings.TrimSpace(lines[i])) {
			res.Controller = "SIEMENS"
			return
		}
	}
	for i := 0; i < min(len(lines), 2000); i++ {
		if reSiemensMarker.MatchString(lines[i]) {
			res.Controller = "SIEMENS"
			return
		}
	}
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(res.ProgramNoRaw)), "O") {
		res.Controller = "FANUC"
	}
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
// 规则（按现场最常见到最少见的顺序）：
//   - T0101        → 刀号 01、刀补 D01（FANUC 车床的四位写法）
//   - T01 D01      → 同一行给出刀补
//   - T1 M06，后面另起一行 G43 H01 / G41 D01 → 补到前面那把刀上
//     （铣床几乎都这么写，所以必须跨行跟随，不能只看同一行）
//
// 同一把刀只记一行，刀补取第一个识别到的；如果后面又出现了不同的刀补号，
// 记进警告让人工确认，不擅自覆盖也不静默丢弃。
func detectTools(lines []string, res *domain.ParseResult) {
	index := make(map[string]int) // 归一化刀号 -> res.Tools 下标
	extra := make(map[string][]string)
	currentKey := ""

	limit := min(len(lines), maxScanLines)
	for i := 0; i < limit; i++ {
		code := stripComments(lines[i])
		if !strings.ContainsAny(code, "TtDdHh") {
			continue
		}

		tools := reTool.FindAllStringSubmatch(code, -1)
		offsets := findOffsets(code)

		// 这一行没有刀具调用，但可能有刀补调用（G43 H01 / G41 D01），
		// 补到当前这把刀上——铣床的典型写法。
		if len(tools) == 0 {
			if currentKey != "" && len(offsets) > 0 {
				attachOffsets(res.Tools, index[currentKey], currentKey, offsets, extra)
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
