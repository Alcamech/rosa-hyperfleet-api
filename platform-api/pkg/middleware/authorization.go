package middleware

import (
	"log/slog"
	"net/http"
)

// Authorization provides account allowlist-based authorization middleware
type Authorization struct {
	allowedAccounts map[string]struct{}
	logger          *slog.Logger
}

// NewAuthorization creates a new Authorization middleware
func NewAuthorization(allowedAccounts []string, logger *slog.Logger) *Authorization {
	allowed := make(map[string]struct{}, len(allowedAccounts))
	for _, acc := range allowedAccounts {
		allowed[acc] = struct{}{}
	}
	return &Authorization{
		allowedAccounts: allowed,
		logger:          logger,
	}
}

// RequireAllowedAccount verifies that the AWS account is in the allowlist
func (a *Authorization) RequireAllowedAccount(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		accountID := GetAccountID(ctx)

		if accountID == "" {
			a.logger.Warn("missing account ID in request")
			writeError(w, ErrMissingAccountID, a.logger)
			return
		}

		if _, allowed := a.allowedAccounts[accountID]; !allowed {
			a.logger.Warn("account not allowed", "account_id", accountID)
			writeError(w, ErrAccountNotAllowed, a.logger)
			return
		}

		next.ServeHTTP(w, r)
	})
}
