package middleware

import (
	"log/slog"
	"net/http"

	"github.com/openshift-online/rosa-hyperfleet-api/platform-api/pkg/authz"
)

// AccountCheck provides middleware for checking account provisioning status
type AccountCheck struct {
	authorizer authz.Checker
	logger     *slog.Logger
}

// NewAccountCheck creates a new AccountCheck middleware
func NewAccountCheck(authorizer authz.Checker, logger *slog.Logger) *AccountCheck {
	return &AccountCheck{
		authorizer: authorizer,
		logger:     logger,
	}
}

// RequireProvisioned returns 403 if the account is not provisioned
// Privileged accounts are always considered provisioned
func (a *AccountCheck) RequireProvisioned(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		accountID := GetAccountID(ctx)

		if accountID == "" {
			writeError(w, ErrMissingAccountID)
			return
		}

		// Check if privileged first (from context if available)
		if GetPrivileged(ctx) {
			next.ServeHTTP(w, r)
			return
		}

		// Check if account is provisioned
		provisioned, err := a.authorizer.IsAccountProvisioned(ctx, accountID)
		if err != nil {
			a.logger.Error("failed to check account provisioning status", "error", err, "account_id", accountID)
			writeError(w, ErrProvisionedCheckFailed)
			return
		}

		if !provisioned {
			a.logger.Warn("account not provisioned", "account_id", accountID)
			writeError(w, ErrAccountNotProvisioned)
			return
		}

		next.ServeHTTP(w, r)
	})
}
