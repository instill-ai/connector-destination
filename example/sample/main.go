package main

import (
	"fmt"

	connectorDestination "github.com/instill-ai/connector-destination/pkg"
	numbers "github.com/instill-ai/connector-destination/pkg/numbers"
	connectorPB "github.com/instill-ai/protogen-go/vdp/connector/v1alpha"
)

func main() {

	// It is singleton, should be loaded when connector-backend started
	connector := connectorDestination.Init()

	// For apis: Get connector definitsion apis
	for k, v := range connector.GetConnectorDefinitionMap() {
		fmt.Println(k)
		fmt.Println(v.(*connectorPB.DestinationConnectorDefinition).GetId())
	}

	// in connector-backend:
	// if user trigger connectorA
	// ->connectorA.defUid
	// ->connectorA.configuration
	connection, _ := connector.CreateConnection("70d8664a-d512-4517-a5e8-5d4da81756a7", numbers.Config{
		ApiUrl:   "111",
		ApiToken: "222",
	})
	_, err := connection.Execute(nil)

	if err != nil {
		fmt.Println(err)
	}

}
