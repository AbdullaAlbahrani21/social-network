package posts

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/google/uuid"

	"social-network/backend/pkg/media"
)

const maxImageSize = 10 << 20 // 10MB

var allowedImageTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
}

// SaveUploadedImage stores an optional JPEG/PNG/GIF from the form under a random name and returns its URL path.
func SaveUploadedImage(r *http.Request, field string) (string, error) {
	file, header, err := r.FormFile(field)
	if errors.Is(err, http.ErrMissingFile) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	defer file.Close()

	if header.Size > maxImageSize {
		return "", fmt.Errorf("image must be under %dMB", maxImageSize/(1<<20))
	}

	sniff := make([]byte, 512)
	n, err := file.Read(sniff)
	if err != nil && err != io.EOF {
		return "", err
	}
	ext, ok := allowedImageTypes[http.DetectContentType(sniff[:n])]
	if !ok {
		return "", fmt.Errorf("image must be JPEG, PNG, or GIF")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", err
	}

	if err := os.MkdirAll(media.Dir, 0755); err != nil {
		return "", err
	}

	filename := uuid.NewString() + ext
	dst, err := os.Create(filepath.Join(media.Dir, filename))
	if err != nil {
		return "", err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		return "", err
	}

	return media.URLPrefix + filename, nil
}

// removeUploadedImage deletes a saved image file, used when saving the post fails afterwards.
func removeUploadedImage(publicPath string) {
	if publicPath == "" {
		return
	}
	os.Remove(filepath.Join(media.Dir, filepath.Base(publicPath)))
}

// nullable turns an empty string into SQL NULL.
func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
