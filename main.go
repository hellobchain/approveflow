package main

import (
	"fmt"
	"time"

	"github.com/hellobchain/approveflow/common/constant"
	"github.com/hellobchain/approveflow/common/models"
	"github.com/hellobchain/approveflow/core/approve"
	"github.com/hellobchain/approveflow/core/loggers"
	"github.com/hellobchain/approveflow/pkg/uuid"
	"github.com/hellobchain/wswlog/wlogging"
)

var logger = wlogging.MustGetFileLoggerWithoutName(loggers.LogConfig)

// 邮件通知实现
type EmailNotifier struct{}

func (en EmailNotifier) SendNotification(to, subject, message string) error {
	logger.Infof("发送邮件给 %s: 主题=%s, 内容=%s\n", to, subject, message)
	return nil
}

type EmailEventer struct{}

func (ee EmailEventer) EndEvent(request *models.ApprovalRequest, oprateType constant.OperateType) error {
	logger.Infof("结束事件: %s, 状态: %s\n", request.ID, request.Status)
	return nil
}
func (ee EmailEventer) StartEvent(request *models.ApprovalRequest, oprateType constant.OperateType) error {
	logger.Infof("开始事件: %s, 状态: %s\n", request.ID, request.Status)
	return nil
}

func main() {
	// MySQL 连接配置
	dsn := "approve:approve@tcp(127.0.0.1:3306)/approval_system?parseTime=true"
	db, err := approve.NewDb(dsn)
	if err != nil {
		logger.Fatal(fmt.Errorf("连接数据库失败: %w", err))
	}
	defer db.Close()
	// 初始化通知器
	notifier := EmailNotifier{}
	// 初始化事件处理器
	eventer := EmailEventer{}
	for i := 0; i < 200; i++ {
		// 初始化审批系统
		system := approve.NewApprovalSystem(approve.DB(db), approve.Notifier(notifier), approve.Eventer(eventer))
		// 定期检查过期请求
		go func() {
			for {
				time.Sleep(1 * time.Hour)
				if err := system.ProcessExpiredRequests(); err != nil {
					logger.Infof("处理过期请求失败: %v", err)
				}
			}
		}()

		bussinessKey := uuid.GetPureUUID()
		category := "年度预算审批"
		// 示例使用
		logger.Infof("创建多级审批请求...")
		request, err := system.CreateRequest(
			bussinessKey,
			category,
			"年度预算审批",
			"2025年度部门预算计划",
			"财务部-张三",
			[][]string{
				{"部门经理-李四"},          // 第一级审批人
				{"财务总监-王五"},          // 第二级审批人
				{"CEO-赵六", "COO-钱七"}, // 第三级审批人 (需要多人审批)
			},
			72*time.Hour, // 3天内有效
		)
		if err != nil {
			logger.Fatal(err)
		}
		logger.Infof("创建请求成功: %+v\n", request)

		// 第一级审批
		logger.Infof("\n第一级审批...")
		err = system.Approve(request.ID, "部门经理-李四", "预算合理，同意")
		if err != nil {
			logger.Fatal(err)
		}
		logger.Infof("部门经理-李四审批通过")

		// 第二级审批
		logger.Infof("\n第二级审批...")
		err = system.Approve(request.ID, "财务总监-王五", "符合公司财务政策")
		if err != nil {
			logger.Fatal(err)
		}
		logger.Infof("财务总监-王五审批通过")

		// 第三级审批 (需要多人审批)
		logger.Infof("\n第三级审批...")
		err = system.Approve(request.ID, "CEO-赵六", "战略规划相符")
		if err != nil {
			logger.Fatal(err)
		}
		logger.Infof("CEO-赵六审批通过")

		err = system.Approve(request.ID, "COO-钱七", "运营需求匹配")
		if err != nil {
			logger.Fatal(err)
		}
		logger.Infof("COO-钱七审批通过")

		comments, err := system.GetCommentsByBussinessKeyAndCategory(bussinessKey, category)
		if err != nil {
			logger.Fatal(err)
		}
		logger.Infof("审批意见:")
		for _, comment := range comments {
			logger.Infof("- %s: %s (%s)\n", comment.User, comment.Message, comment.Timestamp.Format("2006-01-02 15:04"))
		}
		// 历史记录
		history, err := system.GetBussinessKeyAndCategoryHistory(request.BussinessKey, request.Category)
		if err != nil {
			logger.Fatal(err)
		}
		logger.Infof("审批历史记录:")
		for _, record := range history {
			logger.Infof("- %s: %s -> %s (%s) by %s: %s\n", record.Action, record.FromStatus, record.ToStatus, record.CreatedAt.Format("2006-01-02 15:04"), record.Operator, record.Comment)
		}
		fmt.Println("多级审批流程演示完成！请查看日志获取详细信息。")
	}
}
