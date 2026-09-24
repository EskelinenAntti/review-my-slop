package comments

import "path/filepath"

func DataDir() (string, error) {
	return appDir("XDG_DATA_HOME", filepath.Join(".local", "share"))
}

func DefaultPath() (string, error) {
	data, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(data, "comments.db"), nil
}
