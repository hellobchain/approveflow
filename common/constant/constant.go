package constant

// 审批状态
type ApprovalStatus string

const (
	StatusPending   ApprovalStatus = "pending"
	StatusApproved  ApprovalStatus = "approved"
	StatusRejected  ApprovalStatus = "rejected"
	StatusCancelled ApprovalStatus = "cancelled"
	StatusExpired   ApprovalStatus = "expired"
)

// HistoryAction 历史操作类型
type HistoryAction string

const (
	ActionCreate  HistoryAction = "create"
	ActionApprove HistoryAction = "approve"
	ActionReject  HistoryAction = "reject"
	ActionCancel  HistoryAction = "cancel"
	ActionComment HistoryAction = "comment"
	ActionSystem  HistoryAction = "system"
)
