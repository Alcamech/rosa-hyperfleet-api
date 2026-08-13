package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/openshift-online/rosa-hyperfleet-api/platform-api/pkg/api"
)

// InfoHandler handles the info endpoint
type InfoHandler struct {
	logger *slog.Logger
}

// NewInfoHandler creates a new InfoHandler
func NewInfoHandler(logger *slog.Logger) *InfoHandler {
	return &InfoHandler{logger: logger}
}

// Info handles GET /api/v0/info
// Returns the ARN of the IAM role used to invoke Lambda functions in this regional account.
// The account ID is parsed from the TARGET_GROUP_ARN environment variable.
func (h *InfoHandler) Info(w http.ResponseWriter, r *http.Request) {
	tgARN := os.Getenv("TARGET_GROUP_ARN")
	// Target Group ARN format: arn:aws:elasticloadbalancing:{region}:{account_id}:targetgroup/{name}/{id}
	parts := strings.SplitN(tgARN, ":", 6)
	if len(parts) < 6 || parts[4] == "" {
		writeAPIError(w, ErrInfoRegionalAccountUnavailable, h.logger)
		return
	}

	accountID := parts[4]
	arn := fmt.Sprintf("arn:aws:iam::%s:role/LambdaExecutor", accountID)

	if err := api.Write(w, http.StatusOK, map[string]string{"arn": arn}); err != nil {
		h.logger.Error("failed to write response", "error", err)
	}
}
