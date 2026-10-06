package auth

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"

	"social-network/backend/pkg/media"
)

const maxAvatarSize int64 = 10 << 20 // 10 MB

var allowedAvatarTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
}

// A function, not a variable: media.Dir is settable from the environment, and a package-level var would snapshot it at init.
func avatarDir() string { return filepath.Join(media.Dir, "avatars") }

func saveAvatar(
	file multipart.File,
	userID int64,
	contentType string,
) (string, error) {
	extension, ok := allowedAvatarTypes[contentType]
	if !ok {
		return "", fmt.Errorf("unsupported avatar type")
	}

	if err := os.MkdirAll(avatarDir(), 0755); err != nil {
		return "", err
	}

	filename := fmt.Sprintf(
		"user_%d%s",
		userID,
		extension,
	)

	filesystemPath := filepath.Join(
		avatarDir(),
		filename,
	)

	destination, err := os.Create(filesystemPath)
	if err != nil {
		return "", err
	}
	defer destination.Close()

	if _, err := io.Copy(destination, file); err != nil {
		os.Remove(filesystemPath)
		return "", err
	}

	return media.URLPrefix + "avatars/" + filename, nil
}

func removeAvatar(publicPath string) {
	if publicPath == "" {
		return
	}

	filename := filepath.Base(publicPath)

	path := filepath.Join(
		avatarDir(),
		filename,
	)

	os.Remove(path)
}
