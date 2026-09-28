package ncparse

import (
	"strings"
	"testing"
)

// 这些用例都按真实机床程序的写法来，不是编出来的。
// 解析器最容易在「注释里的 T 号」「跨行的刀补」「中文注释」上出错，
// 所以每种情况都单独立一个用例。

func TestFanucTurning(t *testing.T) {
	src := `%
O1234 (DEMO - OUTER TURNING)
(T01 D01 - OD ROUGH  D12.0 r0.8)
(T02 D02 - OD FINISH D6.0  r0.8)
G21 G40 G80 G99
T0101
G97 S1200 M03
G00 X52.0 Z2.0
G71 U2.0 R0.5
M30
%`
	got := Parse(src)

	if got.ProgramNo != "O1234" {
		t.Errorf("程序号 = %q，期望 O1234", got.ProgramNo)
	}
	if got.Controller != "FANUC" {
		t.Errorf("数控系统 = %q，期望 FANUC", got.Controller)
	}
	// 注释里有两行都写了 T01/T02，但那是说明文字，不是调用
	if len(got.Tools) != 1 {
		t.Fatalf("刀具数 = %d，期望 1（注释里的 T 号不能算）: %+v", len(got.Tools), got.Tools)
	}
	if got.Tools[0].ToolNo != "T01" || got.Tools[0].OffsetNo != "D01" {
		t.Errorf("T0101 应拆成 T01 / D01，实际 %s / %s", got.Tools[0].ToolNo, got.Tools[0].OffsetNo)
	}
}

func TestFanucMillingToolOffsetOnOtherLine(t *testing.T) {
	// 铣床的典型写法：T 号单独一行换刀，长度补偿在后面的 G43 行，
	// 半径补偿又在 G41 行。只看同一行会全部漏掉。
	src := `%
O2001 (MILLING DEMO)
G21 G40 G49 G80
T1 M06
G90 G54 G00 X0.0 Y0.0
S3000 M03
G43 H01 Z50.0
G41 D01 X10.0 Y10.0
G40 X0.0 Y0.0
M30
%`
	got := Parse(src)

	if got.ProgramNo != "O2001" {
		t.Errorf("程序号 = %q，期望 O2001", got.ProgramNo)
	}
	if len(got.Tools) != 1 {
		t.Fatalf("刀具数 = %d，期望 1: %+v", len(got.Tools), got.Tools)
	}
	if got.Tools[0].ToolNo != "T1" {
		t.Errorf("刀号 = %q，期望 T1", got.Tools[0].ToolNo)
	}
	// 取第一个出现的刀补号
	if got.Tools[0].OffsetNo != "H01" {
		t.Errorf("刀补号 = %q，期望 H01（G43 那行是第一个）", got.Tools[0].OffsetNo)
	}
	// 同一把刀出现了两个不同刀补号，必须提示而不是悄悄丢掉
	found := false
	for _, w := range got.Warnings {
		if strings.Contains(w, "多个刀补号") && strings.Contains(w, "D01") {
			found = true
		}
	}
	if !found {
		t.Errorf("应提示 T1 有多个刀补号（H01 / D01），实际警告：%v", got.Warnings)
	}
}

func TestSiemens(t *testing.T) {
	src := `%_N_PART_A_MPF
;$PATH=/_N_MPF_DIR
MSG("ROUGH TURN")
T1 D1
G54 G0 X100 Z100
M30`
	got := Parse(src)

	if got.ProgramNo != "PART_A" {
		t.Errorf("程序号 = %q，期望 PART_A", got.ProgramNo)
	}
	if got.Controller != "SIEMENS" {
		t.Errorf("数控系统 = %q，期望 SIEMENS", got.Controller)
	}
	if len(got.Tools) != 1 {
		t.Fatalf("刀具数 = %d，期望 1: %+v", len(got.Tools), got.Tools)
	}
	if got.Tools[0].OffsetNo != "D1" {
		t.Errorf("刀补号 = %q，期望 D1", got.Tools[0].OffsetNo)
	}
}

func TestChineseCommentsIgnored(t *testing.T) {
	src := `%
O3001 (精车外圆 φ60 用 T01 刀)
(T02 精车，T03 切槽)
G21 G40
T0101
M30
%`
	got := Parse(src)

	if got.ProgramNo != "O3001" {
		t.Errorf("程序号 = %q，期望 O3001", got.ProgramNo)
	}
	if len(got.Tools) != 1 || got.Tools[0].ToolNo != "T01" {
		t.Errorf("中文注释里的 T02/T03 不该被识别，实际: %+v", got.Tools)
	}
}

func TestMultipleToolsKeepOrder(t *testing.T) {
	src := `%
O4001
T0303
G0 X50
T0101
G0 X40
T0202
M30
%`
	got := Parse(src)
	if len(got.Tools) != 3 {
		t.Fatalf("刀具数 = %d，期望 3: %+v", len(got.Tools), got.Tools)
	}
	want := []string{"T03", "T01", "T02"}
	for i, w := range want {
		if got.Tools[i].ToolNo != w {
			t.Errorf("第 %d 把刀 = %q，期望 %q（应保持出现顺序）", i+1, got.Tools[i].ToolNo, w)
		}
	}
}

func TestSameToolCalledTwiceCountsOnce(t *testing.T) {
	// T1 和 T01 是同一把刀，只记一行
	src := `%
O5001
T1 M06
G0 X0
T01
M30
%`
	got := Parse(src)
	if len(got.Tools) != 1 {
		t.Errorf("T1 与 T01 应视为同一把刀，实际识别出 %d 行: %+v", len(got.Tools), got.Tools)
	}
}

func TestNoProgramNumber(t *testing.T) {
	src := `G21 G40
T1 M06
M30`
	got := Parse(src)
	if got.ProgramNo != "" {
		t.Errorf("不该识别出程序号，实际 = %q", got.ProgramNo)
	}
	found := false
	for _, w := range got.Warnings {
		if strings.Contains(w, "没有识别到程序号") {
			found = true
		}
	}
	if !found {
		t.Errorf("应给出「没有识别到程序号」的提示，实际：%v", got.Warnings)
	}
}

func TestEmptyContent(t *testing.T) {
	got := Parse("   \r\n  ")
	if len(got.Warnings) == 0 {
		t.Error("空内容应给出提示")
	}
	if len(got.Tools) != 0 {
		t.Errorf("空内容不该识别出刀具，实际: %+v", got.Tools)
	}
}

func TestLineNumbersPointAtRightLine(t *testing.T) {
	src := `%
O6001
G21
T0101
M30
%`
	got := Parse(src)
	if len(got.Tools) != 1 {
		t.Fatalf("刀具数 = %d，期望 1", len(got.Tools))
	}
	// T0101 在第 4 行，行号用于人工核对，必须准
	if got.Tools[0].LineNo != 4 {
		t.Errorf("行号 = %d，期望 4", got.Tools[0].LineNo)
	}
	if !strings.Contains(got.Tools[0].Raw, "T0101") {
		t.Errorf("原文应包含 T0101，实际 = %q", got.Tools[0].Raw)
	}
}
