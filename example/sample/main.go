package main

import (
	"fmt"

	"github.com/gofrs/uuid"
	connectorDestination "github.com/instill-ai/connector-destination/pkg"
	connectorDestinationAirbyte "github.com/instill-ai/connector-destination/pkg/airbyte"
	connectorDestinationNumbers "github.com/instill-ai/connector-destination/pkg/numbers"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/structpb"
)

func main() {

	logger, _ := zap.NewDevelopment()
	// It is singleton, should be loaded when connector-backend started
	connector := connectorDestination.Init(logger, connectorDestination.ConnectorOptions{
		Airbyte: connectorDestinationAirbyte.ConnectorOptions{
			MountSourceVDP:     "vdp",
			MountTargetVDP:     "/tmp/vdp",
			MountSourceAirbyte: "airbyte",
			MountTargetAirbyte: "/tmp/airbyte",
			VDPProtocolPath:    "vdp_protocol.yaml",
		},
		Numbers: connectorDestinationNumbers.ConnectorOptions{
			SharedToken: connectorDestinationNumbers.ConnectorOptionsSharedToken{
				Token:   "",
				Enabled: false,
			},
		},
	})

	// in connector-backend:
	// if user trigger connectorA
	// ->connectorA.defUid
	// ->connectorA.configuration
	connection, _ := connector.CreateConnection(uuid.FromStringOrNil("70d8664a-d512-4517-a5e8-5d4da81756a7"), &structpb.Struct{}, logger)
	_, err := connection.Execute(nil)

	if err != nil {
		fmt.Println(err)
	}

}
