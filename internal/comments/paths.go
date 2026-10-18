package comments

import (
	"github.com/eskelinenantti/review-my-slop/internal/appinfo"
)

// DataDir returns the application data directory, respecting XDG_DATA_HOME.
func DataDir() (string, error) {
	return appinfo.DataDir()
}
