package main

import (
	"fmt"

	connectorDestination "github.com/instill-ai/connector-destination/pkg"
	connectorPB "github.com/instill-ai/protogen-go/vdp/connector/v1alpha"
	"go.uber.org/zap"
)

func main() {

	logger, _ := zap.NewDevelopment()
	// It is singleton, should be loaded when connector-backend started
	connector := connectorDestination.Init(logger)

	// For apis: Get connector definitsion apis
	for _, v := range connector.ListConnectorDefinitions() {
		fmt.Println(v.(*connectorPB.DestinationConnectorDefinition).GetId())
	}

	// in connector-backend:
	// if user trigger connectorA
	// ->connectorA.defUid
	// ->connectorA.configuration
	// connection, _ := connector.CreateConnection(uuid.FromStringOrNil("70d8664a-d512-4517-a5e8-5d4da81756a7"), &structpb.Struct{})
	// _, err := connection.Execute(nil)

	// if err != nil {
	// 	fmt.Println(err)
	// }

}
