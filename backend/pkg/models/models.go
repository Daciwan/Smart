package models

import "time"

// User 对应 DS-1 UserInfo
type User struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	WalletAddr string    `gorm:"type:char(42);uniqueIndex;not null" json:"walletAddr"`
	RealName   string    `gorm:"type:varchar(20);not null" json:"realName"`
	IDCard4    string    `gorm:"type:char(4);not null" json:"idCard4"`
	BuildNo    string    `gorm:"type:varchar(10);not null" json:"buildNo"`
	UnitNo     string    `gorm:"type:varchar(10);not null" json:"unitNo"`
	RoomNo     string    `gorm:"type:varchar(10);not null" json:"roomNo"`
	HouseArea  float64   `gorm:"type:decimal(10,2);not null" json:"houseArea"`
	PhoneNo    string    `gorm:"type:char(11);not null" json:"phoneNo"`
	AuthStatus int       `gorm:"type:int;default:0;not null" json:"authStatus"` // 0:审核中 1:已认证 2:被驳回
	VoteWeight float64   `gorm:"type:decimal(10,2);default:0" json:"voteWeight"`
	Remark     string    `gorm:"type:varchar(255)" json:"remark"` // 审核驳回理由等
	RegTime    time.Time `gorm:"autoCreateTime" json:"regTime"`
}

// Proposal 对应 DS-2 Proposal
type Proposal struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	PropID      uint      `gorm:"uniqueIndex;not null" json:"propId"` // 链上 ProposalID
	PropTitle   string    `gorm:"type:varchar(50);not null" json:"propTitle"`
	PropDesc    string    `gorm:"type:text;not null" json:"propDesc"`
	ContentHash string    `gorm:"type:char(66);not null" json:"contentHash"`
	PropType    int       `gorm:"type:int;not null" json:"propType"` // 0:一人一票 1:面积加权
	CreatorAddr string    `gorm:"type:char(42);not null" json:"creatorAddr"`
	CreateTime  time.Time `gorm:"autoCreateTime" json:"createTime"`
	Deadline    time.Time `gorm:"not null" json:"deadline"`
	PropStatus  int       `gorm:"type:int;default:0;not null" json:"propStatus"` // 0:进行中 1:已通过 2:已驳回
	TxHash      string    `gorm:"type:char(66)" json:"txHash"`
}

// VoteRecord 对应 DS-3 VoteRecord
type VoteRecord struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	VoteTxHash string    `gorm:"type:char(66);uniqueIndex;not null" json:"voteTxHash"`
	PropID     uint      `gorm:"index;not null" json:"propId"`
	VoterAddr  string    `gorm:"type:char(42);not null" json:"voterAddr"`
	VoteChoice int       `gorm:"type:int;not null" json:"voteChoice"` // 1:赞成,2:反对,3:弃权
	VoteWeight float64   `gorm:"type:decimal(10,2);not null" json:"voteWeight"`
	VoteTime   time.Time `gorm:"autoCreateTime" json:"voteTime"`
}

// SysConfig 对应 DS-4 SysConfig
type SysConfig struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	ParamName  string    `gorm:"type:varchar(30);not null" json:"paramName"`
	ParamValue string    `gorm:"type:varchar(64);not null" json:"paramValue"`
	UpdateTime time.Time `gorm:"autoUpdateTime" json:"updateTime"`
	AdminAddr  string    `gorm:"type:char(42);not null" json:"adminAddr"`
}

