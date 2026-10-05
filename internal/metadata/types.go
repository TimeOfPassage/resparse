package metadata

import "encoding/json"

// SourceInfo 描述解析来源。当前仅实现 R2，后续可扩展 HTTP、上传等。
type SourceInfo struct {
	Type         string  `json:"type"`
	Bucket       string  `json:"bucket"`
	Key          string  `json:"key"`
	SizeBytes    *int64  `json:"sizeBytes"`
	ContentType  *string `json:"contentType"`
	ETag         *string `json:"etag"`
	LastModified *string `json:"lastModified"`
}

// SourceOnly 非视频/图片/音频文件的结果：仅返回来源信息，按成功处理。
type SourceOnly struct {
	Source SourceInfo `json:"source"`
}

// FormatInfo 归一化后的容器信息。
type FormatInfo struct {
	FormatName      *string           `json:"formatName"`
	FormatLongName  *string           `json:"formatLongName"`
	DurationSeconds *float64          `json:"durationSeconds"`
	SizeBytes       *int64            `json:"sizeBytes"`
	BitRate         *float64          `json:"bitRate"`
	Tags            map[string]string `json:"tags"`
}

// VideoStream 归一化后的视频流信息（取第一条 video 流）。
type VideoStream struct {
	Index              int      `json:"index"`
	Codec              *string  `json:"codec"`
	CodecLongName      *string  `json:"codecLongName"`
	Profile            *string  `json:"profile"`
	Level              *float64 `json:"level"`
	Width              *float64 `json:"width"`
	Height             *float64 `json:"height"`
	PixFmt             *string  `json:"pixFmt"`
	DisplayAspectRatio *string  `json:"displayAspectRatio"`
	FrameRate          *float64 `json:"frameRate"`
	BitRate            *float64 `json:"bitRate"`
	DurationSeconds    *float64 `json:"durationSeconds"`
}

// AudioStream 归一化后的音频流信息（取第一条 audio 流）。
type AudioStream struct {
	Index           int      `json:"index"`
	Codec           *string  `json:"codec"`
	CodecLongName   *string  `json:"codecLongName"`
	Profile         *string  `json:"profile"`
	SampleRate      *float64 `json:"sampleRate"`
	Channels        *float64 `json:"channels"`
	ChannelLayout   *string  `json:"channelLayout"`
	BitRate         *float64 `json:"bitRate"`
	DurationSeconds *float64 `json:"durationSeconds"`
}

// ProbeInfo 探测耗时与 ffprobe 版本。
type ProbeInfo struct {
	DurationMs     int64   `json:"durationMs"`
	FfprobeVersion *string `json:"ffprobeVersion"`
}

// Timings 各阶段耗时（毫秒），用于在响应与日志中排查性能问题。
type Timings struct {
	HeadMs     int64 `json:"headMs"`
	DownloadMs int64 `json:"downloadMs"`
	ProbeMs    int64 `json:"probeMs"`
	TotalMs    int64 `json:"totalMs"`
}

// MediaMetadata 媒体文件的完整元数据结果。
type MediaMetadata struct {
	Source    SourceInfo       `json:"source"`
	Format    FormatInfo       `json:"format"`
	Video     *VideoStream     `json:"video"`
	Audio     *AudioStream     `json:"audio"`
	Subtitles int              `json:"subtitles"`
	Streams   []map[string]any `json:"streams"`
	Chapters  []map[string]any `json:"chapters"`
	Probe     ProbeInfo        `json:"probe"`
	Timings   Timings          `json:"timings"`
	Raw       json.RawMessage  `json:"raw"`
}
