package platform

import (
	"context"
	"fmt"
	"sync"

	"github.com/wyw14/cry-072/internal/domain"
)

type NotificationAdapter interface {
	Send(ctx context.Context, notification domain.Notification) error
	Name() string
}

type LocalNotificationAdapter struct {
	mu       sync.Mutex
	messages []domain.Notification
	failFor  map[string]error
}

func NewLocalNotificationAdapter() *LocalNotificationAdapter {
	return &LocalNotificationAdapter{failFor: make(map[string]error)}
}

func (a *LocalNotificationAdapter) Name() string { return "local-demo" }

func (a *LocalNotificationAdapter) Send(ctx context.Context, notification domain.Notification) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.failFor[notification.Destination.OperatorID]; err != nil {
		return fmt.Errorf("local notification rejected: %w", err)
	}
	a.messages = append(a.messages, notification)
	return nil
}

func (a *LocalNotificationAdapter) Messages() []domain.Notification {
	a.mu.Lock()
	defer a.mu.Unlock()
	result := make([]domain.Notification, len(a.messages))
	copy(result, a.messages)
	return result
}

func (a *LocalNotificationAdapter) FailRecipient(recipient string, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.failFor[recipient] = err
}
