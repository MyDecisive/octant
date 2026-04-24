package budgetfilter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/mydecisive/octant/internal/config"
	"github.com/mydecisive/octant/internal/wrapper"
)

const (
	contentTypeJSON             = "application/json"
	mdaiGatewayRootURLFormatter = "http://%s.%s.svc.cluster.local"
	mdaiGatewayGetVarFormatter  = "/variables/values/hub/%s/var/%s"
	mdaiGatewayPostVarFormatter = "/variables/hub/%s/var/%s"
)

// VariableAccessor provide functionalities to get and update the underlying storage for the filter settings.
type VariableAccessor interface {
	// GetVariable returns the value of the given variable.
	GetVariable(namespace string, hubName string, varName string) (string, error)
	// UpdateVariable updates the value of the given variable.
	// This will return `ErrInvalid` if the operation failed.
	UpdateVariable(namespace string, hubName string, varName string, value any) error
}

// MDAIGateway implements VariableAccessor.
type MDAIGateway struct {
	client      wrapper.HTTPClient
	gatewayName string
}

// Ensure MDAIGateway implements VariableAccessor.
var _ VariableAccessor = &MDAIGateway{}

// NewMDAIGateway returns a new instance of MDAIGateway.
func NewMDAIGateway(c *config.Configuration, client wrapper.HTTPClient) *MDAIGateway {
	return &MDAIGateway{
		client:      client,
		gatewayName: c.Budget.DefaultMDAIGatewayName,
	}
}

// GetVariable returns the value of the given variable from MDAI gateway.
func (mdai *MDAIGateway) GetVariable(namespace string, hubName string, varName string) (string, error) {
	url := fmt.Sprintf(mdaiGatewayRootURLFormatter, mdai.gatewayName, namespace) + fmt.Sprintf(mdaiGatewayGetVarFormatter, hubName, varName)
	resp, err := mdai.client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("%w:status %d", ErrInvalid, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result map[string]string
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	return result[varName], nil
}

// UpdateVariable updates the value of the given variable in MDAI gateway.
// This will return `ErrInvalid` if the operation failed.
func (mdai *MDAIGateway) UpdateVariable(namespace string, hubName string, varName string, value any) error {
	url := fmt.Sprintf(mdaiGatewayRootURLFormatter, mdai.gatewayName, namespace) + fmt.Sprintf(mdaiGatewayPostVarFormatter, hubName, varName)
	jsonValue, err := json.Marshal(map[string]any{"data": value})
	if err != nil {
		return err
	}
	resp, err := mdai.client.Post(url, contentTypeJSON, bytes.NewBuffer(jsonValue))
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode >= 300 {
		return fmt.Errorf("%w:status %d", ErrInvalid, resp.StatusCode)
	}
	return nil
}
