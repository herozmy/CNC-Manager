package domain

// ToolUse 是从 NC 程序里识别到的一次刀具调用。
type ToolUse struct {
	Seq      int    `json:"seq"`
	ToolNo   string `json:"toolNo"`   // 如 T01
	OffsetNo string `json:"offsetNo"` // 如 D01 / H01，识别不到时为空
	LineNo   int    `json:"lineNo"`   // 出现在第几行，便于人工核对
	Raw      string `json:"raw"`      // 该行的原文（已去掉注释）
}

// ParseResult 是从 NC 程序文本里识别出来的信息。
//
// 只识别刀具相关的内容。程序号由用户手工填写，系统不做识别、也不做比对——
// 现场的程序号写法五花八门，机器判断反而不如人一眼看得准，
// 误报还会打断正常的上传流程。
type ParseResult struct {
	Controller string    `json:"controller"` // 推测的数控系统，识别不出为空
	Tools      []ToolUse `json:"tools"`
	LineCount  int       `json:"lineCount"`
	Warnings   []string  `json:"warnings"`
}

// ParseTextInput 是「解析一段 NC 文本」的请求体。
type ParseTextInput struct {
	Content string `json:"content"`
}

// UploadVersionResult 是上传 NC 新版本的响应。
//
// 除了版本信息，还附带从程序正文里识别出来的刀具调用，
// 界面可以据此一键把它们加进刀具补偿表，省掉手工敲一遍。
type UploadVersionResult struct {
	Version
	Parse *ParseResult `json:"parse,omitempty"`
}
