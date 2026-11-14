package domain

import (
	"errors"
	"math"
	"strings"
	"testing"
)

func TestMilliConversion(t *testing.T) {
	cases := map[float64]int64{0: 0, 12.5: 12500, 0.0004: 0, 0.0005: 1, -1.2345: -1235, math.NaN(): 0, math.Inf(1): 0}
	for in, want := range cases {
		if got := FloatToMilli(in); got != want {
			t.Errorf("FloatToMilli(%v) = %d，期望 %d", in, got, want)
		}
	}
	if MilliToFloat(12500) != 12.5 {
		t.Error("MilliToFloat 错误")
	}
}

func TestDrawingInputNormalize(t *testing.T) {
	in := DrawingInput{DrawingNo: "  D-1 ", Name: " 壳体 "}
	if err := in.Normalize(); err != nil || in.DrawingNo != "D-1" || in.Name != "壳体" {
		t.Fatalf("正常输入失败: %+v %v", in, err)
	}
	for _, bad := range []DrawingInput{{DrawingNo: "  "}, {DrawingNo: "x", Name: strings.Repeat("a", 101)}, {DrawingNo: "x", Remark: strings.Repeat("a", 2001)}} {
		if err := bad.Normalize(); !errors.Is(err, ErrInvalid) {
			t.Errorf("%+v 应为参数错误，得到 %v", bad, err)
		}
	}
}

func TestOperationInputNormalize(t *testing.T) {
	zero := int64(0)
	in := OperationInput{OpNo: 10, MachineID: &zero, ZHeight: 25.5}
	if err := in.Normalize(); err != nil || in.MachineID != nil {
		t.Fatalf("机台 0 应视为未选择: %+v %v", in, err)
	}
	for _, bad := range []OperationInput{{OpNo: 0}, {OpNo: 10000}, {OpNo: 10, ZHeight: -100001}, {OpNo: 10, ZHeight: math.NaN()}} {
		if err := bad.Normalize(); !errors.Is(err, ErrInvalid) {
			t.Errorf("%+v 应为参数错误", bad)
		}
	}
}

func TestOperationCoordinateBounds(t *testing.T) {
	for _, axis := range []string{"X", "Y", "Z"} {
		for _, value := range []float64{-100000, -25.501, 0, 25.501, 100000, -100000.001, 100000.001, math.Inf(-1), math.Inf(1), math.NaN()} {
			in := OperationInput{OpNo: 10}
			switch axis {
			case "X":
				in.XCoordinate = &value
			case "Y":
				in.YCoordinate = &value
			case "Z":
				in.ZHeight = value
			}
			err := in.Normalize()
			invalid := math.Abs(value) > 100000 || math.IsNaN(value) || math.IsInf(value, 0)
			if invalid && (!errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), axis+" 坐标")) {
				t.Errorf("%s=%v 应提示该轴错误: %v", axis, value, err)
			} else if !invalid && err != nil {
				t.Errorf("%s=%v 应合法: %v", axis, value, err)
			}
		}
	}
}

func TestProgramInputNormalize(t *testing.T) {
	in := ProgramInput{ProgramNo: " O1234 "}
	if err := in.Normalize(); err != nil || in.ProgramNo != "O1234" {
		t.Fatal(err)
	}
	if err := (&ProgramInput{}).Normalize(); !errors.Is(err, ErrInvalid) {
		t.Error("空程序号应报错")
	}
}
