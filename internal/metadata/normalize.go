package metadata

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// 本文件将 ffprobe 的原始 map 输出归一化为稳定的、类型明确的字段。
// 对应参考项目 src/lib/server/metadata.ts 中的 normalize* 系列函数。

func finite(f float64) bool {
	return !math.IsNaN(f) && !math.IsInf(f, 0)
}

func floatPtr(f float64) *float64 {
	if !finite(f) {
		return nil
	}
	return &f
}

func getStr(m map[string]any, key string) *string {
	if m == nil {
		return nil
	}
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	switch t := v.(type) {
	case string:
		return &t
	case fmt.Stringer:
		s := t.String()
		return &s
	default:
		return nil
	}
}

// getNum 将值转换为数值，兼容 JSON 数字与字符串数字。
func getNum(m map[string]any, key string) *float64 {
	if m == nil {
		return nil
	}
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	switch t := v.(type) {
	case float64:
		return floatPtr(t)
	case int:
		return floatPtr(float64(t))
	case int64:
		return floatPtr(float64(t))
	case string:
		if t == "" {
			return nil
		}
		if f, err := strconv.ParseFloat(t, 64); err == nil {
			return floatPtr(f)
		}
		return nil
	case fmt.Stringer:
		if f, err := strconv.ParseFloat(t.String(), 64); err == nil {
			return floatPtr(f)
		}
		return nil
	default:
		return nil
	}
}

func getInt(m map[string]any, key string) *int {
	f := getNum(m, key)
	if f == nil {
		return nil
	}
	i := int(*f)
	return &i
}

func getInt64(m map[string]any, key string) *int64 {
	f := getNum(m, key)
	if f == nil {
		return nil
	}
	n := int64(*f)
	return &n
}

// fracToNum 将 `"30000/1001"` 这类分数帧率转换为小数。
func fracToNum(s *string) *float64 {
	if s == nil {
		return nil
	}
	parts := strings.SplitN(*s, "/", 2)
	num, err := strconv.ParseFloat(parts[0], 64)
	if err != nil || !finite(num) {
		return nil
	}
	den := 1.0
	if len(parts) == 2 {
		den, err = strconv.ParseFloat(parts[1], 64)
		if err != nil || !finite(den) {
			return nil
		}
	}
	if den == 0 {
		return nil
	}
	return floatPtr(num / den)
}

func getStrMap(m map[string]any, key string) map[string]string {
	if m == nil {
		return nil
	}
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	raw, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	out := make(map[string]string, len(raw))
	for k, val := range raw {
		if s, ok := val.(string); ok {
			out[k] = s
		} else {
			out[k] = fmt.Sprint(val)
		}
	}
	return out
}

func codecType(s map[string]any) string {
	if v, ok := s["codec_type"].(string); ok {
		return v
	}
	return ""
}

// isMediaStream 判断是否属于需要解析的媒体类型：视频 / 图片（ffprobe 归为 video 流）/ 音频。
func isMediaStream(s map[string]any) bool {
	t := codecType(s)
	return t == "video" || t == "audio"
}

func hasAttachedPic(s map[string]any) bool {
	disp, ok := s["disposition"].(map[string]any)
	if !ok {
		return false
	}
	v := getNum(disp, "attached_pic")
	return v != nil && *v == 1
}

// pickVideoStream 优先选择非封面图（attached_pic）的视频流。
func pickVideoStream(streams []map[string]any) map[string]any {
	var first, fallback map[string]any
	for _, s := range streams {
		if codecType(s) != "video" {
			continue
		}
		if fallback == nil {
			fallback = s
		}
		if !hasAttachedPic(s) {
			first = s
			break
		}
	}
	if first != nil {
		return first
	}
	return fallback
}

func firstAudioStream(streams []map[string]any) map[string]any {
	for _, s := range streams {
		if codecType(s) == "audio" {
			return s
		}
	}
	return nil
}

func normalizeVideo(s map[string]any) VideoStream {
	return VideoStream{
		Index:              derefInt(getInt(s, "index")),
		Codec:              getStr(s, "codec_name"),
		CodecLongName:      getStr(s, "codec_long_name"),
		Profile:            getStr(s, "profile"),
		Level:              getNum(s, "level"),
		Width:              getNum(s, "width"),
		Height:             getNum(s, "height"),
		PixFmt:             getStr(s, "pix_fmt"),
		DisplayAspectRatio: getStr(s, "display_aspect_ratio"),
		FrameRate:          fracToNum(firstPtr(getStr(s, "avg_frame_rate"), getStr(s, "r_frame_rate"))),
		BitRate:            getNum(s, "bit_rate"),
		DurationSeconds:    getNum(s, "duration"),
	}
}

func normalizeAudio(s map[string]any) AudioStream {
	return AudioStream{
		Index:           derefInt(getInt(s, "index")),
		Codec:           getStr(s, "codec_name"),
		CodecLongName:   getStr(s, "codec_long_name"),
		Profile:         getStr(s, "profile"),
		SampleRate:      getNum(s, "sample_rate"),
		Channels:        getNum(s, "channels"),
		ChannelLayout:   getStr(s, "channel_layout"),
		BitRate:         getNum(s, "bit_rate"),
		DurationSeconds: getNum(s, "duration"),
	}
}

func normalizeFormat(format map[string]any, fallbackSize *int64) FormatInfo {
	size := getInt64(format, "size")
	if size == nil {
		size = fallbackSize
	}
	return FormatInfo{
		FormatName:      getStr(format, "format_name"),
		FormatLongName:  getStr(format, "format_long_name"),
		DurationSeconds: getNum(format, "duration"),
		SizeBytes:       size,
		BitRate:         getNum(format, "bit_rate"),
		Tags:            getStrMap(format, "tags"),
	}
}

func firstPtr(a, b *string) *string {
	if a != nil {
		return a
	}
	return b
}

func derefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}
