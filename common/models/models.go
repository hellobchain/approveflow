package models

import (
	"time"

	"github.com/hellobchain/approveflow/common/constant"
)

// 审批级别
type ApprovalLevel struct {
	Level      int
	Approvers  []string
	CurrentIdx int // 当前正在审批的级别索引
}

// 审批请求
type ApprovalRequest struct {
	ID                string
	BussinessKey      string
	Category          string
	Title             string
	Description       string
	Requester         string
	Status            constant.ApprovalStatus
	Levels            []ApprovalLevel
	CurrentLevel      int
	Comments          []Comment
	CreatedAt         time.Time
	UpdatedAt         time.Time
	ExpiresAt         time.Time
	ApprovalDecisions []ApprovalDecision
}

// 审批意见
type Comment struct {
	BussinessKey string
	Category     string
	User         string
	Message      string
	Timestamp    time.Time
}

// ApprovalDecision 表示单个审批人的决定
type ApprovalDecision struct {
	RequestID    string
	Level        int
	Approver     string
	Status       constant.ApprovalStatus // "approved" 或 "rejected"
	Comment      string
	DecisionTime time.Time
}

// ApprovalHistory 审批历史记录
type ApprovalHistory struct {
	ID           int
	RequestID    string
	BussinessKey string
	Category     string
	Operator     string
	Action       string // create, approve, reject, cancel, comment, system
	FromStatus   constant.ApprovalStatus
	ToStatus     constant.ApprovalStatus
	Comment      string
	CreatedAt    time.Time
}
