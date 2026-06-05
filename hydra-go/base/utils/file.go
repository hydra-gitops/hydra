package utils

import "strings"

func FileUriToPath(fileUri string) string {
	return strings.TrimPrefix(fileUri, "file://")
}
