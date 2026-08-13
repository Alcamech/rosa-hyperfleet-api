package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/openshift-online/rosa-hyperfleet-api/platform-api/pkg/authz"
)

// contextKey for privileged status
const ContextKeyPrivileged contextKey = "privileged"

// Privileged provides middleware for checking privileged account status
type Privileged struct {
	authorizer authz.Checker
	logger     *slog.Logger
}

// NewPrivileged creates a new Privileged middleware
func NewPrivileged(authorizer authz.Checker, logger *slog.Logger) *Privileged {
	return &Privileged{
		authorizer: authorizer,
		logger:     logger,
	}
}

// CheckPrivileged adds privileged status to the context
// This middleware should run after Identity middleware
func (p *Privileged) CheckPrivileged(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		accountID := GetAccountID(ctx)

		if accountID == "" {
			next.ServeHTTP(w, r)
			return
		}

		isPrivileged, err := p.authorizer.IsPrivileged(ctx, accountID)
		if err != nil {
			p.logger.Error("failed to check privileged status", "error", err, "account_id", accountID)
			// Continue without privileged status on error
			next.ServeHTTP(w, r)
			return
		}

		ctx = context.WithValue(ctx, ContextKeyPrivileged, isPrivileged)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequirePrivileged returns 403 if the account is not privileged
func (p *Privileged) RequirePrivileged(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		accountID := GetAccountID(ctx)

		if accountID == "" {
			writeError(w, ErrMissingAccountID, p.logger)
			return
		}

		isPrivileged := GetPrivileged(ctx)
		if !isPrivileged {
			// Double-check in case CheckPrivileged wasn't run
			var err error
			isPrivileged, err = p.authorizer.IsPrivileged(ctx, accountID)
			if err != nil {
				p.logger.Error("failed to check privileged status", "error", err, "account_id", accountID)
				writeError(w, ErrPrivilegedCheckFailed, p.logger)
				return
			}
		}

		if !isPrivileged {
			p.logger.Warn("privileged access denied", "account_id", accountID)
			writeError(w, ErrNotPrivileged, p.logger)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// GetPrivileged retrieves the privileged status from context
func GetPrivileged(ctx context.Context) bool {
	if v := ctx.Value(ContextKeyPrivileged); v != nil {
		return v.(bool)
	}
	return false
}
