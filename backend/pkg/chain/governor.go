package chain

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	"smart-community/pkg/secure"
)

type GovernorClient struct {
	rpcURL          string
	contractAddress common.Address
	privateKeyHex   string

	client   *ethclient.Client
	contract *bind.BoundContract
	parsedABI abi.ABI
	chainID  *big.Int
	from     common.Address
}

// 仅包含自动结算所需的方法
const governorABIJSON = `[
  {"inputs":[{"internalType":"uint256","name":"proposalId","type":"uint256"}],"name":"resolve","outputs":[],"stateMutability":"nonpayable","type":"function"},
  {"inputs":[{"internalType":"uint256","name":"proposalId","type":"uint256"}],"name":"getProposal","outputs":[{"components":[
    {"internalType":"uint256","name":"id","type":"uint256"},
    {"internalType":"bytes32","name":"contentHash","type":"bytes32"},
    {"internalType":"address","name":"creator","type":"address"},
    {"internalType":"uint8","name":"propType","type":"uint8"},
    {"internalType":"uint64","name":"startTime","type":"uint64"},
    {"internalType":"uint64","name":"deadline","type":"uint64"},
    {"internalType":"uint256","name":"yesVotes","type":"uint256"},
    {"internalType":"uint256","name":"noVotes","type":"uint256"},
    {"internalType":"uint256","name":"abstainVotes","type":"uint256"},
    {"internalType":"uint8","name":"status","type":"uint8"},
    {"internalType":"bool","name":"tallied","type":"bool"}
  ],"internalType":"struct SmartCommunityGovernor.Proposal","name":"","type":"tuple"}],"stateMutability":"view","type":"function"}
]`

type GovernorProposal struct {
	Id          *big.Int
	ContentHash [32]byte
	Creator     common.Address
	PropType    uint8
	StartTime   uint64
	Deadline    uint64
	YesVotes    *big.Int
	NoVotes     *big.Int
	AbstainVotes *big.Int
	Status      uint8
	Tallied     bool
}

// NewGovernorClientFromEnv 从环境变量初始化合约客户端
//
// - SC_RPC_URL: RPC 地址（默认 http://127.0.0.1:8545）
// - SC_GOVERNOR_ADDRESS: 合约地址（必须）
// - SC_ADMIN_PRIVATE_KEY_ENC: 管理员私钥（DPAPI 加密，base64）（推荐）
// - SC_ADMIN_PRIVATE_KEY: 管理员私钥（明文，备选）
func NewGovernorClientFromEnv() (*GovernorClient, error) {
	// 加载环境变量：为了兼容从项目根目录/从 backend 目录启动
	// 这里依次尝试加载 .env / backend/.env / ./backend/.env
	if err := godotenv.Load(); err != nil {
		// 常见情况：运行工作目录是项目根目录 D:\code\work，所以 backend/.env 才是目标
		_ = godotenv.Load("backend/.env")
		_ = godotenv.Load("./backend/.env")
	}

	rpcURL := os.Getenv("SC_RPC_URL")
	if strings.TrimSpace(rpcURL) == "" {
		rpcURL = "http://127.0.0.1:8545"
	}

	contractAddr := strings.TrimSpace(os.Getenv("SC_GOVERNOR_ADDRESS"))
	enc := strings.TrimSpace(os.Getenv("SC_ADMIN_PRIVATE_KEY_ENC"))
	priv := strings.TrimSpace(os.Getenv("SC_ADMIN_PRIVATE_KEY"))
	log.Printf("[governor-env] rpc=%q governorAddressSet=%t adminEncSet=%t adminKeySet=%t",
		rpcURL,
		contractAddr != "",
		enc != "",
		priv != "",
	)
	if contractAddr == "" {
		return nil, fmt.Errorf("SC_GOVERNOR_ADDRESS is required")
	}

	if enc == "" && priv == "" {
		return nil, fmt.Errorf("SC_ADMIN_PRIVATE_KEY_ENC or SC_ADMIN_PRIVATE_KEY is required")
	}

	if enc != "" {
		plain, err := secure.DecryptFromBase64(enc)
		if err != nil {
			return nil, fmt.Errorf("decrypt SC_ADMIN_PRIVATE_KEY_ENC failed: %w", err)
		}
		priv = strings.TrimSpace(plain)
	}

	if strings.HasPrefix(priv, "0x") {
		priv = priv[2:]
	}

	addr := common.HexToAddress(contractAddr)
	return NewGovernorClient(rpcURL, addr, priv)
}

