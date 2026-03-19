//go:build !windows

package secure

import "fmt"

func EncryptToBase64(_ string) (string, error) {
	return "", fmt.Errorf("dpapi only supported on windows")
}

func DecryptFromBase64(_ string) (string, error) {
	return "", fmt.Errorf("dpapi only supported on windows")
}

