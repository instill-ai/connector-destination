package numbers

import (
	"fmt"
	"sync"

	_ "embed"

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

type Config struct {
	ApiUrl   string
	ApiToken string
}

type Connector struct {
	base.BaseConnector
}

type Connection struct {
	base.BaseConnection
	config Config
}

func Init() base.IConnector {
	once.Do(func() {
		connDefs := []*connectorPB.DestinationConnectorDefinition{}

		configLoader.InitJSONSchema()
		configLoader.Load(destinationDefinitionsYaml, destinationSpecsYaml, &connDefs)

		definitionMap := map[string]interface{}{}
		for idx := range connDefs {
			definitionMap[connDefs[idx].GetUid()] = connDefs[idx]

		}
		connector = &Connector{
			BaseConnector: base.BaseConnector{
				DefinitionMap: definitionMap,
			},
		}
	})
	return connector
}

func (c *Connector) CreateConnection(defUid string, config interface{}) (base.IConnection, error) {
	return &Connection{config: config.(Config)}, nil
}

func (Conn *Connection) Execute(input interface{}) (interface{}, error) {
	fmt.Printf("%s %s\n", Conn.config.ApiUrl, Conn.config.ApiToken)
	return input, nil
}
