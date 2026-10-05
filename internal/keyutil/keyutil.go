// Package keyutil 提供对象 key 的规范化与扩展名推断。
//
// 对应参考项目中 r2.ts 的 normalizeKey 与 temp.ts 的 extensionOf，
// 抽取为独立包以便 r2 / tempfile / metadata 复用。
package keyutil

import (
	"path"
	"regexp"
	"strings"
)

var extPattern = regexp.MustCompile(`(?i)^\.[a-z0-9]{1,8}$`)

// NormalizeKey 去除首尾空白与前导斜杠。
func NormalizeKey(key string) string {
	return strings.TrimLeft(strings.TrimSpace(key), "/")
}

// ExtensionOf 从 key 中提取安全的扩展名（含点，小写）；无则返回空串。
func ExtensionOf(key string) string {
	base := path.Base(key)
	dot := strings.LastIndex(base, ".")
	if dot <= 0 {
		return ""
	}
	ext := base[dot:]
	if !extPattern.MatchString(ext) {
		return ""
	}
	return strings.ToLower(ext)
}
