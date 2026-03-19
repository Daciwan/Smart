import { JsonRpcProvider, Wallet, ContractFactory } from "ethers";
import governorJson from "../artifacts/contracts/SmartCommunityGovernor.sol/SmartCommunityGovernor.json" assert { type: "json" };

// 使用你在 Ganache 中的第一个私钥（确保 Ganache 正在使用同一组私钥）
const PRIVATE_KEY =
  "0x368d5980d5f16e80aeb24f704e543f18af05f84b15bdabc3d8f18af90c36e802";
const RPC_URL = "http://127.0.0.1:8545";

async function main() {
  // 默认最小公示期 3 天，阈值 51%
  const minDurationSeconds = 3 * 24 * 60 * 60;
  const passPercentage = 51;

  const provider = new JsonRpcProvider(RPC_URL);
  const deployer = new Wallet(PRIVATE_KEY, provider);

  console.log("Deploying contracts with account:", deployer.address);
  const balance = await provider.getBalance(deployer.address);
  console.log("Deployer balance:", balance.toString(), "wei");

  const factory = new ContractFactory(
    governorJson.abi,
    governorJson.bytecode,
    deployer
  );

  const contract = await factory.deploy(minDurationSeconds, passPercentage);
  await contract.waitForDeployment();

  const address = await contract.getAddress();
  console.log("SmartCommunityGovernor deployed to:", address);
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});