## 智慧社区协同决策平台（毕设项目骨架）

本项目按照你的《需求规格说明书》要求，实现了一个基于区块链的智慧社区协同决策平台的完整工程骨架：

- 后端：Go（Gin + GORM + MySQL），负责链下用户身份、提案、投票记录等数据
- 区块链：Solidity 智能合约，使用 Hardhat 在 Ganache（`http://127.0.0.1:8545`）上部署
- 前端：Vue 3 + TypeScript + Vite + Pinia + Vue Router + Ethers.js，与 MetaMask 和后端 / 合约交互

### 一、后端（Go）—— `backend`

#### 1. 运行前准备

1. 启动本地 MySQL，并确保：
   - 地址：`127.0.0.1`
   - 端口：`3306`
   - 用户：`root`
   - 密码：`123456`
2. 不需要手动建库，后端会自动在 MySQL 中创建数据库：`smart_community`。
3. （可选）启用“自动结算 resolve”后台任务时，需要配置合约地址与管理员私钥（不明文保存）。

#### 2. 启动后端

```bash
cd backend
go run .
```

#### 3. 配置自动结算（不明文保存私钥）

后端支持自动扫描到期提案并用管理员账户自动发送 `resolve(proposalId)` 交易（消耗 Gas 由该管理员账户承担）。

该功能使用 Windows DPAPI 将私钥加密后写入 `backend/.env`，**`.env` 中不会出现明文私钥**。

在项目根目录执行（PowerShell）：

```powershell
cd d:\code\work
go run .\backend\cmd\secureconfig set --rpc http://127.0.0.1:8545 --address 0x你的合约地址 --pk 0x你的管理员私钥
```

写入后会生成/更新 `backend/.env`：

- `SC_RPC_URL`
- `SC_GOVERNOR_ADDRESS`
- `SC_ADMIN_PRIVATE_KEY_ENC`（DPAPI 加密后的 base64 密文）

之后正常启动后端 `go run .` 即可启用自动结算任务。

默认监听：`http://127.0.0.1:8080`

主要接口示例：

- `GET /api/health`：健康检查
- `POST /api/identity/register`：业主提交实名认证信息（链下，SRS_USRM01.02）
- `GET /api/identity/me`：根据 `X-Wallet-Addr` 查询当前钱包身份状态
- `GET /api/admin/identity/pending`：管理员查看待审核用户列表
- `POST /api/admin/identity/:id/approve`：审核通过（链下），设置投票权重
- `POST /api/admin/identity/:id/reject`：审核驳回
- `POST /api/proposals`：创建提案的链下部分（存储标题、详情，生成 `ContentHash`）
- `GET /api/proposals` / `GET /api/proposals/:id`：查询提案列表与详情
- `POST /api/proposals/:id/votes` / `GET /api/proposals/:id/votes`：投票记录的链下缓存

### 二、智能合约 + Hardhat —— `blockchain`

#### 1. 目录结构

- `contracts/SmartCommunityGovernor.sol`：核心协同决策合约
- `hardhat.config.ts`：Hardhat 配置（已配置 Ganache 网络和你提供的 4 个私钥）
- `scripts/deploy.ts`：部署脚本（默认最小公示期 3 天，通过阈值 51%）

合约实现功能对应需求：

- 白名单与权重管理（SRS_USRM01.03）：
  - `setVoter(address voter, uint256 weight, bool auth)`
- 提案管理（SRS_PROP02）：
  - `createProposal(bytes32 contentHash, uint8 propType, uint64 deadline)`
- 投票与自动裁决（SRS_VOTE03）：
  - `vote(uint256 proposalId, uint8 choice)`
  - `resolve(uint256 proposalId)`
  - 事件：`ProposalCreated`、`VoteCast`、`ProposalResolved`

#### 2. 编译与部署

确保 Ganache 已在 `http://127.0.0.1:8545` 启动，并使用你提供的私钥对应账户有足够测试 ETH。

```bash
cd blockchain
npx hardhat compile
npx hardhat run scripts/deploy.ts --network ganache
```

部署完成后命令行会输出合约地址，例如：

```text
SmartCommunityGovernor deployed to: 0xABC...
```

记下该地址，稍后需要配置到前端。

### 三、前端（Vue3）—— `frontend`

#### 1. 安装依赖与启动

