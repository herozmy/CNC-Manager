package httpapi

import (
	"strings"
	"testing"

	"cnccool/internal/domain"
)

// tool 造一行最小可用的刀具，只覆盖测试关心的字段。
func tool(seq int, tweak func(*domain.ProgramToolInput)) domain.ProgramToolInput {
	it := domain.ProgramToolInput{
		Seq: seq, ToolNo: "T01", OffsetNo: "D01",
		ToolDia: 12, CornerRadius: 0.8, CompAmount: 0,
		SpeedMode: 0, FeedMode: 0, Coolant: 1,
	}
	if tweak != nil {
		tweak(&it)
	}
	return it
}

// TestValidateToolsAllowsNegativeComp 是这一组里最要紧的一条。
//
// 刀补记的是「实际值相对理论值的偏差」：磨损修下去、半径补偿取反，
// 都会是负数。早先这里一刀切禁掉了负数，现场根本填不进去。
func TestValidateToolsAllowsNegativeComp(t *testing.T) {
	cases := []struct {
		name string
		comp float64
	}{
		{"磨损修下去", -0.05},
		{"半径补偿取反", -0.4},
		{"整毫米级", -2},
		{"刚好在下限", -10000},
		{"零", 0},
		{"正数", 0.8},
		{"刚好在上限", 10000},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			items := []domain.ProgramToolInput{tool(1, func(it *domain.ProgramToolInput) {
				it.CompAmount = c.comp
			})}
			if err := validateProgramTools(items); err != nil {
				t.Errorf("补偿量 %g 应当被接受，却被拒绝：%v", c.comp, err)
			}
		})
	}
}

func TestValidateToolsRejectsOutOfRangeComp(t *testing.T) {
	for _, comp := range []float64{10000.001, -10000.001, 99999, -99999} {
		items := []domain.ProgramToolInput{tool(1, func(it *domain.ProgramToolInput) {
			it.CompAmount = comp
		})}
		err := validateProgramTools(items)
		if err == nil {
			t.Errorf("补偿量 %g 超出范围，应当被拒绝", comp)
			continue
		}
		if !strings.Contains(err.Error(), "补偿量") {
			t.Errorf("错误信息应当点明是补偿量：%v", err)
		}
	}
}

// TestValidateToolsRejectsNegativeWhereMeaningless 验证该拦的负数还是拦着。
//
// 直径、刀尖圆弧、进给、切深没有负数的物理意义，
// 放宽补偿量不能顺手把这几项也放开。
func TestValidateToolsRejectsNegativeWhereMeaningless(t *testing.T) {
	cases := []struct {
		name  string
		tweak func(*domain.ProgramToolInput)
	}{
		{"直径为负", func(it *domain.ProgramToolInput) { it.ToolDia = -1 }},
		{"刀尖圆弧为负", func(it *domain.ProgramToolInput) { it.CornerRadius = -0.1 }},
		{"进给为负", func(it *domain.ProgramToolInput) { it.Feed = -1 }},
		{"切深为负", func(it *domain.ProgramToolInput) { it.CutDepth = -0.5 }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			items := []domain.ProgramToolInput{tool(1, c.tweak)}
			if err := validateProgramTools(items); err == nil {
				t.Errorf("%s 应当被拒绝", c.name)
			}
		})
	}
}

func TestValidateToolsRejectsDuplicateSeq(t *testing.T) {
	items := []domain.ProgramToolInput{tool(1, nil), tool(1, func(it *domain.ProgramToolInput) {
		it.ToolNo = "T02"
	})}
	if err := validateProgramTools(items); err == nil {
		t.Errorf("行号重复应当被拒绝")
	}
}

func TestValidateToolsRejectsBadModes(t *testing.T) {
	items := []domain.ProgramToolInput{tool(1, func(it *domain.ProgramToolInput) { it.SpeedMode = 2 })}
	if err := validateProgramTools(items); err == nil {
		t.Errorf("速度模式 2 应当被拒绝")
	}

	items = []domain.ProgramToolInput{tool(1, func(it *domain.ProgramToolInput) { it.FeedMode = 3 })}
	if err := validateProgramTools(items); err == nil {
		t.Errorf("进给模式 3 应当被拒绝")
	}

	items = []domain.ProgramToolInput{tool(1, func(it *domain.ProgramToolInput) { it.Coolant = 4 })}
	if err := validateProgramTools(items); err == nil {
		t.Errorf("冷却方式 4 应当被拒绝")
	}
}

func TestValidateToolsAcceptsEmptyTable(t *testing.T) {
	if err := validateProgramTools(nil); err != nil {
		t.Errorf("空刀具表应当被接受（等于清空），实际：%v", err)
	}
}
