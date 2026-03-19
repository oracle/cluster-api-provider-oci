package scope

import (
	"net/http"
	"strings"

	"github.com/oracle/oci-go-sdk/v65/common"
)

func isAlreadyExistsOCIError(err error) bool {
	serviceErr, ok := common.IsServiceError(err)
	if !ok {
		return false
	}
	if serviceErr.GetHTTPStatusCode() == http.StatusConflict {
		return true
	}

	code := strings.ToLower(serviceErr.GetCode())
	message := strings.ToLower(serviceErr.GetMessage())
	return strings.Contains(code, "already") || strings.Contains(message, "already exists")
}
