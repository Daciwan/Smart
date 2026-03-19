import { HardhatUserConfig } from "hardhat/config";

const ganacheRpcUrl = "http://127.0.0.1:8545";

// 使用你提供的 4 个私钥作为本地账户，便于直接部署和调试
const accounts = [
  "0x368d5980d5f16e80aeb24f704e543f18af05f84b15bdabc3d8f18af90c36e802",
  "0x2070a8619ca02f320af7eae65e9ddc5ce402478daa297030a89e32c5aa1c2dc9",
  "0x34757654e2a434f9da6b7abab9da91af8ca619e9f6a82d8c395d7252d18a1b03",
  "0xa9c1e7525b7ea36d3ef4c4b986ccbfddd1618e429c9b06f1b94f42191e6f9d25",
];

const config: HardhatUserConfig = {
  solidity: {
    version: "0.8.24",
    settings: {
      optimizer: {
        enabled: true,
        runs: 200,
      },
      // 让编译出的字节码兼容 Ganache（避免使用 Shanghai 的 PUSH0 等新指令）
      evmVersion: "paris",
    },
  },
  networks: {
    ganache: {
      type: "http",
      url: ganacheRpcUrl,
      accounts,
    },
  },
};

export default config;

