package fs

import (
	"context"
	"io"
	"os"
	"time"
)

// TailFile reads a file from the beginning and streams lines to outChan.
// It continues to poll for new writes until ctx is canceled.
func TailFile(ctx context.Context, path string, outChan chan<- []byte) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	buf := make([]byte, 4096)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// Attempt to read new data
			for {
				n, err := file.Read(buf)
				if n > 0 {
					// Copy the bytes because buf will be reused
					chunk := make([]byte, n)
					copy(chunk, buf[:n])
					outChan <- chunk
				}
				if err == io.EOF {
					break // Wait for next tick
				}
				if err != nil {
					return err // Fatal error
				}
			}
		}
	}
}
