package domain

import (
	"math"
	"strings"
	"testing"
)

func TestToolTableValidation(t *testing.T) {
	for name, change := range map[string]func(*ToolTableInput){
		"missing revision":  func(in *ToolTableInput) { in.ExpectedRevision = "" },
		"null items":        func(in *ToolTableInput) { in.Items = nil },
		"too many rows":     func(in *ToolTableInput) { in.Items = make([]ProgramToolInput, 501) },
		"empty tool":        func(in *ToolTableInput) { in.Items[0].ToolNo = " " },
		"negative diameter": func(in *ToolTableInput) { in.Items[0].ToolDia = -1 },
		"overflow":          func(in *ToolTableInput) { in.Items[0].CompAmount = 1e20 },
		"nan":               func(in *ToolTableInput) { in.Items[0].Feed = math.NaN() },
		"infinite":          func(in *ToolTableInput) { in.Items[0].CutDepth = math.Inf(1) },
		"speed mode":        func(in *ToolTableInput) { in.Items[0].SpeedMode = 2 },
		"feed mode":         func(in *ToolTableInput) { in.Items[0].FeedMode = -1 },
		"coolant":           func(in *ToolTableInput) { in.Items[0].Coolant = 4 },
		"speed":             func(in *ToolTableInput) { in.Items[0].SpindleSpeed = -1 },
		"long text":         func(in *ToolTableInput) { in.Items[0].Remark = strings.Repeat("a", 2001) },
		"blank param":       func(in *ToolTableInput) { in.Items[0].CustomParams = []ToolCustomParam{{Name: " "}} },
		"duplicate param": func(in *ToolTableInput) {
			in.Items[0].CustomParams = []ToolCustomParam{{Name: "Brand"}, {Name: " brand "}}
		},
	} {
		t.Run(name, func(t *testing.T) {
			in := ToolTableInput{ExpectedRevision: strings.Repeat("a", 64), Items: []ProgramToolInput{{ToolNo: "T01"}}}
			change(&in)
			if in.Normalize() == nil {
				t.Fatal("应拒绝")
			}
		})
	}
	in := ToolTableInput{ExpectedRevision: strings.Repeat("a", 64), Items: []ProgramToolInput{{Seq: 99, ToolNo: " T01 ", CompAmount: -.001}, {Seq: 99, ToolNo: "T02"}}}
	if err := in.Normalize(); err != nil {
		t.Fatal(err)
	}
	if in.Items[0].Seq != 1 || in.Items[1].Seq != 2 || in.Items[0].ToolNo != "T01" || in.Items[0].CustomParams == nil {
		t.Fatal("标准化失败")
	}
}
