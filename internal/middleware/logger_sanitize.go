package middleware

import "regexp"

// sensitiveFieldRegex 匹配 JSON 中的敏感字段（不区分大小写，兼容 snake_case / camelCase / PascalCase）。
// $1 捕获原始字段名（包括两侧引号），保持日志中的字段名不变，仅将值替换为 "***"。
var sensitiveFieldRegex = regexp.MustCompile(
	`(?i)("(?:new[_-]?password|old[_-]?password|password|passwd|token|access[_-]?token|` +
		`refresh[_-]?token|id[_-]?token|bearer[_-]?token|auth[_-]?token|authorization|` +
		`proxy[_-]?authorization|api[_-]?key|api[_-]?secret|secret[_-]?access[_-]?key|` +
		`access[_-]?key[_-]?id|access[_-]?key|secret[_-]?key|client[_-]?secret|` +
		`private[_-]?key|credentials?|secret)")\s*:\s*"[^"]*"`,
)

// sanitizeBody 清理敏感信息。
func sanitizeBody(body string) string {
	return sensitiveFieldRegex.ReplaceAllString(body, `$1:"***"`)
}
