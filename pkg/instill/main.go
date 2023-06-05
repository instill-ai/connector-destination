package instill

import (
	"fmt"
	"sync"

	"github.com/instill-ai/connector/pkg/base"
	"github.com/instill-ai/connector/pkg/configLoader"
	connectorPB "github.com/instill-ai/protogen-go/vdp/connector/v1alpha"
)

var once sync.Once
var connector base.IConnector

type Config struct {
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

		dstConnDefs := []*connectorPB.DestinationConnectorDefinition{}
		dstDefs := []*connectorPB.ConnectorDefinition{}
		dockerImageSpecs := []*connectorPB.DockerImageSpec{}

		if err := loadDefinitionAndDockerImageSpecs(&dstConnDefs, &dstDefs, &dockerImageSpecs); err != nil {
			// logger.Fatal(err.Error())
			fmt.Println(err)
		}

		for idx, def := range dstDefs {
			var imgTag string
			if def.GetDockerImageTag() != "" {
				imgTag = ":" + def.GetDockerImageTag()
			} else {
				imgTag = def.GetDockerImageTag()
			}
			if spec, err := configLoader.DindDockerImageSpec(def.GetDockerRepository()+imgTag, &dockerImageSpecs); err != nil {
				fmt.Println(err)
			} else {
				// Create destination definition record
				if err := configLoader.CreateDestinationConnectorDefinition(dstConnDefs[idx], def, spec); err != nil {
					fmt.Println(err)
				}
				definitionMap[dstConnDefs[idx].GetUid()] = dstConnDefs[idx]
			}
		}
		connector = &Connector{
			BaseConnector: base.BaseConnector{
				DefinitionMap: definitionMap,
			},
		}

		configLoader.InitJSONSchema()

	})
	return connector
}

func (c *Connector) CreateConnection(defUid string, config interface{}) (base.IConnection, error) {
	return &Connection{config: config.(Config)}, nil
}

func (Conn *Connection) Execute(input interface{}) (interface{}, error) {
	fmt.Println("Send to Instill Artifact")
	return input, nil
}
