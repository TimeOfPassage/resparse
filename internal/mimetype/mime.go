// Package mimetype 根据对象 key 的扩展名推断 MIME 类型。
//
// 对应参考项目 src/lib/server/mime.ts。R2 的 HEAD 不一定返回 Content-Type，
// 这里做兜底，保证响应/日志中的「类型」字段始终可用。
package mimetype

import "resparse/internal/keyutil"

var extToMIME = map[string]string{
	// video
	".mp4":  "video/mp4",
	".m4v":  "video/mp4",
	".mov":  "video/quicktime",
	".webm": "video/webm",
	".mkv":  "video/x-matroska",
	".avi":  "video/x-msvideo",
	".flv":  "video/x-flv",
	".ts":   "video/mp2t",
	".m2ts": "video/mp2t",
	".mpg":  "video/mpeg",
	".mpeg": "video/mpeg",
	".wmv":  "video/x-ms-wmv",
	".3gp":  "video/3gpp",
	// audio
	".mp3":  "audio/mpeg",
	".wav":  "audio/wav",
	".aac":  "audio/aac",
	".flac": "audio/flac",
	".ogg":  "audio/ogg",
	".oga":  "audio/ogg",
	".opus": "audio/opus",
	".m4a":  "audio/mp4",
	".wma":  "audio/x-ms-wma",
	// image
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".webp": "image/webp",
	".bmp":  "image/bmp",
	".tif":  "image/tiff",
	".tiff": "image/tiff",
}

// Guess 由 key 推断 MIME 类型，未知返回空串与 false。
func Guess(key string) (string, bool) {
	m, ok := extToMIME[keyutil.ExtensionOf(key)]
	return m, ok
}
