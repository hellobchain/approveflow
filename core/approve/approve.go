package approve

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/hellobchain/approveflow/common/constant"
	"github.com/hellobchain/approveflow/common/models"
	"github.com/hellobchain/approveflow/core/eventer"
	"github.com/hellobchain/approveflow/core/loggers"
	"github.com/hellobchain/approveflow/core/notifier"
	"github.com/hellobchain/approveflow/pkg/uuid"
	"github.com/hellobchain/wswlog/wlogging"
)

var logger = wlogging.MustGetFileLoggerWithoutName(loggers.LogConfig)

// 审批系统
type ApprovalSystem struct {
	db       *sql.DB
	notifier notifier.Notifier
	eventer  eventer.Eventer // 事件处理器
}

// DB 设置数据库连接
func DB(db *sql.DB) func(*ApprovalSystem) {
	return func(as *ApprovalSystem) {
		as.db = db
	}
}

// Notifier 设置通知器
func Notifier(notifier notifier.Notifier) func(*ApprovalSystem) {
	return func(as *ApprovalSystem) {
		as.notifier = notifier
	}
}

// Eventer 设置事件处理器
func Eventer(eventer eventer.Eventer) func(*ApprovalSystem) {
	return func(as *ApprovalSystem) {
		as.eventer = eventer
	}
}

// GetDb 获取数据库连接
func (as *ApprovalSystem) GetDb() *sql.DB {
	return as.db
}

// GetNotifier 获取通知器
func (as *ApprovalSystem) GetNotifier() notifier.Notifier {
	return as.notifier
}

// GetEventer 获取事件处理器
func (as *ApprovalSystem) GetEventer() eventer.Eventer {
	return as.eventer
}

// 创建新的审批系统
func NewApprovalSystem(opts ...func(*ApprovalSystem)) *ApprovalSystem {
	approvalSystem := &ApprovalSystem{
		db:       nil,
		notifier: nil,
		eventer:  nil,
	}
	for _, opt := range opts {
		opt(approvalSystem)
	}
	approvalSystem.initTables()
	return approvalSystem
}

