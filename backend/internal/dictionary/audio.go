package dictionary

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

type audioTranscoder struct {
	cacheRoot string
	ffmpegBin string
	mu        sync.Mutex
}

func newAudioTranscoder(cacheRoot string, ffmpegBin string) *audioTranscoder {
	return &audioTranscoder{
		cacheRoot: strings.TrimSpace(cacheRoot),
		ffmpegBin: strings.TrimSpace(ffmpegBin),
	}
}

func (t *audioTranscoder) enabled() bool {
	return t != nil && t.ffmpegBin != "" && t.cacheRoot != ""
}

func (t *audioTranscoder) transcodeToMP3(resourcePath string, data []byte) ([]byte, error) {
	if !t.enabled() {
		return nil, errors.New("ffmpeg transcoder is not configured")
	}
	if err := os.MkdirAll(t.cacheRoot, 0o755); err != nil {
		return nil, err
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	sum := sha1.Sum(append([]byte(resourcePath+"\x00"), data...))
	key := hex.EncodeToString(sum[:])
	inputPath := filepath.Join(t.cacheRoot, key+".spx")
	outputPath := filepath.Join(t.cacheRoot, key+".mp3")

	if cached, err := os.ReadFile(outputPath); err == nil {
		return cached, nil
	}
	if err := os.WriteFile(inputPath, data, 0o644); err != nil {
		return nil, err
	}

	cmd := exec.Command(t.ffmpegBin, "-y", "-i", inputPath, "-vn", "-ac", "1", "-codec:a", "libmp3lame", "-b:a", "64k", outputPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg transcode failed: %v :: %s", err, strings.TrimSpace(string(out)))
	}
	transcoded, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, err
	}
	return transcoded, nil
}

func resolveFFmpegBinary() string {
	if explicit := strings.TrimSpace(os.Getenv("FFMPEG_BIN")); explicit != "" {
		if _, err := os.Stat(explicit); err == nil {
			return explicit
		}
	}
	path, err := exec.LookPath("ffmpeg")
	if err != nil {
		return ""
	}
	return path
}

func looksLikeSpeex(data []byte) bool {
	return len(data) > 48 && strings.Contains(string(data[:64]), "Speex")
}

// AudioProbeTranscoder exposes the internal transcoder for local verification helpers.
type AudioProbeTranscoder struct{ inner *audioTranscoder }

func dictionaryProbeNewAudioTranscoder(cacheRoot string, ffmpegBin string) *AudioProbeTranscoder {
	return &AudioProbeTranscoder{inner: newAudioTranscoder(cacheRoot, ffmpegBin)}
}

func (p *AudioProbeTranscoder) Transcode(resourcePath string, data []byte) ([]byte, error) {
	return p.inner.transcodeToMP3(resourcePath, data)
}
