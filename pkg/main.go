package destination

import (
	"fmt"
	"sync"

	"github.com/gofrs/uuid"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/instill-ai/connector-destination/pkg/airbyte"
	"github.com/instill-ai/connector-destination/pkg/instill"
	"github.com/instill-ai/connector-destination/pkg/numbers"
	"github.com/instill-ai/connector/pkg/base"
)

var once sync.Once
var connector base.IConnector

type Connector struct {
	base.BaseConnector
	airbyteConnector base.IConnector
	instillConnector base.IConnector
	numbersConnector base.IConnector
}

type ConnectorOptions struct {
	Airbyte airbyte.ConnectorOptions
	Numbers numbers.ConnectorOptions
}

func Init(logger *zap.Logger, options ConnectorOptions) base.IConnector {
	once.Do(func() {

		airbyteConnector := airbyte.Init(logger, options.Airbyte)
		instillConnector := instill.Init(logger)
		// numbersConnector := numbers.Init(logger, options.Numbers)

		connector = &Connector{
			BaseConnector:    base.BaseConnector{Logger: logger},
			airbyteConnector: airbyteConnector,
			instillConnector: instillConnector,
			// numbersConnector: numbersConnector,
		}

		// TODO: assert no duplicate uid
		// Note: we preserve the order as yaml
		for _, uid := range airbyteConnector.ListConnectorDefinitionUids() {
			def, err := airbyteConnector.GetConnectorDefinitionByUid(uid)
			if err != nil {
				logger.Error(err.Error())
			}
			connector.AddConnectorDefinition(uid, def.GetId(), def)
		}
		for _, uid := range instillConnector.ListConnectorDefinitionUids() {
			def, err := instillConnector.GetConnectorDefinitionByUid(uid)
			if err != nil {
				logger.Error(err.Error())
			}
			connector.AddConnectorDefinition(uid, def.GetId(), def)
		}
		// for _, uid := range numbersConnector.ListConnectorDefinitionUids() {
		// 	def, err := numbersConnector.GetConnectorDefinitionByUid(uid)
		// 	if err != nil {
		// 		logger.Error(err.Error())
		// 	}
		// 	connector.AddConnectorDefinition(uid, def.GetId(), def)
		// }

	})
	return connector
}

func (c *Connector) CreateConnection(defUid uuid.UUID, config *structpb.Struct, logger *zap.Logger) (base.IConnection, error) {
	switch {
	case c.airbyteConnector.HasUid(defUid):
		return c.airbyteConnector.CreateConnection(defUid, config, logger)
	case c.instillConnector.HasUid(defUid):
		return c.instillConnector.CreateConnection(defUid, config, logger)
	// case c.numbersConnector.HasUid(defUid):
	// 	return c.numbersConnector.CreateConnection(defUid, config, logger)
	default:
		return nil, fmt.Errorf("no destinationConnector uid: %s", defUid)
	}
}
