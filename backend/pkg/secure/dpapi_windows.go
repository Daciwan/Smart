//go:build windows

package secure

import (
	"encoding/base64"
	"fmt"
	"syscall"
	"unsafe"
)

type dataBlob struct {
	cbData uint32
	pbData *byte
}

var (
	crypt32              = syscall.NewLazyDLL("crypt32.dll")
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	procCryptProtectData = crypt32.NewProc("CryptProtectData")
	procCryptUnprotectData = crypt32.NewProc("CryptUnprotectData")
	procLocalFree        = kernel32.NewProc("LocalFree")
)

func bytesToBlob(b []byte) *dataBlob {
	if len(b) == 0 {
		return &dataBlob{}
	}
	return &dataBlob{
		cbData: uint32(len(b)),
		pbData: &b[0],
	}
}

func blobToBytes(blob *dataBlob) []byte {
	if blob == nil || blob.cbData == 0 || blob.pbData == nil {
		return nil
	}
	buf := unsafe.Slice(blob.pbData, blob.cbData)
	out := make([]byte, len(buf))
	copy(out, buf)
	return out
}

// EncryptToBase64 使用 Windows DPAPI（当前用户）加密明文，输出 base64 密文。
func EncryptToBase64(plain string) (string, error) {
	in := []byte(plain)
	var out dataBlob

	r, _, err := procCryptProtectData.Call(
		uintptr(unsafe.Pointer(bytesToBlob(in))),
		0,
		0,
		0,
		0,
		0,
		uintptr(unsafe.Pointer(&out)),
	)
	if r == 0 {
		return "", fmt.Errorf("CryptProtectData failed: %v", err)
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))

	cipher := blobToBytes(&out)
	return base64.StdEncoding.EncodeToString(cipher), nil
}

// DecryptFromBase64 使用 Windows DPAPI（当前用户）解密 base64 密文，输出明文。
func DecryptFromBase64(cipherBase64 string) (string, error) {
	cipher, err := base64.StdEncoding.DecodeString(cipherBase64)
	if err != nil {
		return "", err
	}

	var out dataBlob
	r, _, err := procCryptUnprotectData.Call(
		uintptr(unsafe.Pointer(bytesToBlob(cipher))),
		0,
		0,
		0,
		0,
		0,
		uintptr(unsafe.Pointer(&out)),
	)
	if r == 0 {
		return "", fmt.Errorf("CryptUnprotectData failed: %v", err)
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))

	plain := blobToBytes(&out)
	return string(plain), nil
}

