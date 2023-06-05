package numbersProtocol

import (
	"fmt"
	"sync"

	"github.com/instill-ai/connector/pkg/base"
	connectorPB "github.com/instill-ai/protogen-go/vdp/connector/v1alpha"
)

var once sync.Once
var connector base.IConnector

const DefinitionUuid = "70d8664a-d512-4517-a5e8-5d4da81756a7"

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
		definitionMap := map[string]interface{}{}
		definitionMap[DefinitionUuid] = &connectorPB.DestinationConnectorDefinition{
			Uid: DefinitionUuid,
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
