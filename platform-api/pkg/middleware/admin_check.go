package middleware

import (
	"log/slog"
	"net/http"

	"github.com/openshift-online/rosa-hyperfleet-api/platform-api/pkg/authz"
)

// AdminCheck provides middleware for checking admin status
type AdminCheck struct {
	authorizer authz.Checker
	logger     *slog.Logger
}

// NewAdminCheck creates a new AdminCheck middleware
func NewAdminCheck(authorizer authz.Checker, logger *slog.Logger) *AdminCheck {
	return &AdminCheck{
		authorizer: authorizer,
		logger:     logger,
	}
}

// RequireAdmin returns 403 if the caller is not an admin for the account.
// Privileged accounts bypass this check.
func (a *AdminCheck) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		accountID := GetAccountID(ctx)

		if accountID == "" {
			writeError(w, ErrMissingAccountID)
			return
		}

		// Privileged accounts bypass admin check
		if GetPrivileged(ctx) {
			next.ServeHTTP(w, r)
			return
		}

		callerARN := GetCallerARN(ctx)
		if callerARN == "" {
			writeError(w, ErrMissingCallerARN)
			return
		}

		isAdmin, err := a.authorizer.IsAdmin(ctx, accountID, callerARN)
		if err != nil {
			a.logger.Error("failed to check admin status", "error", err, "account_id", accountID, "caller_arn", callerARN)
			writeError(w, ErrAdminCheckFailed)
			return
		}

		if !isAdmin {
			a.logger.Warn("admin access denied", "account_id", accountID, "caller_arn", callerARN)
			writeError(w, ErrNotAdmin)
			return
		}

		next.ServeHTTP(w, r)
	})
}
