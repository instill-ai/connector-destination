package numbers

import (
	"sync"

	_ "embed"

	"github.com/gofrs/uuid"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/instill-ai/connector/pkg/base"
	"github.com/instill-ai/connector/pkg/configLoader"

	connectorPB "github.com/instill-ai/protogen-go/vdp/connector/v1alpha"
)

//go:embed config/seed/destination_definitions.yaml
var destinationDefinitionsYaml []byte

//go:embed config/seed/destination_specs.yaml
var destinationSpecsYaml []byte

var once sync.Once
var connector base.IConnector

type Connector struct {
	base.BaseConnector
}

type ConnectorOptions struct {
	APIToken string
}

type Connection struct {
	base.BaseConnection
	config *structpb.Struct
}

func Init(logger *zap.Logger, options ConnectorOptions) base.IConnector {
	once.Do(func() {
		connDefs := []*connectorPB.DestinationConnectorDefinition{}

		loader := configLoader.InitJSONSchema(logger)
		loader.Load(destinationDefinitionsYaml, destinationSpecsYaml, &connDefs)

		connector = &Connector{
			BaseConnector: base.BaseConnector{Logger: logger},
		}
		for idx := range connDefs {
			err := connector.AddConnectorDefinition(uuid.FromStringOrNil(connDefs[idx].GetUid()), connDefs[idx].GetId(), connDefs[idx])
			if err != nil {
				logger.Warn(err.Error())
			}
		}
	})
	return connector
}

func (c *Connector) CreateConnection(defUid uuid.UUID, config *structpb.Struct, logger *zap.Logger) (base.IConnection, error) {
	return &Connection{
		BaseConnection: base.BaseConnection{Logger: logger},
		config:         config,
	}, nil
}

func (con *Connection) Execute(input []*connectorPB.DataPayload) ([]*connectorPB.DataPayload, error) {
	return input, nil
}

func (con *Connection) Test() (connectorPB.Connector_State, error) {
	return connectorPB.Connector_STATE_UNSPECIFIED, nil
}
