package ncstore

import (
	"bytes"
	"fmt"
	"os"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/simplifiedchinese"
)

// 支持的文本编码。数控程序的注释在国内现场基本就是这两种。
const (
	EncodingUTF8 = "utf-8"
	EncodingGBK  = "gbk"
)

// DetectEncoding 判断一段程序内容是 UTF-8 还是 GBK。
//
// 判断依据：GBK 的中文字节序列几乎不可能是合法的 UTF-8，
// 所以「是合法 UTF-8」就可以认为它是 UTF-8（纯 ASCII 也属于这一类，
// 而纯 ASCII 在两种编码下字节完全相同，按 UTF-8 处理不会有任何问题）。
func DetectEncoding(b []byte) string {
	if utf8.Valid(b) {
		return EncodingUTF8
	}
	return EncodingGBK
}

// NormalizeEncoding 把外部传来的编码名收敛成受支持的两个值之一。
func NormalizeEncoding(name string) string {
	switch name {
	case EncodingGBK, "GBK", "gb2312", "GB2312", "gb18030", "ansi", "ANSI":
		return EncodingGBK
	default:
		return EncodingUTF8
	}
}

// DecodeToUTF8 把文件原始字节按指定编码解码成 UTF-8 文本，供界面显示与编辑。
func DecodeToUTF8(raw []byte, encName string) (string, error) {
	if NormalizeEncoding(encName) == EncodingUTF8 {
		return string(raw), nil
	}
	decoded, err := simplifiedchinese.GBK.NewDecoder().Bytes(raw)
	if err != nil {
		return "", fmt.Errorf("按 GBK 解码程序文件失败: %w", err)
	}
	return string(decoded), nil
}

// EncodeFromUTF8 把界面上编辑好的 UTF-8 文本按目标编码编回字节。
//
// 用 ReplaceUnsupported 包一层：万一内容里有 GBK 表示不了的字符
// （比如从别处粘贴进来的 Ø、emoji 等），替换成占位符而不是直接报错——
// 让用户能存下去，总比整个保存操作失败、改动全丢要好。
// 被替换掉的字符由 UnrepresentableRunes 单独列出来警告用户。
func EncodeFromUTF8(text string, encName string) ([]byte, error) {
	if NormalizeEncoding(encName) == EncodingUTF8 {
		return []byte(text), nil
	}
	enc := encoding.ReplaceUnsupported(simplifiedchinese.GBK.NewEncoder())
	out, err := enc.Bytes([]byte(text))
	if err != nil {
		return nil, fmt.Errorf("按 GBK 编码程序文件失败: %w", err)
	}
	return out, nil
}

// UnrepresentableRunes 找出 text 里无法用目标编码表示的字符。
//
// 为什么必须做这件事：
//
//	GBK 里没有 Ø(U+00D8)，但中文注释里「精车外圆 Ø60」这种写法很常见
//	（GBK 能表示的是希腊字母 φ/Φ）。用户粘贴进去一个 Ø 然后保存，
//	如果不提示，程序里的这个字符就被静默替换掉了，肉眼看不出区别，
//	到了机床上可能就是错的。
//
// 只检查非 ASCII 字符并缓存判定结果，所以对几 MB 的文件也很快。
func UnrepresentableRunes(text, encName string) []string {
	if NormalizeEncoding(encName) == EncodingUTF8 {
		return nil
	}
	enc := simplifiedchinese.GBK.NewEncoder()

	checked := make(map[rune]bool, 64)
	var bad []string
	for _, r := range text {
		if r < 0x80 {
			continue // ASCII 在任何目标编码下都能表示
		}
		ok, seen := checked[r]
		if !seen {
			_, err := enc.Bytes([]byte(string(r)))
			ok = err == nil
			checked[r] = ok
			if !ok {
				bad = append(bad, string(r))
			}
		}
	}
	return bad
}

// SaveText 把 UTF-8 文本按目标编码写入文件库。
// Unrepresentable 会列出目标编码存不下、已被替换的字符（通常是空）。
func (s *Store) SaveText(text, encName, originalName string) (*Saved, error) {
	// 先算好哪些字符存不下，再编码——编码之后就分辨不出来了
	unrepresentable := UnrepresentableRunes(text, encName)

	raw, err := EncodeFromUTF8(text, encName)
	if err != nil {
		return nil, err
	}
	saved, err := s.Save(bytes.NewReader(raw), originalName)
	if err != nil {
		return nil, err
	}
	// 内容是我们自己编码出来的，编码名以调用方给的为准
	saved.Encoding = NormalizeEncoding(encName)
	saved.Unrepresentable = unrepresentable
	return saved, nil
}

// ReadAll 读出文件全部内容（用于程序查看/编辑，文件本身有大小上限保护）。
func (s *Store) ReadAll(relPath string) ([]byte, error) {
	abs, err := s.Abs(relPath)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("读取程序文件失败: %w", err)
	}
	plain, _, err := s.decrypt(raw)
	if err != nil {
		return nil, err
	}
	if int64(len(plain)) > s.maxBytes {
		return nil, fmt.Errorf("%w: 文件超过 %d MB，无法在界面里打开", ErrTooLarge, s.maxBytes/1024/1024)
	}
	return plain, nil
}
