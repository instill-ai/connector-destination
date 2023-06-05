package main

import (
	"fmt"

	connectorDestination "github.com/instill-ai/connector-destination/pkg"
	"github.com/instill-ai/connector-destination/pkg/numbersProtocol"
	connectorPB "github.com/instill-ai/protogen-go/vdp/connector/v1alpha"
)

func main() {

	connector := connectorDestination.Init()
	for k, v := range connector.GetConnectorDefinitionMap() {
		fmt.Println(k)
		fmt.Println(v.(*connectorPB.DestinationConnectorDefinition).GetId())
	}
	connection, _ := connector.CreateConnection("70d8664a-d512-4517-a5e8-5d4da81756a7", numbersProtocol.Config{
		ApiUrl:   "111",
		ApiToken: "222",
	})
	connection.Execute(nil)

}
