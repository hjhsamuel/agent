package tool

import (
	"fmt"
	"unicode/utf8"
)

const (
	MaxToolRunes         = 32_000 // 支持工具输出的最大字符数
	HeadRatio    float64 = 0.25   // 截断后，头部内容占比
)

type TruncateResult struct {
	Content       string
	OriginalRunes int
	RetainedRunes int
	OmittedRunes  int
}

func Truncate(content string) *TruncateResult {
	originalRunes := utf8.RuneCountInString(content)

	// 不需要截断。
	if originalRunes <= MaxToolRunes {
		return &TruncateResult{
			Content:       content,
			OriginalRunes: originalRunes,
			RetainedRunes: originalRunes,
		}
	}

	runes := []rune(content)

	headSize := int(float64(MaxToolRunes) * HeadRatio)
	tailSize := MaxToolRunes - headSize

	// 防御性处理。
	if headSize > len(runes) {
		headSize = len(runes)
	}
	if tailSize > len(runes)-headSize {
		tailSize = len(runes) - headSize
	}

	omitted := originalRunes - headSize - tailSize

	marker := fmt.Sprintf(
		"\n\n... [TRUNCATED: %d runes omitted] ...\n\n",
		omitted,
	)

	result := make([]rune, 0, headSize+len([]rune(marker))+tailSize)

	result = append(result, runes[:headSize]...)
	result = append(result, []rune(marker)...)
	result = append(result, runes[len(runes)-tailSize:]...)

	return &TruncateResult{
		Content:       string(result),
		OriginalRunes: originalRunes,
		RetainedRunes: headSize + tailSize,
		OmittedRunes:  omitted,
	}
}