```bash
cd frontend
npm install
npm run dev
```

默认访问：`http://localhost:5173`（以 Vite 输出为准）

请使用 Chrome / Edge，并安装 MetaMask 插件；在 MetaMask 中切换到 Ganache 对应的本地网络，并导入你提供的测试账户私钥。

#### 2. 核心页面与流程映射

- 首页 `HomePage.vue`：
  - 提供“身份认证”、“查看提案”入口，符合 8.1 节界面友好要求。

- 身份认证页面 `IdentityPage.vue`：
  - 使用 MetaMask 连接（在全局头部），提交真实姓名、身份证后四位、房屋信息、电话等（SRS_USRM01.02）。
  - 调用后端：
    - `POST /api/identity/register`
    - `GET /api/identity/me` 查询认证状态（审核中 / 已认证 / 被驳回）。

- 提案列表页面 `ProposalListPage.vue`：
  - 调用 `GET /api/proposals`，卡片式展示提案标题、发起人、截止时间、状态（进行中 / 已通过 / 已驳回）（SRS_PROP02.02）。

- 提案详情页面 `ProposalDetailPage.vue`：
  - 调用后端 `GET /api/proposals/:id` 获取链下文本详情。
  - 使用 Ethers.js 通过 MetaMask 调用合约：
    - `getProposal` 读取链上状态（票数、状态），用于结果公示（SRS_VOTE03.02 / SYS3-01）。
    - `vote` 对当前提案进行投票（SRS_VOTE03.01）。
  - 投票成功后，将交易哈希等信息通过 `POST /api/proposals/:id/votes` 写入链下缓存。
  - 当前代码中合约地址常量为占位：`0x0000...0000`，**需要你在部署后替换为真实地址**。

- 管理后台 `AdminPage.vue`：
  - 调用 `GET /api/admin/identity/pending` 获取待审核用户列表。
  - 管理员使用 MetaMask 调用合约 `setVoter` 将用户地址和权重写入白名单（SRS_USRM01.03 / SYS1-04）。
  - 后端接口：
    - `POST /api/admin/identity/:id/approve`：更新链下认证状态及权重
    - `POST /api/admin/identity/:id/reject`：驳回并记录原因

#### 3. 前端与合约地址配置

部署合约后，请在以下两个文件中将占位地址替换为实际地址：

- `src/views/ProposalDetailPage.vue` 中的 `CONTRACT_ADDRESS`
- `src/views/AdminPage.vue` 中的 `CONTRACT_ADDRESS`

同时可以根据需要，增加一个统一的配置文件（例如 `src/config.ts`）来集中管理 RPC / 合约地址等。

### 四、整体联调顺序建议

1. 启动 Ganache（`http://127.0.0.1:8545`），确认包含你给出的 4 个地址及私钥。
2. 在 `blockchain` 目录编译并部署 `SmartCommunityGovernor` 合约，记录部署地址。
3. 替换前端中两个页面的 `CONTRACT_ADDRESS` 常量并重新启动前端。
4. 启动后端：
   - 确保 MySQL 已运行且账号密码为 `root/123456`。
   - 运行 `go run .`，自动建库建表。
5. 前端中：
   - 使用普通用户钱包连接，进入“身份认证”页面提交身份信息。
   - 使用管理员钱包连接，进入“管理后台”页面审核通过并上链白名单。
   - 已认证用户可以“发起提案”（你可以在现有 API 基础上补充前端页面），再在“提案详情”中发起投票。
   - 查看投票实时票数和最终裁决结果。

### 五、后续可扩展点（按需求文档可继续完善）

- 增加“发起提案”专用页面（调用 `/api/proposals` + 合约 `createProposal`），补齐 SRS_PROP02.01 的 UI。
- 在前端监听合约事件 `ProposalResolved`，实时刷新提案状态和结果图表（饼图 / 柱状图）。
- 增加系统参数配置界面，对接合约 `setConfig`（SRS_ADMN04.01）。
- 对接合约暂停功能（`setPaused`），在前端展示“系统维护中”横幅（SRS_ADMN04.02）。

目前项目已经为你搭好了从 **Go 后端 + MySQL** 到 **Solidity 智能合约 + Ganache**，再到 **Vue 前端 + MetaMask + Ethers.js** 的完整骨架，你可以在此基础上根据答辩需要继续细化和美化。 

