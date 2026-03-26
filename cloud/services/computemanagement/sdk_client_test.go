package computemanagement

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/onsi/gomega"
	"github.com/oracle/oci-go-sdk/v65/core"
)

type trackingReadCloser struct {
	reader   io.Reader
	readErr  error
	closeErr error
	closed   bool
}

func (t *trackingReadCloser) Read(p []byte) (int, error) {
	if t.readErr != nil {
		return 0, t.readErr
	}
	return t.reader.Read(p)
}

func (t *trackingReadCloser) Close() error {
	t.closed = true
	return t.closeErr
}

func TestReadAndReplayResponseBodyClosesOriginalBody(t *testing.T) {
	g := gomega.NewWithT(t)
	originalBody := &trackingReadCloser{reader: strings.NewReader(`{"foo":"bar"}`)}
	httpResponse := &http.Response{Body: originalBody}

	body, err := readAndReplayResponseBody(httpResponse)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(body).To(gomega.Equal([]byte(`{"foo":"bar"}`)))
	g.Expect(originalBody.closed).To(gomega.BeTrue())

	replayedBody, err := io.ReadAll(httpResponse.Body)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(replayedBody).To(gomega.Equal(body))
}

func TestReadAndReplayResponseBodyClosesOriginalBodyOnReadError(t *testing.T) {
	g := gomega.NewWithT(t)
	originalBody := &trackingReadCloser{readErr: errors.New("read failed")}
	httpResponse := &http.Response{Body: originalBody}

	body, err := readAndReplayResponseBody(httpResponse)
	g.Expect(err).To(gomega.MatchError("read failed"))
	g.Expect(body).To(gomega.BeNil())
	g.Expect(originalBody.closed).To(gomega.BeTrue())
}

func TestReadAndReplayResponseBodyIgnoresCloseErrorAfterSuccessfulRead(t *testing.T) {
	g := gomega.NewWithT(t)
	originalBody := &trackingReadCloser{
		reader:   strings.NewReader(`{"foo":"bar"}`),
		closeErr: errors.New("close failed"),
	}
	httpResponse := &http.Response{Body: originalBody}

	body, err := readAndReplayResponseBody(httpResponse)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(body).To(gomega.Equal([]byte(`{"foo":"bar"}`)))
	g.Expect(originalBody.closed).To(gomega.BeTrue())

	replayedBody, err := io.ReadAll(httpResponse.Body)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(replayedBody).To(gomega.Equal(body))
}

func TestPreserveInstanceConfigurationExtendedMetadataNumbers(t *testing.T) {
	g := gomega.NewWithT(t)

	response := core.GetInstanceConfigurationResponse{
		InstanceConfiguration: core.InstanceConfiguration{
			InstanceDetails: core.ComputeInstanceDetails{
				LaunchDetails: &core.InstanceConfigurationLaunchInstanceDetails{
					ExtendedMetadata: map[string]interface{}{
						"workload": map[string]interface{}{
							"timestampNs": float64(9223372036854775807),
						},
					},
				},
			},
		},
	}

	rawBody := []byte(`{
		"instanceDetails": {
			"launchDetails": {
				"extendedMetadata": {
					"workload": {
						"timestampNs": 9223372036854775807,
						"profile": "standard",
						"features": {
							"hpc": true
						}
					}
				}
			}
		}
	}`)

	err := preserveInstanceConfigurationExtendedMetadataNumbers(&response, rawBody)
	g.Expect(err).ToNot(gomega.HaveOccurred())

	instanceDetails, ok := response.InstanceConfiguration.InstanceDetails.(core.ComputeInstanceDetails)
	g.Expect(ok).To(gomega.BeTrue())
	g.Expect(instanceDetails.LaunchDetails).ToNot(gomega.BeNil())
	g.Expect(instanceDetails.LaunchDetails.ExtendedMetadata).To(gomega.Equal(map[string]interface{}{
		"workload": map[string]interface{}{
			"timestampNs": json.Number("9223372036854775807"),
			"profile":     "standard",
			"features": map[string]interface{}{
				"hpc": true,
			},
		},
	}))
}
