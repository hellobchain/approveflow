package eventer

import "github.com/hellobchain/approveflow/common/models"

type Eventer interface {
	EndEvent(request *models.ApprovalRequest) error
	StartEvent(request *models.ApprovalRequest) error
}