func (as *ApprovalSystem) initTables() error {
	if as.db == nil {
		return errors.New("database connection is not set")
	}
	if as.notifier == nil {
		return errors.New("notifier is not set")
	}
	if as.eventer == nil {
		return errors.New("eventer is not set")
	}
	db := as.db
	// 创建表
	_, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS requests (
		id VARCHAR(64) PRIMARY KEY,
		bussiness_key VARCHAR(64) NOT NULL,
		category VARCHAR(64) NOT NULL,
		title VARCHAR(255) NOT NULL,
		description TEXT,
		requester VARCHAR(100) NOT NULL,
		status ENUM('pending', 'approved', 'rejected', 'cancelled', 'expired') NOT NULL DEFAULT 'pending',
		current_level INT NOT NULL DEFAULT 1,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		expires_at DATETIME NOT NULL,
		INDEX idx_status (status),
		INDEX idx_requester (requester),
		INDEX idx_expires (expires_at)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS levels (
		request_id VARCHAR(64) NOT NULL,
		level INT NOT NULL,
		approver VARCHAR(100) NOT NULL,
		PRIMARY KEY (request_id, level, approver),
		FOREIGN KEY (request_id) REFERENCES requests(id) ON DELETE CASCADE,
		INDEX idx_level (level)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
	`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS comments (
		id INT AUTO_INCREMENT PRIMARY KEY,
		request_id VARCHAR(64) NOT NULL,
		bussiness_key VARCHAR(64) NOT NULL,
	    category VARCHAR(64) NOT NULL,
		user VARCHAR(100) NOT NULL,
		message TEXT NOT NULL,
		timestamp DATETIME NOT NULL,
		FOREIGN KEY (request_id) REFERENCES requests(id) ON DELETE CASCADE,
		INDEX idx_request (request_id),
		INDEX idx_user (user),
		INDEX idx_timestamp (timestamp)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
	`)
	if err != nil {
		return err
	}

	// 在NewApprovalSystem函数中添加新表
	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS approvals (
		request_id VARCHAR(36) NOT NULL,
		level INT NOT NULL,
		approver VARCHAR(100) NOT NULL,
		status ENUM('approved', 'rejected') NOT NULL,
		comment TEXT,
		decision_time DATETIME NOT NULL,
		PRIMARY KEY (request_id, level, approver),
		FOREIGN KEY (request_id) REFERENCES requests(id) ON DELETE CASCADE,
		INDEX idx_approver (approver)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS approval_history (
		id INT AUTO_INCREMENT PRIMARY KEY,
		request_id VARCHAR(36) NOT NULL,
		bussiness_key VARCHAR(64) NOT NULL,
	    category VARCHAR(64) NOT NULL,
		operator VARCHAR(100) NOT NULL,
		action ENUM('create', 'approve', 'reject', 'cancel', 'comment', 'system') NOT NULL,
		from_status ENUM('', 'pending', 'approved', 'rejected', 'cancelled', 'expired') NULL,
		to_status ENUM('', 'pending', 'approved', 'rejected', 'cancelled', 'expired') NULL,
		comment TEXT NULL,
		created_at DATETIME NOT NULL,
		-- FOREIGN KEY (request_id) REFERENCES requests(id) ON DELETE CASCADE,
		INDEX idx_request (request_id),
		INDEX idx_operator (operator),
		INDEX idx_created (created_at)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
`)
	if err != nil {
		return err
	}
	return nil
}

// 创建新的审批系统
func NewApprovalSystemWithDb(db *sql.DB, notifier notifier.Notifier, eventer eventer.Eventer) (*ApprovalSystem, error) {
	approvalSystem := &ApprovalSystem{
		db:       db,
		notifier: notifier,
		eventer:  eventer,
	}
	err := approvalSystem.initTables()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize tables: %w", err)
	}
	return approvalSystem, nil
}

func NewDb(dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	// 测试数据库连接
	if err = db.Ping(); err != nil {
		return nil, err
	}
	return db, nil
}

// 创建新的审批系统
func NewApprovalSystemWithDsn(dsn string, notifier notifier.Notifier, eventer eventer.Eventer) (*ApprovalSystem, error) {
	db, err := NewDb(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	return NewApprovalSystemWithDb(db, notifier, eventer)
}

// 创建审批请求
func (as *ApprovalSystem) CreateRequest(bussinessKey, category, title, description, requester string, levels [][]string, expiresIn time.Duration) (*models.ApprovalRequest, error) {
	// 检查请求bussinessKey是否已存在
	var exists bool
	err := as.db.QueryRow("SELECT EXISTS(SELECT 1 FROM requests WHERE bussiness_key = ? AND category = ?)", bussinessKey, category).Scan(&exists)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errors.New("request bussinessKey already exists")
	}

	// 准备审批级别
	approvalLevels := make([]models.ApprovalLevel, len(levels))
	for i, approvers := range levels {
		approvalLevels[i] = models.ApprovalLevel{
			Level:     i + 1,
			Approvers: approvers,
		}
	}

	now := time.Now()
	request := &models.ApprovalRequest{
		ID:           uuid.GetUUID(),
		BussinessKey: bussinessKey,
		Category:     category,
		Title:        title,
		Description:  description,
		Requester:    requester,
		Status:       constant.StatusPending,
		Levels:       approvalLevels,
		CurrentLevel: 1,
		CreatedAt:    now,
		UpdatedAt:    now,
		ExpiresAt:    now.Add(expiresIn),
	}

	// 开启事务
	tx, err := as.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// 插入请求基本信息
	_, err = tx.Exec(`
		INSERT INTO requests (id, bussiness_key, category, title, description, requester, status, current_level, created_at, updated_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		request.ID, request.BussinessKey, request.Category, request.Title, request.Description, request.Requester, string(request.Status),
		request.CurrentLevel, request.CreatedAt, request.UpdatedAt, request.ExpiresAt)
	if err != nil {
		return nil, err
	}

	// 插入审批级别信息
	for _, level := range request.Levels {
		for _, approver := range level.Approvers {
			_, err = tx.Exec(`
				INSERT INTO levels (request_id, level, approver)
				VALUES (?, ?, ?)`,
				request.ID, level.Level, approver)
			if err != nil {
				return nil, err
			}
		}
	}

	// 提交事务
	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	// 通知第一级审批人
	for _, approver := range request.Levels[0].Approvers {
		as.notifier.SendNotification(approver, "新的审批请求",
			fmt.Sprintf("您有一个新的审批请求: %s\n描述: %s", request.Title, request.Description))
	}

	// 添加创建记录
	err = as.addHistoryRecord(request.ID, request.BussinessKey, request.Category, requester, constant.ActionCreate, "", constant.StatusPending, "创建审批请求")
	if err != nil {
		return nil, err
	}

	return request, nil
}

// 审批通过
func (as *ApprovalSystem) Approve(requestID, approver, comment string) error {
	// 获取请求
	request, err := as.getRequest(requestID)
	if err != nil {
		return err
	}
	oldStatus := request.Status
	// 检查状态
	if request.Status != constant.StatusPending {
		return fmt.Errorf("request is already %s", request.Status)
	}

	// 检查是否过期
	if time.Now().After(request.ExpiresAt) {
		request.Status = constant.StatusExpired
		as.updateRequestStatus(request)
		return errors.New("request has expired")
	}

	// 检查当前审批级别
	currentLevel := request.Levels[request.CurrentLevel-1]

	// 检查审批人是否有权限
	hasPermission := false
	for _, a := range currentLevel.Approvers {
		if a == approver {
			hasPermission = true
			break
		}
	}
	if !hasPermission {
		return errors.New("approver not authorized for current level")
	}

	// 检查是否已经审批过
	var existingStatus string
	err = as.db.QueryRow(`
        SELECT status FROM approvals 
        WHERE request_id = ? AND level = ? AND approver = ?`,
		requestID, request.CurrentLevel, approver).Scan(&existingStatus)

	if err == nil {
		return fmt.Errorf("approver has already %s this request", existingStatus)
	} else if err != sql.ErrNoRows {
		return err
	}

	// 记录审批决定 - 使用结构体
	decision := models.ApprovalDecision{
		RequestID:    requestID,
		Level:        request.CurrentLevel,
		Approver:     approver,
		Status:       constant.StatusApproved,
		Comment:      comment,
		DecisionTime: time.Now(),
	}

	_, err = as.db.Exec(`
        INSERT INTO approvals (request_id, level, approver, status, comment, decision_time)
        VALUES (?, ?, ?, ?, ?, ?)`,
		decision.RequestID, decision.Level, decision.Approver,
		string(decision.Status), decision.Comment, decision.DecisionTime)
	if err != nil {
		return err
	}

	// 检查是否所有当前级别的审批人都已批准
	allApproved, err := as.checkAllApproversApproved(requestID, request.CurrentLevel)
	if err != nil {
		return err
	}

	if allApproved {
		// 移动到下一级或完成审批
		if request.CurrentLevel < len(request.Levels) {
			request.CurrentLevel++
			_, err = as.db.Exec("UPDATE requests SET current_level = ?, updated_at = ? WHERE id = ?",
				request.CurrentLevel, time.Now(), requestID)
			if err != nil {
				return err
			}

			// 通知下一级审批人
			nextLevel := request.Levels[request.CurrentLevel-1]
			for _, nextApprover := range nextLevel.Approvers {
				as.notifier.SendNotification(nextApprover, "新的审批请求",
					fmt.Sprintf("您有一个新的审批请求需要处理: %s", request.Title))
			}
			// 添加审批记录
			err = as.addHistoryRecord(requestID, request.BussinessKey, request.Category, approver, constant.ActionApprove, oldStatus, request.Status, comment)
			if err != nil {
				return err
			}
			// 添加审批意见
			if comment != "" {
				err = as.addComment(requestID, approver, comment, request.BussinessKey, request.Category)
				if err != nil {
					return err
				}
			}
		} else {
			// 所有级别都已完成
			request.Status = constant.StatusApproved
			err = as.updateRequestStatus(request)
			if err != nil {
				return err
			}
			// 添加审批记录
			err = as.addHistoryRecord(requestID, request.BussinessKey, request.Category, approver, constant.ActionSystem, oldStatus, request.Status, comment)
			if err != nil {
				return err
			}
			// 添加审批意见
			if comment != "" {
				err = as.addComment(requestID, approver, comment, request.BussinessKey, request.Category)
				if err != nil {
					return err
				}
			}
			// 结束事件
			as.eventer.EndEvent(request, constant.OperateTypeApprove)
			// 删除请求状态
			err := as.deleteRequestStatus(requestID)
			if err != nil {
				return err
			}
			// 通知请求人
			as.notifier.SendNotification(request.Requester, "审批完成",
				fmt.Sprintf("您的请求 '%s' 已获得批准", request.Title))
		}
	}

	return nil
}

// 检查当前级别的所有审批人是否都已批准
func (as *ApprovalSystem) checkAllApproversApproved(requestID string, level int) (bool, error) {
	// 获取当前级别的所有审批人
	var approvers []string
	rows, err := as.db.Query(`
        SELECT approver FROM levels 
        WHERE request_id = ? AND level = ?`,
		requestID, level)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var approver string
		if err := rows.Scan(&approver); err != nil {
			return false, err
		}
		approvers = append(approvers, approver)
	}

	// 如果没有审批人，直接返回true
	if len(approvers) == 0 {
		return true, nil
	}

	// 检查每个审批人是否都已批准
	for _, approver := range approvers {
		var status string
		err := as.db.QueryRow(`
            SELECT status FROM approvals 
            WHERE request_id = ? AND level = ? AND approver = ?`,
			requestID, level, approver).Scan(&status)

		if err == sql.ErrNoRows {
			return false, nil // 有审批人未做决定
		} else if err != nil {
			return false, err
		}

		if status != "approved" {
			return false, nil // 有审批人拒绝
		}
	}

	return true, nil
}

// 审批拒绝
func (as *ApprovalSystem) Reject(requestID, approver, comment string) error {
	// 获取请求
	request, err := as.getRequest(requestID)
	if err != nil {
		return err
	}
	oldStatus := request.Status

	// 检查状态
	if request.Status != constant.StatusPending {
		return fmt.Errorf("request is already %s", request.Status)
	}

	// 检查当前审批级别
	currentLevel := request.Levels[request.CurrentLevel-1]

	// 检查审批人是否有权限
	hasPermission := false
	for _, a := range currentLevel.Approvers {
		if a == approver {
			hasPermission = true
			break
		}
	}
	if !hasPermission {
		return errors.New("approver not authorized for current level")
	}

	// 检查是否已经审批过
	var existingStatus string
	err = as.db.QueryRow(`
        SELECT status FROM approvals 
        WHERE request_id = ? AND level = ? AND approver = ?`,
		requestID, request.CurrentLevel, approver).Scan(&existingStatus)

	if err == nil {
		return fmt.Errorf("approver has already %s this request", existingStatus)
	} else if err != sql.ErrNoRows {
		return err
	}

	// 记录审批决定
	decisionTime := time.Now()
	_, err = as.db.Exec(`
        INSERT INTO approvals (request_id, level, approver, status, comment, decision_time)
        VALUES (?, ?, ?, ?, ?, ?)`,
		requestID, request.CurrentLevel, approver, "rejected", comment, decisionTime)
	if err != nil {
		return err
	}
	// 更新状态为拒绝
	request.Status = constant.StatusRejected
	err = as.updateRequestStatus(request)
	if err != nil {
		return err
	}
	// 添加拒绝记录
	err = as.addHistoryRecord(requestID, request.BussinessKey, request.Category, approver, constant.ActionReject, oldStatus, constant.StatusRejected, comment)
	if err != nil {
		return err
	}
	// 添加审批意见
	if comment != "" {
		err = as.addComment(requestID, approver, comment, request.BussinessKey, request.Category)
		if err != nil {
			return err
		}
	}
	// 结束事件
	as.eventer.EndEvent(request, constant.OperateTypeReject)
	err = as.deleteRequestStatus(requestID)
	if err != nil {
		return err
	}
	// 通知请求人
	as.notifier.SendNotification(request.Requester, "审批被拒绝",
		fmt.Sprintf("您的请求 '%s' 已被 %s 拒绝", request.Title, approver))

	return nil
}

// 取消请求
func (as *ApprovalSystem) Cancel(requestID, requester string) error {
	// 获取请求
	request, err := as.getRequest(requestID)
	if err != nil {
		return err
	}
	oldStatus := request.Status

	if request.Requester != requester {
		return errors.New("only requester can cancel the request")
	}

	if request.Status != constant.StatusPending {
		return fmt.Errorf("cannot cancel request in %s state", request.Status)
	}

	request.Status = constant.StatusCancelled
	err = as.updateRequestStatus(request)
	if err != nil {
		return err
	}

	// 添加取消记录
	err = as.addHistoryRecord(requestID, request.BussinessKey, request.Category, requester, constant.ActionCancel, oldStatus, constant.StatusCancelled, "用户取消请求")
	if err != nil {
		return err
	}
	// 结束事件
	as.eventer.EndEvent(request, constant.OperateTypeCancel)
	err = as.deleteRequestStatus(requestID)
	if err != nil {
		return err
	}
	// 通知所有相关审批人
	for _, level := range request.Levels {
		for _, approver := range level.Approvers {
			as.notifier.SendNotification(approver, "审批已取消",
				fmt.Sprintf("请求 '%s' 已被取消", request.Title))
		}
	}

	return nil
}

// 添加审批意见
func (as *ApprovalSystem) addComment(requestID, user, message string, bussinessKey string, category string) error {
	_, err := as.db.Exec(`
		INSERT INTO comments (request_id, bussiness_key, category, user, message, timestamp)
		VALUES (?, ?, ?, ?, ?, ?)`,
		requestID, bussinessKey, category, user, message, time.Now())
	if err != nil {
		return fmt.Errorf("failed to add comment: %w", err)
	}
	// 记录注释历史
	err = as.addHistoryRecord(requestID, bussinessKey, category, user, constant.ActionComment, "", "", message)
	if err != nil {
		return err
	}
	return nil
}

// 获取审批意见
func (as *ApprovalSystem) GetComments(requestID string) ([]models.Comment, error) {
	rows, err := as.db.Query(`
		SELECT bussiness_key, category, user, message, timestamp 
		FROM comments 
		WHERE request_id = ? 
		ORDER BY timestamp`,
		requestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []models.Comment
	for rows.Next() {
		var c models.Comment
		err := rows.Scan(&c.BussinessKey, &c.Category, &c.User, &c.Message, &c.Timestamp)
		if err != nil {
			return nil, err
		}
		comments = append(comments, c)
	}

	return comments, nil
}

// 获取审批意见
func (as *ApprovalSystem) GetCommentsByBussinessKeyAndCategory(bussinessKey string, category string) ([]models.Comment, error) {
	rows, err := as.db.Query(`
		SELECT bussiness_key, category, user, message, timestamp 
		FROM comments 
		WHERE bussiness_key = ? AND category = ?
		ORDER BY timestamp`,
		bussinessKey, category)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []models.Comment
	for rows.Next() {
		var c models.Comment
		err := rows.Scan(&c.BussinessKey, &c.Category, &c.User, &c.Message, &c.Timestamp)
		if err != nil {
			return nil, err
		}
		comments = append(comments, c)
	}

	return comments, nil
}

// 获取请求详情
func (as *ApprovalSystem) GetRequest(requestID string) (*models.ApprovalRequest, error) {
	return as.getRequest(requestID)
}

func (as *ApprovalSystem) getRequest(requestID string) (*models.ApprovalRequest, error) {
	// 查询基本信息
	var request models.ApprovalRequest
	err := as.db.QueryRow(`
		SELECT id, bussiness_key, category, title, description, requester, status, current_level, created_at, updated_at, expires_at
		FROM requests
		WHERE id = ?`,
		requestID).Scan(
		&request.ID, &request.BussinessKey, &request.Category, &request.Title, &request.Description, &request.Requester, &request.Status,
		&request.CurrentLevel, &request.CreatedAt, &request.UpdatedAt, &request.ExpiresAt)
	if err != nil {
		return nil, err
	}

	// 查询审批级别
	rows, err := as.db.Query(`
		SELECT level, approver
		FROM levels
		WHERE request_id = ?
		ORDER BY level`,
		requestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	maxLevel := 0
	levelsMap := make(map[int][]string)
	for rows.Next() {
		var level int
		var approver string
		err := rows.Scan(&level, &approver)
		if err != nil {
			return nil, err
		}
		levelsMap[level] = append(levelsMap[level], approver)
		if level > maxLevel {
			maxLevel = level
		}
	}

	request.Levels = make([]models.ApprovalLevel, maxLevel)
	// 构建Levels切片
	for level, approvers := range levelsMap {
		request.Levels[level-1] = models.ApprovalLevel{Level: level, Approvers: approvers}
	}

	// 查询审批意见
	request.Comments, err = as.GetComments(requestID)
	if err != nil {
		return nil, err
	}

	// 查询审批决定
	request.ApprovalDecisions, err = as.GetApprovalDecisions(requestID)
	if err != nil {
		return nil, err
	}
	// ret, _ := json.MarshalIndent(request, "", "  ")
	// logger.Infof("获取请求 %s 详情: %+v", requestID, string(ret))
	return &request, nil
}

func (as *ApprovalSystem) GetApprovalDecisions(requestID string) ([]models.ApprovalDecision, error) {
	var decisions []models.ApprovalDecision
	// 查询审批决定
	rows, err := as.db.Query(`
        SELECT level, approver, status, comment, decision_time
        FROM approvals
        WHERE request_id = ?
        ORDER BY level, decision_time`,
		requestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var level int
		var approver, status, comment string
		var decisionTime time.Time
		err := rows.Scan(&level, &approver, &status, &comment, &decisionTime)
		if err != nil {
			return nil, err
		}
		decisions = append(decisions, models.ApprovalDecision{
			Level:        level,
			Approver:     approver,
			Status:       constant.ApprovalStatus(status),
			Comment:      comment,
			DecisionTime: decisionTime,
			RequestID:    requestID,
		})
	}
	return decisions, nil
}

func (as *ApprovalSystem) updateRequestStatus(request *models.ApprovalRequest) error {
	request.UpdatedAt = time.Now()
	_, err := as.db.Exec(`
		UPDATE requests 
		SET status = ?, updated_at = ?
		WHERE id = ?`,
		string(request.Status), request.UpdatedAt, request.ID)
	return err
}

func (as *ApprovalSystem) deleteRequestStatus(requestId string) error {
	_, err := as.db.Exec(`
		DELETE FROM requests 
		WHERE id = ?`,
		requestId)
	if err != nil {
		return err
	}
	logger.Infof("删除请求 %s 成功", requestId)
	return nil
}

// 检查并处理过期请求
func (as *ApprovalSystem) ProcessExpiredRequests() error {
	now := time.Now()
	rows, err := as.db.Query(`
		SELECT id 
		FROM requests 
		WHERE status = ? AND expires_at < ?`,
		string(constant.StatusPending), now)
	if err != nil {
		return err
	}
	defer rows.Close()

	var expiredIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		expiredIDs = append(expiredIDs, id)
	}

	for _, id := range expiredIDs {
		request, err := as.getRequest(id)
		if err != nil {
			logger.Errorf("获取请求 %s 失败: %v", id, err)
			continue
		}
		oldStatus := request.Status

		request.Status = constant.StatusExpired
		if err := as.updateRequestStatus(request); err != nil {
			logger.Errorf("更新请求 %s 状态失败: %v", id, err)
			continue
		}

		// 添加过期记录
		err = as.addHistoryRecord(request.ID, request.BussinessKey, request.Category, "system", constant.ActionSystem, oldStatus, constant.StatusExpired, "审批请求已过期")
		if err != nil {
			logger.Errorf("记录过期历史失败: %v", err)
			continue
		}

		// 通知请求人
		as.notifier.SendNotification(request.Requester, "审批已过期",
			fmt.Sprintf("您的请求 '%s' 已过期", request.Title))
	}

	return nil
}

func (as *ApprovalSystem) Close() error {
	return as.db.Close()
}

// 在ApprovalSystem结构体中添加历史记录方法
func (as *ApprovalSystem) addHistoryRecord(requestID, businessKey, category, operator string, action constant.HistoryAction, fromStatus, toStatus constant.ApprovalStatus, comment string) error {
	logger.Infof("添加审批历史记录: requestID=%s, businessKey=%s, category=%s, operator=%s, action=%s, fromStatus=%s, toStatus=%s, comment=%s",
		requestID, businessKey, category, operator, action, fromStatus, toStatus, comment)
	_, err := as.db.Exec(`
		INSERT INTO approval_history 
		(request_id, bussiness_key, category, operator, action, from_status, to_status, comment, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		requestID, businessKey, category, operator, string(action), string(fromStatus), string(toStatus), comment, time.Now())
	return err
}

// GetRequestHistory 获取审批请求的历史记录
func (as *ApprovalSystem) GetRequestHistory(requestID string) ([]models.ApprovalHistory, error) {
	rows, err := as.db.Query(`
		SELECT id, request_id, bussiness_key, category, operator, action, from_status, to_status, comment, created_at
		FROM approval_history
		WHERE request_id = ?
		ORDER BY created_at DESC`,
		requestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []models.ApprovalHistory
	for rows.Next() {
		var h models.ApprovalHistory
		var fromStatus, toStatus sql.NullString
		err := rows.Scan(
			&h.ID, &h.RequestID, &h.BussinessKey, &h.Category, &h.Operator, &h.Action,
			&fromStatus, &toStatus, &h.Comment, &h.CreatedAt)
		if err != nil {
			return nil, err
		}

		h.FromStatus = constant.ApprovalStatus(fromStatus.String)
		h.ToStatus = constant.ApprovalStatus(toStatus.String)
		history = append(history, h)
	}

	return history, nil
}

// GetBussinessKeyHistory 获取审批请求的历史记录
func (as *ApprovalSystem) GetBussinessKeyAndCategoryHistory(bussinessKey string, category string) ([]models.ApprovalHistory, error) {
	rows, err := as.db.Query(`
		SELECT id, request_id, bussiness_key, category, operator, action, from_status, to_status, comment, created_at
		FROM approval_history
		WHERE bussiness_key = ? AND category = ?
		ORDER BY created_at DESC`,
		bussinessKey, category)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []models.ApprovalHistory
	for rows.Next() {
		var h models.ApprovalHistory
		var fromStatus, toStatus sql.NullString
		err := rows.Scan(
			&h.ID, &h.RequestID, &h.BussinessKey, &h.Category, &h.Operator, &h.Action,
			&fromStatus, &toStatus, &h.Comment, &h.CreatedAt)
		if err != nil {
			return nil, err
		}

		h.FromStatus = constant.ApprovalStatus(fromStatus.String)
		h.ToStatus = constant.ApprovalStatus(toStatus.String)
		history = append(history, h)
	}

	return history, nil
}

// GetUserHistory 获取用户参与的审批历史
func (as *ApprovalSystem) GetUserHistory(userID string, limit int) ([]models.ApprovalHistory, error) {
	rows, err := as.db.Query(`
		SELECT h.id, h.request_id, h.bussiness_key, h.category, h.operator, h.action, h.from_status, h.to_status, h.comment, h.created_at
		FROM approval_history h
		WHERE h.operator = ?
		ORDER BY h.created_at DESC
		LIMIT ?`,
		userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []models.ApprovalHistory
	for rows.Next() {
		var h models.ApprovalHistory
		var fromStatus, toStatus sql.NullString
		err := rows.Scan(
			&h.ID, &h.RequestID, &h.BussinessKey, &h.Category, &h.Operator, &h.Action,
			&fromStatus, &toStatus, &h.Comment, &h.CreatedAt)
		if err != nil {
			return nil, err
		}

		h.FromStatus = constant.ApprovalStatus(fromStatus.String)
		h.ToStatus = constant.ApprovalStatus(toStatus.String)
		history = append(history, h)
	}

	return history, nil
}
