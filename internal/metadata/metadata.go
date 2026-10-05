// Package metadata 编排「R2 取流 -> 落临时文件 -> ffprobe -> 归一化输出」。
//
// 对应参考项目 src/lib/server/metadata.ts。各阶段耗时会被记录到日志与响应的
// `timings` 字段中，便于排查问题。
package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"resparse/internal/apperr"
	"resparse/internal/config"
	"resparse/internal/ffprobe"
	"resparse/internal/keyutil"
	"resparse/internal/logging"
	"resparse/internal/mimetype"
	"resparse/internal/r2"
	"resparse/internal/tempfile"
)

// Service 元数据解析服务。
type Service struct {
	cfg    *config.Config
	runner *ffprobe.Runner

	mu     sync.Mutex
	client *r2.Client
}

// NewService 构造 Service。
func NewService(cfg *config.Config, runner *ffprobe.Runner) *Service {
	return &Service{cfg: cfg, runner: runner}
}

// getClient 惰性创建并复用 R2 客户端；配置缺失时返回 config_missing。
func (s *Service) getClient() (*r2.Client, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.client != nil {
		return s.client, nil
	}
	r2cfg, err := s.cfg.ResolveR2()
	if err != nil {
		return nil, err
	}
	client, err := r2.NewClient(r2cfg)
	if err != nil {
		return nil, err
	}
	s.client = client
	return client, nil
}

// Parse 从 R2 指定 key 读取文件并同步解析元数据。
//
// 返回类型为 any：媒体文件返回 MediaMetadata；其他文件返回 SourceOnly。
func (s *Service) Parse(ctx context.Context, rawKey string) (any, error) {
	started := time.Now()

	key := keyutil.NormalizeKey(rawKey)
	if key == "" {
		return nil, apperr.New("invalid_key", "key 不能为空", 400, nil)
	}

	client, err := s.getClient()
	if err != nil {
		return nil, err
	}

	// 1) 先 HEAD，尽量在下载前就拦截超大文件。
	headStart := time.Now()
	head, err := client.Head(ctx, key)
	if err != nil {
		return nil, err
	}
	headMs := sinceMs(headStart)
	if head.SizeBytes != nil && *head.SizeBytes > s.cfg.MaxFileBytes {
		return nil, apperr.New(
			"file_too_large",
			fmt.Sprintf("文件体积 %d 字节超过上限 %d 字节", *head.SizeBytes, s.cfg.MaxFileBytes),
			413, nil,
		)
	}

	// 2) 拉流并落临时文件。
	downloadStart := time.Now()
	body, _, err := client.Get(ctx, key, &head)
	if err != nil {
		return nil, err
	}
	download, err := tempfile.DownloadToFile(
		body, s.cfg.MaxFileBytes, s.cfg.TempDirPrefix, keyutil.ExtensionOf(key),
	)
	body.Close()
	if err != nil {
		return nil, err
	}
	downloadMs := sinceMs(downloadStart)
	defer tempfile.RemoveDir(download.Dir)

	// 3) 执行 ffprobe。
	//    非媒体/损坏文件会让 ffprobe 失败，此时不报错，而是仅返回 source。
	probeStart := time.Now()
	var data *ffprobe.Output
	var raw json.RawMessage
	var probeErrCode *string
	pr, err := s.runner.Run(ctx, download.FilePath)
	if err != nil {
		if ae, ok := apperr.As(err); ok && ae.Code == "probe_failed" {
			code := ae.Code
			probeErrCode = &code
		} else {
			// 超时、找不到 ffprobe 等仍按错误返回。
			return nil, err
		}
	} else {
		data = &pr.Data
		raw = json.RawMessage(pr.Raw)
	}
	probeMs := sinceMs(probeStart)

	sizeBytes := head.SizeBytes
	if sizeBytes == nil {
		v := download.Bytes
		sizeBytes = &v
	}

	// Content-Type 兜底：优先用 R2 HEAD 返回值，其次按扩展名推断，最后用通用类型。
	contentType := "application/octet-stream"
	if head.ContentType != nil {
		contentType = *head.ContentType
	} else if guessed, ok := mimetype.Guess(key); ok {
		contentType = guessed
	}
	ct := contentType

	source := SourceInfo{
		Type:         "r2",
		Bucket:       client.Bucket(),
		Key:          key,
		SizeBytes:    sizeBytes,
		ContentType:  &ct,
		ETag:         head.ETag,
		LastModified: head.LastModified,
	}

	var streams []map[string]any
	if data != nil {
		streams = data.Streams
	}

	timings := Timings{
		HeadMs:     headMs,
		DownloadMs: downloadMs,
		ProbeMs:    probeMs,
		TotalMs:    sinceMs(started),
	}

	// 4) 非视频/图片/音频：仅返回 source（体积、类型），按成功处理。
	if !anyMedia(streams) {
		fields := map[string]any{
			"key":         key,
			"bucket":      client.Bucket(),
			"sizeBytes":   derefInt64(sizeBytes),
			"contentType": contentType,
			"extension":   keyutil.ExtensionOf(key),
			"timings":     timings,
		}
		if probeErrCode != nil {
			fields["probeError"] = *probeErrCode
		}
		logging.Info("metadata.source_only", fields)
		return SourceOnly{Source: source}, nil
	}

	// 5) 媒体文件：归一化输出。
	videoStream := pickVideoStream(streams)
	audioStream := firstAudioStream(streams)

	var video *VideoStream
	if videoStream != nil {
		v := normalizeVideo(videoStream)
		video = &v
	}
	var audio *AudioStream
	if audioStream != nil {
		a := normalizeAudio(audioStream)
		audio = &a
	}

	var format map[string]any
	if data != nil {
		format = data.Format
	}
	chapters := dataChapters(data)
	if chapters == nil {
		chapters = []map[string]any{}
	}
	if streams == nil {
		streams = []map[string]any{}
	}
	if raw == nil {
		raw = json.RawMessage("{}")
	}

	result := MediaMetadata{
		Source:    source,
		Format:    normalizeFormat(format, sizeBytes),
		Video:     video,
		Audio:     audio,
		Subtitles: countSubtitles(streams),
		Streams:   streams,
		Chapters:  chapters,
		Probe: ProbeInfo{
			DurationMs:     probeMs,
			FfprobeVersion: s.runner.Version(ctx),
		},
		Timings: timings,
		Raw:     raw,
	}

	logging.Info("metadata.parsed", map[string]any{
		"key":             key,
		"bucket":          client.Bucket(),
		"sizeBytes":       derefInt64(sizeBytes),
		"contentType":     contentType,
		"extension":       keyutil.ExtensionOf(key),
		"durationSeconds": result.Format.DurationSeconds,
		"timings":         timings,
	})

	return result, nil
}

func sinceMs(t time.Time) int64 { return time.Since(t).Milliseconds() }

func anyMedia(streams []map[string]any) bool {
	for _, s := range streams {
		if isMediaStream(s) {
			return true
		}
	}
	return false
}

func countSubtitles(streams []map[string]any) int {
	n := 0
	for _, s := range streams {
		if codecType(s) == "subtitle" {
			n++
		}
	}
	return n
}

func dataChapters(data *ffprobe.Output) []map[string]any {
	if data == nil {
		return nil
	}
	return data.Chapters
}

func derefInt64(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}
