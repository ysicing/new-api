package setting

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const SensitiveWordContactMessageMaxRunes = 500

var CheckSensitiveEnabled = true
var CheckSensitiveOnPromptEnabled = true

//var CheckSensitiveOnCompletionEnabled = true

// StopOnSensitiveEnabled 如果检测到敏感词，是否立刻停止生成，否则替换敏感词
var StopOnSensitiveEnabled = true

// StreamCacheQueueLength 流模式缓存队列长度，0表示无缓存
var StreamCacheQueueLength = 0

// SensitiveWordContactMessage 是敏感词误判处理的完整提示文本；空值表示不追加联系说明。
var SensitiveWordContactMessage string

// SensitiveWords 敏感词
// var SensitiveWords []string
var SensitiveWords = []string{
	"test_sensitive",
}

func SensitiveWordsToString() string {
	return strings.Join(SensitiveWords, "\n")
}

func SensitiveWordsFromString(s string) {
	SensitiveWords = []string{}
	sw := strings.Split(s, "\n")
	for _, w := range sw {
		w = strings.TrimSpace(w)
		if w != "" {
			SensitiveWords = append(SensitiveWords, w)
		}
	}
}

func NormalizeSensitiveWordContactMessage(message string) string {
	return strings.TrimSpace(message)
}

func ValidateSensitiveWordContactMessage(message string) error {
	message = NormalizeSensitiveWordContactMessage(message)
	if utf8.RuneCountInString(message) > SensitiveWordContactMessageMaxRunes {
		return fmt.Errorf("sensitive word contact message must be %d characters or fewer", SensitiveWordContactMessageMaxRunes)
	}
	return nil
}

func SetSensitiveWordContactMessage(message string) error {
	message = NormalizeSensitiveWordContactMessage(message)
	if err := ValidateSensitiveWordContactMessage(message); err != nil {
		return err
	}
	SensitiveWordContactMessage = message
	return nil
}

func ShouldCheckPromptSensitive() bool {
	return CheckSensitiveEnabled && CheckSensitiveOnPromptEnabled
}

//func ShouldCheckCompletionSensitive() bool {
//	return CheckSensitiveEnabled && CheckSensitiveOnCompletionEnabled
//}
