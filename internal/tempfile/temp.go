// Package tempfile 提供临时文件下载与清理。
//
// 对应参考项目 src/lib/server/temp.ts。ffprobe 直接读取本地文件最稳妥
// （可随机 seek），因此从 R2 拉流时先落盘到临时目录，解析完成后删除。
// 下载过程中通过 io.LimitedReader 做体积上限保护。
package tempfile

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"resparse/internal/apperr"
)

// Download 描述一次落盘结果。
type Download struct {
	Dir      string
	FilePath string
	Bytes    int64
}

// DownloadToFile 将 body 写入临时目录下的文件。
//
// 读取超过 maxBytes 时中断并返回 file_too_large(413)。
// suffix 为文件名后缀（如 ".mp4"），有助于少数容器格式探测。
func DownloadToFile(body io.Reader, maxBytes int64, prefix, suffix string) (*Download, error) {
	dir, err := os.MkdirTemp("", prefix)
	if err != nil {
		return nil, err
	}
	filePath := filepath.Join(dir, "input"+suffix)

	f, err := os.Create(filePath)
	if err != nil {
		RemoveDir(dir)
		return nil, err
	}

	lr := &io.LimitedReader{R: body, N: maxBytes + 1}
	written, copyErr := io.Copy(f, lr)
	closeErr := f.Close()
	if copyErr != nil {
		RemoveDir(dir)
		return nil, copyErr
	}
	if closeErr != nil {
		RemoveDir(dir)
		return nil, closeErr
	}
	if written > maxBytes {
		RemoveDir(dir)
		return nil, apperr.New(
			"file_too_large",
			fmt.Sprintf("文件超过允许解析的最大体积（%d 字节）", maxBytes),
			413, nil,
		)
	}

	return &Download{Dir: dir, FilePath: filePath, Bytes: written}, nil
}

// RemoveDir 递归删除临时目录，忽略不存在等错误。
func RemoveDir(dir string) {
	_ = os.RemoveAll(dir)
}
