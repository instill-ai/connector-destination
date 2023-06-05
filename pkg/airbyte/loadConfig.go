package airbyte

import (
	"fmt"

	"github.com/instill-ai/connector/pkg/configLoader"
	connectorPB "github.com/instill-ai/protogen-go/vdp/connector/v1alpha"
)

const (
	seedDir = "config/init/%s/seed/%s"
)

func loadDefinitionAndDockerImageSpecs(
	dstConnDefs *[]*connectorPB.DestinationConnectorDefinition,
	dstDefs *[]*connectorPB.ConnectorDefinition,
	dockerImageSpecs *[]*connectorPB.DockerImageSpec) error {

	destinationDefsFiles := []string{
		fmt.Sprintf(seedDir, "airbyte", "destination_definitions.yaml"),
	}

	specsFiles := []string{
		fmt.Sprintf(seedDir, "airbyte", "destination_specs.yaml"),
	}

	for _, filename := range destinationDefsFiles {
		if jsonSliceMap, err := configLoader.ProcessJSONSliceMap(filename); err == nil {
			if err := configLoader.UnmarshalConnectorPB(jsonSliceMap, dstConnDefs); err != nil {
				return err
			}
			if err := configLoader.UnmarshalConnectorPB(jsonSliceMap, dstDefs); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	for _, filename := range specsFiles {
		if jsonSliceMap, err := configLoader.ProcessJSONSliceMap(filename); err == nil {
			if err := configLoader.UnmarshalConnectorPB(jsonSliceMap, dockerImageSpecs); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	return nil
}
