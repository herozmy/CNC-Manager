package domain

// ToolUse 是从 NC 程序里识别到的一次刀具调用。
type ToolUse struct {
	Seq      int    `json:"seq"`
	ToolNo   string `json:"toolNo"`   // 如 T01
	OffsetNo string `json:"offsetNo"` // 如 D01 / H01，识别不到时为空
	LineNo   int    `json:"lineNo"`   // 出现在第几行，便于人工核对
	Raw      string `json:"raw"`      // 该行原文（已去掉注释）
}

// ParseResult 是从 NC 文本识别出的信息。只识别刀具与数控系统，
// 程序号由用户手工填写——现场写法五花八门，机器判断不如人准。
type ParseResult struct {
	Controller string    `json:"controller"`
	Tools      []ToolUse `json:"tools"`
	LineCount  int       `json:"lineCount"`
	Warnings   []string  `json:"warnings"`
}

// DiffLine 是两个版本之间的一行差异。
type DiffLine struct {
	Type      string `json:"type"` // same / add / del / change
	LeftNo    *int   `json:"leftNo"`
	RightNo   *int   `json:"rightNo"`
	LeftText  string `json:"leftText"`
	RightText string `json:"rightText"`
}

// DiffResult 是版本对比结果。
type DiffResult struct {
	TotalLines     int        `json:"totalLines"`
	TotalChanges   int        `json:"totalChanges"`
	LeftVersionNo  int        `json:"leftVersionNo"`
	RightVersionNo int        `json:"rightVersionNo"`
	Identical      bool       `json:"identical"`
	Truncated      bool       `json:"truncated"` // 改动过大，中间部分按整块替换处理
	Lines          []DiffLine `json:"lines"`
}
