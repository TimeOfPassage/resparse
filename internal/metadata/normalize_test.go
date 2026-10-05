package metadata

import (
	"math"
	"testing"
)

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func TestFractionToNumber(t *testing.T) {
	if got := fracToNum(strPtr("30000/1001")); got == nil || !approx(*got, 29.97002997) {
		t.Errorf("30000/1001 = %v", got)
	}
	if got := fracToNum(strPtr("0/0")); got != nil {
		t.Errorf("0/0 should be nil, got %v", *got)
	}
	if got := fracToNum(nil); got != nil {
		t.Errorf("nil should be nil")
	}
	if got := fracToNum(strPtr("25")); got == nil || *got != 25 {
		t.Errorf("25 = %v", got)
	}
}

func TestNormalizeVideo(t *testing.T) {
	s := map[string]any{
		"index":                float64(0),
		"codec_name":           "h264",
		"codec_long_name":      "H.264 / AVC",
		"profile":              "High",
		"level":                float64(41),
		"width":                float64(1920),
		"height":               float64(1080),
		"pix_fmt":              "yuv420p",
		"display_aspect_ratio": "16:9",
		"avg_frame_rate":       "30000/1001",
		"bit_rate":             "750000",
		"duration":             "12.34",
	}
	v := normalizeVideo(s)

	if v.Codec == nil || *v.Codec != "h264" {
		t.Errorf("codec = %v", v.Codec)
	}
	if v.Width == nil || *v.Width != 1920 || v.Height == nil || *v.Height != 1080 {
		t.Errorf("size = %v x %v", v.Width, v.Height)
	}
	if v.BitRate == nil || *v.BitRate != 750000 {
		t.Errorf("bitRate = %v", v.BitRate)
	}
	if v.DurationSeconds == nil || !approx(*v.DurationSeconds, 12.34) {
		t.Errorf("duration = %v", v.DurationSeconds)
	}
	if v.FrameRate == nil || !approx(*v.FrameRate, 29.97002997) {
		t.Errorf("frameRate = %v", v.FrameRate)
	}
}

func TestNormalizeAudio(t *testing.T) {
	s := map[string]any{
		"index":          float64(1),
		"codec_name":     "aac",
		"sample_rate":    "48000",
		"channels":       float64(2),
		"channel_layout": "stereo",
	}
	a := normalizeAudio(s)
	if a.SampleRate == nil || *a.SampleRate != 48000 {
		t.Errorf("sampleRate = %v", a.SampleRate)
	}
	if a.Channels == nil || *a.Channels != 2 {
		t.Errorf("channels = %v", a.Channels)
	}
}

func TestNormalizeFormat(t *testing.T) {
	fallback := int64(999)
	f := map[string]any{
		"format_name":      "mov,mp4,m4a,3gp,3g2,mj2",
		"format_long_name": "QuickTime / MOV",
		"duration":         "12.34",
		"size":             "1234567",
		"bit_rate":         "800000",
		"tags":             map[string]any{"major_brand": "isom"},
	}
	fi := normalizeFormat(f, &fallback)
	if fi.SizeBytes == nil || *fi.SizeBytes != 1234567 {
		t.Errorf("sizeBytes = %v", fi.SizeBytes)
	}
	if fi.Tags == nil || fi.Tags["major_brand"] != "isom" {
		t.Errorf("tags = %v", fi.Tags)
	}

	// size 缺失时回退到 fallbackSize。
	fi2 := normalizeFormat(map[string]any{}, &fallback)
	if fi2.SizeBytes == nil || *fi2.SizeBytes != 999 {
		t.Errorf("fallback sizeBytes = %v", fi2.SizeBytes)
	}
	if fi2.Tags != nil {
		t.Errorf("tags should be nil, got %v", fi2.Tags)
	}
}

func TestPickVideoStreamSkipsAttachedPic(t *testing.T) {
	streams := []map[string]any{
		{
			"index":       float64(0),
			"codec_type":  "video",
			"disposition": map[string]any{"attached_pic": float64(1)},
		},
		{
			"index":       float64(1),
			"codec_type":  "video",
			"disposition": map[string]any{"attached_pic": float64(0)},
		},
	}
	picked := pickVideoStream(streams)
	if picked == nil || *getInt(picked, "index") != 1 {
		t.Errorf("expected non-cover video stream (index 1), got %v", picked)
	}
}

func TestIsMediaStream(t *testing.T) {
	if !isMediaStream(map[string]any{"codec_type": "video"}) {
		t.Error("video should be media")
	}
	if !isMediaStream(map[string]any{"codec_type": "audio"}) {
		t.Error("audio should be media")
	}
	if isMediaStream(map[string]any{"codec_type": "subtitle"}) {
		t.Error("subtitle should not be media")
	}
}

func strPtr(s string) *string { return &s }
