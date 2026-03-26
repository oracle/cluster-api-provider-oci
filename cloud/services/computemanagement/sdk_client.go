package computemanagement

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
)

type sdkClient struct {
	*core.ComputeManagementClient
}

func NewClient(client *core.ComputeManagementClient) Client {
	return &sdkClient{ComputeManagementClient: client}
}

func (c *sdkClient) GetInstanceConfiguration(ctx context.Context, request core.GetInstanceConfigurationRequest) (response core.GetInstanceConfigurationResponse, err error) {
	var ociResponse common.OCIResponse
	policy := common.NoRetryPolicy()
	if c.RetryPolicy() != nil {
		policy = *c.RetryPolicy()
	}
	if request.RetryPolicy() != nil {
		policy = *request.RetryPolicy()
	}

	ociResponse, err = common.Retry(ctx, request, c.getInstanceConfiguration, policy)
	if err != nil {
		if ociResponse != nil {
			if httpResponse := ociResponse.HTTPResponse(); httpResponse != nil {
				opcRequestID := httpResponse.Header.Get("opc-request-id")
				response = core.GetInstanceConfigurationResponse{RawResponse: httpResponse, OpcRequestId: &opcRequestID}
			} else {
				response = core.GetInstanceConfigurationResponse{}
			}
		}
		return
	}
	if convertedResponse, ok := ociResponse.(core.GetInstanceConfigurationResponse); ok {
		response = convertedResponse
	} else {
		err = fmt.Errorf("failed to convert OCIResponse into GetInstanceConfigurationResponse")
	}
	return
}

func (c *sdkClient) getInstanceConfiguration(ctx context.Context, request common.OCIRequest, binaryReqBody *common.OCIReadSeekCloser, extraHeaders map[string]string) (common.OCIResponse, error) {
	httpRequest, err := request.HTTPRequest(http.MethodGet, "/instanceConfigurations/{instanceConfigurationId}", binaryReqBody, extraHeaders)
	if err != nil {
		return nil, err
	}

	var response core.GetInstanceConfigurationResponse
	httpResponse, err := c.Call(ctx, &httpRequest)
	response.RawResponse = httpResponse
	if err != nil {
		common.CloseBodyIfValid(httpResponse)
		apiReferenceLink := "https://docs.oracle.com/iaas/api/#/en/iaas/20160918/InstanceConfiguration/GetInstanceConfiguration"
		err = common.PostProcessServiceError(err, "ComputeManagement", "GetInstanceConfiguration", apiReferenceLink)
		return response, err
	}

	body, err := readAndReplayResponseBody(httpResponse)
	if err != nil {
		return response, err
	}
	defer common.CloseBodyIfValid(httpResponse)

	if err := common.UnmarshalResponse(httpResponse, &response); err != nil {
		return response, err
	}
	if err := preserveInstanceConfigurationExtendedMetadataNumbers(&response, body); err != nil {
		return response, err
	}

	return response, nil
}

func readAndReplayResponseBody(httpResponse *http.Response) ([]byte, error) {
	if httpResponse == nil || httpResponse.Body == nil {
		return nil, nil
	}

	originalBody := httpResponse.Body
	body, err := io.ReadAll(originalBody)
	_ = originalBody.Close()
	if err != nil {
		return nil, err
	}

	httpResponse.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

func preserveInstanceConfigurationExtendedMetadataNumbers(response *core.GetInstanceConfigurationResponse, body []byte) error {
	if response == nil || len(body) == 0 {
		return nil
	}

	instanceDetails, ok := response.InstanceConfiguration.InstanceDetails.(core.ComputeInstanceDetails)
	if !ok || instanceDetails.LaunchDetails == nil {
		return nil
	}

	var rawResponse struct {
		InstanceDetails struct {
			LaunchDetails struct {
				ExtendedMetadata map[string]interface{} `json:"extendedMetadata"`
			} `json:"launchDetails"`
		} `json:"instanceDetails"`
	}

	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&rawResponse); err != nil {
		return err
	}

	if rawResponse.InstanceDetails.LaunchDetails.ExtendedMetadata == nil {
		return nil
	}

	instanceDetails.LaunchDetails.ExtendedMetadata = rawResponse.InstanceDetails.LaunchDetails.ExtendedMetadata
	response.InstanceConfiguration.InstanceDetails = instanceDetails
	return nil
}
