package destination

import (
	"fmt"
	"sync"

	"github.com/instill-ai/connector-destination/pkg/airbyte"
	"github.com/instill-ai/connector-destination/pkg/instill"
	"github.com/instill-ai/connector-destination/pkg/numbersProtocol"
	"github.com/instill-ai/connector/pkg/base"
)

var once sync.Once
var connector base.IConnector

type Connector struct {
	base.BaseConnector
	airbyteConnector         base.IConnector
	instillConnector         base.IConnector
	numbersProtocolConnector base.IConnector
}

func Init() base.IConnector {
	once.Do(func() {
		definitionMap := map[string]interface{}{}

		airbyteConnector := airbyte.Init()
		instillConnector := instill.Init()
		numbersProtocolConnector := numbersProtocol.Init()

		// TODO: assert no duplicate uid
		for k, v := range airbyteConnector.GetConnectorDefinitionMap() {
			definitionMap[k] = v
		}
		for k, v := range instillConnector.GetConnectorDefinitionMap() {
			definitionMap[k] = v
		}
		for k, v := range numbersProtocolConnector.GetConnectorDefinitionMap() {
			definitionMap[k] = v
		}
		connector = &Connector{
			BaseConnector: base.BaseConnector{
				DefinitionMap: definitionMap,
			},
			airbyteConnector:         airbyteConnector,
			instillConnector:         instillConnector,
			numbersProtocolConnector: numbersProtocolConnector,
		}
	})
	return connector
}

func (c *Connector) CreateConnection(defUid string, config interface{}) (base.IConnection, error) {
	switch {
	case c.airbyteConnector.HasUid(defUid):
		return c.airbyteConnector.CreateConnection(defUid, config)
	case c.instillConnector.HasUid(defUid):
		return c.instillConnector.CreateConnection(defUid, config)
	case c.numbersProtocolConnector.HasUid(defUid):
		return c.numbersProtocolConnector.CreateConnection(defUid, config)
	default:
		return nil, fmt.Errorf("no destinationConnector uid: %s", defUid)
	}
}
