package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/joho/godotenv"

	"smart-community/pkg/secure"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "set":
		runSet(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Println("用法：")
	fmt.Println("  go run ./cmd/secureconfig set --rpc http://127.0.0.1:8545 --address <合约地址> --pk <管理员私钥>")
	fmt.Println("")
	fmt.Println("说明：会在 backend/.env 写入：")
	fmt.Println("  SC_RPC_URL=...")
	fmt.Println("  SC_GOVERNOR_ADDRESS=...")
	fmt.Println("  SC_ADMIN_PRIVATE_KEY_ENC=...   (DPAPI 加密的 base64 密文)")
}

func runSet(args []string) {
	flags := map[string]string{}
	for i := 0; i < len(args); i++ {
		if !strings.HasPrefix(args[i], "--") {
			continue
		}
		key := strings.TrimPrefix(args[i], "--")
		if i+1 >= len(args) {
			fmt.Printf("缺少参数值：--%s\n", key)
			os.Exit(2)
		}
		flags[key] = args[i+1]
		i++
	}

	rpc := strings.TrimSpace(flags["rpc"])
	addr := strings.TrimSpace(flags["address"])
	pk := strings.TrimSpace(flags["pk"])

	if rpc == "" || addr == "" || pk == "" {
		fmt.Println("缺少必要参数：--rpc、--address、--pk")
		os.Exit(2)
	}

	if strings.HasPrefix(pk, "0x") {
		pk = pk[2:]
	}

	enc, err := secure.EncryptToBase64(pk)
	if err != nil {
		fmt.Printf("加密失败：%v\n", err)
		os.Exit(1)
	}

	// 无论从哪个目录运行，都把 .env 写到：repo/backend/.env
	// main.go 位于：repo/backend/cmd/secureconfig/main.go
	// 所以 backEndDir = 上移 3 级目录后的位置：repo/backend
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		fmt.Println("无法定位运行路径，请重试")
		os.Exit(1)
	}
	secureconfigDir := filepath.Dir(thisFile)                // .../backend/cmd/secureconfig
	backendCmdDir := filepath.Dir(secureconfigDir)           // .../backend/cmd
	backendDir := filepath.Dir(backendCmdDir)               // .../backend
	envPath := filepath.Join(backendDir, ".env")            // .../backend/.env

	err = godotenv.Write(map[string]string{
		"SC_RPC_URL":              rpc,
		"SC_GOVERNOR_ADDRESS":     addr,
		"SC_ADMIN_PRIVATE_KEY_ENC": enc,
	}, envPath)
	if err != nil {
		fmt.Printf("写入 .env 失败：%v\n", err)
		os.Exit(1)
	}

	fmt.Printf("已写入 %s（密文保存，不含明文私钥）\n", envPath)
}

