package eventer

import (
	"github.com/hellobchain/approveflow/common/constant"
	"github.com/hellobchain/approveflow/common/models"
)

type Eventer interface {
	EndEvent(request *models.ApprovalRequest, oprateType constant.OperateType) error
	StartEvent(request *models.ApprovalRequest, oprateType constant.OperateType) error
}
