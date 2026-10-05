package metadata

import (
	"context"
	"encoding/binary"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"resparse/internal/config"
	"resparse/internal/ffprobe"
)

// wavBytes 生成一段最小可用的单声道 16bit PCM WAV，供 ffprobe 解析。
func wavBytes(sampleRate, numSamples int) []byte {
	const bitsPerSample = 16
	channels := 1
	dataSize := numSamples * channels * bitsPerSample / 8
	byteRate := sampleRate * channels * bitsPerSample / 8
	blockAlign := channels * bitsPerSample / 8

	buf := make([]byte, 44+dataSize)
	copy(buf[0:], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:], uint32(36+dataSize))
	copy(buf[8:], "WAVE")
	copy(buf[12:], "fmt ")
	binary.LittleEndian.PutUint32(buf[16:], 16) // subchunk1 size
	binary.LittleEndian.PutUint16(buf[20:], 1)  // PCM
	binary.LittleEndian.PutUint16(buf[22:], uint16(channels))
	binary.LittleEndian.PutUint32(buf[24:], uint32(sampleRate))
	binary.LittleEndian.PutUint32(buf[28:], uint32(byteRate))
	binary.LittleEndian.PutUint16(buf[32:], uint16(blockAlign))
	binary.LittleEndian.PutUint16(buf[34:], bitsPerSample)
	copy(buf[36:], "data")
	binary.LittleEndian.PutUint32(buf[40:], uint32(dataSize))
	// 样本区保持全 0（静音）。
	return buf
}

// TestParseEndToEnd 走完整的 HEAD -> GET -> 落盘 -> ffprobe -> 归一化流程。
// 用 httptest 模拟 R2（忽略签名），仅依赖本机 ffprobe，缺失时跳过。
func TestParseEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("未安装 ffprobe，跳过端到端测试")
	}

	wav := wavBytes(8000, 800) // 0.1s 静音
	const key = "test.wav"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/test-bucket/"+key {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "audio/wav")
		w.Header().Set("ETag", `"abc123"`)
		w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
		w.Header().Set("Content-Length", strconv.Itoa(len(wav)))
		if r.Method == http.MethodGet {
			_, _ = w.Write(wav)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		AccountID:       "test-account",
		AccessKeyID:     "test-key",
		SecretAccessKey: "test-secret",
		Bucket:          "test-bucket",
		Endpoint:        srv.URL,
		Region:          "auto",
		FfprobePath:     "ffprobe",
		ProbeTimeout:    30 * time.Second,
		MaxFileBytes:    64 * 1024 * 1024,
		TempDirPrefix:   "metaparse-test-",
	}

	runner := ffprobe.NewRunner(cfg.FfprobePath, cfg.ProbeTimeout)
	svc := NewService(cfg, runner)

	got, err := svc.Parse(context.Background(), key)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	md, ok := got.(MediaMetadata)
	if !ok {
		t.Fatalf("expected MediaMetadata, got %T", got)
	}
	if md.Source.SizeBytes == nil || *md.Source.SizeBytes != int64(len(wav)) {
		t.Errorf("source.sizeBytes = %v, want %d", md.Source.SizeBytes, len(wav))
	}
	if md.Source.ContentType == nil || *md.Source.ContentType != "audio/wav" {
		t.Errorf("source.contentType = %v", md.Source.ContentType)
	}
	if md.Audio == nil {
		t.Fatal("expected audio stream")
	}
	if md.Audio.SampleRate == nil || *md.Audio.SampleRate != 8000 {
		t.Errorf("audio.sampleRate = %v, want 8000", md.Audio.SampleRate)
	}
	if md.Probe.FfprobeVersion == nil {
		t.Errorf("expected ffprobe version")
	}
	if len(md.Streams) == 0 {
		t.Errorf("expected streams")
	}
}