func NewGovernorClient(rpcURL string, addr common.Address, privateKeyHex string) (*GovernorClient, error) {
	parsedABI, err := abi.JSON(strings.NewReader(governorABIJSON))
	if err != nil {
		return nil, fmt.Errorf("parse abi: %w", err)
	}

	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("dial rpc: %w", err)
	}

	chainID, err := client.ChainID(context.Background())
	if err != nil {
		return nil, fmt.Errorf("get chain id: %w", err)
	}

	pk, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}
	from := crypto.PubkeyToAddress(pk.PublicKey)

	contract := bind.NewBoundContract(addr, parsedABI, client, client, client)

	return &GovernorClient{
		rpcURL:          rpcURL,
		contractAddress: addr,
		privateKeyHex:   privateKeyHex,
		client:          client,
		contract:        contract,
		parsedABI:       parsedABI,
		chainID:         chainID,
		from:            from,
	}, nil
}

func (g *GovernorClient) FromAddress() string {
	return g.from.Hex()
}

func (g *GovernorClient) Close() {
	// ethclient 没有显式 Close，这里保留以便未来扩展
}

func (g *GovernorClient) keyedTransactor(ctx context.Context) (*bind.TransactOpts, *ecdsa.PrivateKey, error) {
	pk, err := crypto.HexToECDSA(g.privateKeyHex)
	if err != nil {
		return nil, nil, err
	}
	auth, err := bind.NewKeyedTransactorWithChainID(pk, g.chainID)
	if err != nil {
		return nil, nil, err
	}
	auth.Context = ctx
	return auth, pk, nil
}

func (g *GovernorClient) GetProposal(ctx context.Context, proposalID uint64) (GovernorProposal, error) {
	// eth_call
	input, err := g.parsedABI.Pack("getProposal", new(big.Int).SetUint64(proposalID))
	if err != nil {
		return GovernorProposal{}, err
	}
	msg := ethereum.CallMsg{
		To:   &g.contractAddress,
		Data: input,
	}
	outBytes, err := g.client.CallContract(ctx, msg, nil)
	if err != nil {
		return GovernorProposal{}, err
	}

	values, err := g.parsedABI.Unpack("getProposal", outBytes)
	if err != nil {
		return GovernorProposal{}, err
	}
	if len(values) != 1 {
		return GovernorProposal{}, fmt.Errorf("unexpected return values: %d", len(values))
	}

	converted := abi.ConvertType(values[0], new(GovernorProposal))
	gp, ok := converted.(*GovernorProposal)
	if !ok || gp == nil {
		return GovernorProposal{}, fmt.Errorf("failed to convert getProposal result")
	}
	return *gp, nil
}

func (g *GovernorClient) Resolve(ctx context.Context, proposalID uint64) (txHash string, final GovernorProposal, err error) {
	// 先读一次，避免重复结算
	p, err := g.GetProposal(ctx, proposalID)
	if err != nil {
		return "", GovernorProposal{}, err
	}
	if p.Tallied {
		return "", p, nil
	}

	auth, _, err := g.keyedTransactor(ctx)
	if err != nil {
		return "", GovernorProposal{}, err
	}

	tx, err := g.contract.Transact(auth, "resolve", new(big.Int).SetUint64(proposalID))
	if err != nil {
		return "", GovernorProposal{}, err
	}

	// 等待交易确认
	waitCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if _, err := bind.WaitMined(waitCtx, g.client, tx); err != nil {
		return tx.Hash().Hex(), GovernorProposal{}, err
	}

	// 再读一次获取最终状态
	p2, err := g.GetProposal(ctx, proposalID)
	if err != nil {
		log.Printf("resolve mined but getProposal failed: %v", err)
		return tx.Hash().Hex(), GovernorProposal{}, nil
	}
	return tx.Hash().Hex(), p2, nil
}

