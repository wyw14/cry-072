package application

import (
	"context"
	"fmt"

	"github.com/wyw14/cry-072/internal/domain"
)

type auditedChange struct {
	operation string
	event     domain.AuditEvent
	apply     func(Store) error
}

func commitAudited(ctx context.Context, repository Repository, change auditedChange) error {
	err := repository.WithinTx(ctx, func(store Store) error {
		if err := change.apply(store); err != nil {
			return err
		}
		return store.AppendAudit(ctx, change.event)
	})
	if err != nil {
		return fmt.Errorf("%s: %w", change.operation, err)
	}
	return nil
}
