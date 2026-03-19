// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/// @title 基于区块链的智慧社区协同决策合约
/// @notice 支持白名单管理、提案创建、多模式投票和自动裁决
contract SmartCommunityGovernor {
    struct Voter {
        bool isAuth;
        uint256 weight; // 面积或 1
    }

    struct Proposal {
        uint256 id;
        bytes32 contentHash; // 对应链下提案详情哈希
        address creator;
        uint8 propType; // 0: 一人一票; 1: 面积加权
        uint64 startTime;
        uint64 deadline;
        uint256 yesVotes;
        uint256 noVotes;
        uint256 abstainVotes;
        uint8 status; // 0: 进行中; 1: 已通过; 2: 已驳回
        bool tallied;
    }

    address public owner;
    bool public paused;

    // 全局治理参数
    uint64 public minDuration; // 最小公示时长（秒）
    uint8 public passPercentage; // 通过阈值百分比（1-100）

    // 提案列表（从 ID=1 开始）
    Proposal[] private _proposals;

    mapping(address => Voter) public whitelist;
    mapping(uint256 => mapping(address => bool)) public hasVoted;

    event OwnershipTransferred(address indexed previousOwner, address indexed newOwner);
    event Paused(address indexed account);
    event Unpaused(address indexed account);
    event ConfigChanged(uint64 minDuration, uint8 passPercentage);
    event VoterWhitelisted(address indexed voter, uint256 weight);
    event VoterRemoved(address indexed voter);
    event ProposalCreated(uint256 indexed proposalId, bytes32 contentHash, uint8 propType, uint64 deadline, address indexed creator);
    event VoteCast(uint256 indexed proposalId, address indexed voter, uint8 choice, uint256 weight);
    event ProposalResolved(uint256 indexed proposalId, uint8 status, uint256 yesVotes, uint256 noVotes, uint256 abstainVotes);

    modifier onlyOwner() {
        require(msg.sender == owner, "not owner");
        _;
    }

    modifier whenNotPaused() {
        require(!paused, "paused");
        _;
    }

    constructor(uint64 _minDuration, uint8 _passPercentage) {
        require(_passPercentage > 0 && _passPercentage <= 100, "invalid passPercentage");
        owner = msg.sender;
        minDuration = _minDuration;
        passPercentage = _passPercentage;

        // 占位，使真实提案从 ID=1 开始
        _proposals.push(
            Proposal({
                id: 0,
                contentHash: bytes32(0),
                creator: address(0),
                propType: 0,
                startTime: 0,
                deadline: 0,
                yesVotes: 0,
                noVotes: 0,
                abstainVotes: 0,
                status: 2,
                tallied: true
            })
        );
    }

    // ============ 权限与配置 ============

    function transferOwnership(address newOwner) external onlyOwner {
        require(newOwner != address(0), "zero address");
        emit OwnershipTransferred(owner, newOwner);
        owner = newOwner;
    }

    function setConfig(uint64 _minDuration, uint8 _passPercentage) external onlyOwner {
        require(_passPercentage > 0 && _passPercentage <= 100, "invalid passPercentage");
        minDuration = _minDuration;
        passPercentage = _passPercentage;
        emit ConfigChanged(_minDuration, _passPercentage);
    }

    function setPaused(bool value) external onlyOwner {
        paused = value;
        if (value) {
            emit Paused(msg.sender);
        } else {
            emit Unpaused(msg.sender);
        }
    }

    // ============ 白名单管理 ============

    function setVoter(address voter, uint256 weight, bool auth) external onlyOwner whenNotPaused {
        require(voter != address(0), "zero address");
        // auth=true 时必须有权重；auth=false（移除）允许 weight=0
        if (auth) {
            require(weight > 0, "weight zero");
        }

        whitelist[voter] = Voter({isAuth: auth, weight: weight});

        if (auth) {
            emit VoterWhitelisted(voter, weight);
        } else {
            emit VoterRemoved(voter);
        }
    }

    /// @notice 移除白名单用户（链上），同时清空其权重
    /// @dev 该函数用于管理员“移除白名单”操作，避免传入 weight=0 触发校验问题
    function removeVoter(address voter) external onlyOwner whenNotPaused {
        require(voter != address(0), "zero address");
        whitelist[voter] = Voter({isAuth: false, weight: 0});
        emit VoterRemoved(voter);
    }

    // ============ 提案与投票 ============

    function createProposal(bytes32 contentHash, uint8 propType, uint64 deadline) external whenNotPaused returns (uint256) {
        require(deadline > block.timestamp, "deadline in past");
        require(deadline >= block.timestamp + minDuration, "too short duration");
        require(propType == 0 || propType == 1, "invalid propType");

        uint256 newId = _proposals.length;
        _proposals.push(
            Proposal({
                id: newId,
                contentHash: contentHash,
                creator: msg.sender,
                propType: propType,
                startTime: uint64(block.timestamp),
                deadline: deadline,
                yesVotes: 0,
                noVotes: 0,
                abstainVotes: 0,
                status: 0,
                tallied: false
            })
        );

        emit ProposalCreated(newId, contentHash, propType, deadline, msg.sender);
        return newId;
    }

    /// @notice 对提案进行投票
    /// @param proposalId 提案 ID（从 1 开始）
    /// @param choice 1:赞成, 2:反对, 3:弃权
    function vote(uint256 proposalId, uint8 choice) external whenNotPaused {
        require(proposalId > 0 && proposalId < _proposals.length, "invalid id");
        require(choice == 1 || choice == 2 || choice == 3, "invalid choice");

        Proposal storage p = _proposals[proposalId];
        require(!p.tallied, "already tallied");
        require(block.timestamp < p.deadline, "voting closed");

        Voter memory v = whitelist[msg.sender];
        require(v.isAuth, "not in whitelist");
        require(!hasVoted[proposalId][msg.sender], "already voted");

        uint256 weight = (p.propType == 0) ? 1 : v.weight;
        require(weight > 0, "zero weight");

        hasVoted[proposalId][msg.sender] = true;

        if (choice == 1) {
            p.yesVotes += weight;
        } else if (choice == 2) {
            p.noVotes += weight;
        } else {
            p.abstainVotes += weight;
        }

        emit VoteCast(proposalId, msg.sender, choice, weight);
    }

    /// @notice 结算提案，自动裁决通过/未通过
    /// @dev 任意用户可以调用，用于触发链上状态变更
    function resolve(uint256 proposalId) public whenNotPaused {
        require(proposalId > 0 && proposalId < _proposals.length, "invalid id");

        Proposal storage p = _proposals[proposalId];
        require(!p.tallied, "already tallied");

        uint256 totalVotes = p.yesVotes + p.noVotes + p.abstainVotes;

        // 到达截止时间或赞成票提前达到绝对多数
        require(
            block.timestamp >= p.deadline ||
                (totalVotes > 0 && (p.yesVotes * 100) / totalVotes >= passPercentage),
            "cannot resolve yet"
        );

        uint8 finalStatus;
        if (totalVotes == 0) {
            // 无人投票视为未通过
            finalStatus = 2;
        } else {
            uint256 yesRate = (p.yesVotes * 100) / totalVotes;
            if (yesRate >= passPercentage) {
                finalStatus = 1;
            } else {
                finalStatus = 2;
            }
        }

        p.status = finalStatus;
        p.tallied = true;

        emit ProposalResolved(proposalId, finalStatus, p.yesVotes, p.noVotes, p.abstainVotes);
    }

    // ============ 只读辅助函数 ============

    function proposalCount() external view returns (uint256) {
        return _proposals.length - 1;
    }

    function getProposal(uint256 proposalId) external view returns (Proposal memory) {
        require(proposalId > 0 && proposalId < _proposals.length, "invalid id");
        return _proposals[proposalId];
    }

    function listProposals(uint256 offset, uint256 limit) external view returns (Proposal[] memory) {
        uint256 total = _proposals.length - 1;
        if (offset >= total) {
            return new Proposal[](0);
        }

        uint256 end = offset + limit;
        if (end > total) {
            end = total;
        }

        uint256 size = end - offset;
        Proposal[] memory result = new Proposal[](size);
        for (uint256 i = 0; i < size; i++) {
            result[i] = _proposals[offset + 1 + i];
        }
        return result;
    }
}

