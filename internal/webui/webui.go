// Package webui 通过 go:embed 提供内嵌的演示页。
//
// 对应参考项目的 Svelte 演示页 src/routes/+page.svelte，
// 这里用单文件静态页面实现，保持零前端构建步骤。
package webui

import _ "embed"

//go:embed index.html
var indexHTML []byte

// Index 返回演示页 HTML。
func Index() []byte { return indexHTML }
