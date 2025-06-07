package notifier

// 通知接口
type Notifier interface {
	SendNotification(to, subject, message string) error
}
