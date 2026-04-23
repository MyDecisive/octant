package budgetfilter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/mydecisive/octant/internal/config"
)

const (
	mdaiGatewayRootURLFormatter   = "%s.%s.svc.cluster.local%s"
	mdaiGatewayGetAllVarFormatter = "/variables/list/hub/%s"
	mdaiGatewayPostVarFormatter   = "/variables/hub/%s/var/%s"
)

type VariableAccessor interface {
	GetAllVariables(namespace string, hubName string) (map[string]string, error)
	PostVariable(namespace string, hubName string, varName string, value any) error
}

type MDAIGateway struct {
	client      *http.Client
	gatewayName string
}

// Ensure MDAIGateway implements VariableAccessor.
var _ VariableAccessor = &MDAIGateway{}

func NewMDAIGateway(c *config.Configuration, client *http.Client) *MDAIGateway {
	return &MDAIGateway{
		client:      client,
		gatewayName: c.Budget.DefaultMDAIGatewayName,
	}
}

func (mdai *MDAIGateway) GetAllVariables(namespace string, hubName string) (map[string]string, error) {
	url := fmt.Sprintf(mdaiGatewayRootURLFormatter, mdai.gatewayName, namespace, fmt.Sprintf(mdaiGatewayGetAllVarFormatter, hubName))

	resp, err := mdai.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result map[string]string
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return result, nil
}

func (mdai *MDAIGateway) PostVariable(namespace string, hubName string, varName string, value any) error {
	url := fmt.Sprintf(mdaiGatewayRootURLFormatter, mdai.gatewayName, namespace, fmt.Sprintf(mdaiGatewayPostVarFormatter, hubName, varName))
	jsonValue, _ := json.Marshal(map[string]any{"data": value})
	resp, err := mdai.client.Post(url, "application/json", bytes.NewBuffer([]byte(jsonValue)))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("%w:status %d", ErrInvalid, resp.StatusCode)
	}
	return nil
}
