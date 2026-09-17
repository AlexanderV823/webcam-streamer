//go:build !linux

package ffmpeg

import (
	"context"
	"fmt"
	"io"
	"log"
	"os/exec"
)

type FFmpegStreamer struct{}

func NewStreamer() *FFmpegStreamer {
	return &FFmpegStreamer{}
}

func (s *FFmpegStreamer) Start(ctx context.Context, url string, width, height, fps int) (<-chan []byte, <-chan error, error) {
	args := []string{
		"-rtsp_transport", "tcp",
		"-i", url,
		"-vf", fmt.Sprintf("fps=%d,scale=%d:%d", fps, width, height),
		"-f", "image2pipe",
		"-vcodec", "mjpeg",
		"pipe:1",
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	cmd.Stderr = log.Writer()

	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	videoChan := make(chan []byte, 10)
	errChan := make(chan error, 1)

	go func() {
		defer close(videoChan)
		defer close(errChan)

		buf := make([]byte, 64*1024)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				chunk := make([]byte, n)
				copy(chunk, buf[:n])
				select {
				case videoChan <- chunk:
				case <-ctx.Done():
					return
				}
			}
			if err != nil {
				if err != io.EOF && ctx.Err() == nil {
					errChan <- err
				}
				return
			}
		}
	}()

	go func() {
		err := cmd.Wait()
		if err != nil && ctx.Err() == nil {
			log.Printf("FFmpeg process exited with error: %v", err)
		}
		if ctx.Err() != nil {
			_ = cmd.Process.Kill()
		}
	}()

	return videoChan, errChan, nil
}
