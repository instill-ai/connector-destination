package numbers

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"strings"
	"sync"
	"time"

	_ "embed"

	"github.com/gofrs/uuid"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/instill-ai/connector-destination/pkg/common"
	"github.com/instill-ai/connector/pkg/base"
	"github.com/instill-ai/connector/pkg/configLoader"

	connectorPB "github.com/instill-ai/protogen-go/vdp/connector/v1alpha"
	modelv1alpha "github.com/instill-ai/protogen-go/vdp/model/v1alpha"
	pipelinev1alpha "github.com/instill-ai/protogen-go/vdp/pipeline/v1alpha"
)

const ApiUrlPin = "https://eoqctv92ahgrcif.m.pipedream.net"
const ApiUrlCommit = "https://eo883tj75azolos.m.pipedream.net"
const ApiUrlMe = "https://api.numbersprotocol.io/api/v3/auth/users/me"

//go:embed config/seed/destination_definitions.yaml
var destinationDefinitionsYaml []byte

//go:embed config/seed/destination_specs.yaml
var destinationSpecsYaml []byte

//go:embed config/seed/destination_specs_shared_token.yaml
var destinationSpecsSharedTokenYaml []byte

var once sync.Once
var connector base.IConnector

type Connector struct {
	base.BaseConnector
	Options ConnectorOptions
}

type ConnectorOptions struct {
	SharedToken ConnectorOptionsSharedToken
}

type ConnectorOptionsSharedToken struct {
	Token   string
	Enabled bool
}

type Connection struct {
	base.BaseConnection
	config    *structpb.Struct
	connector *Connector
}

type CommitCustomLicense struct {
	Name string `json:"name"`
}
type CommitCustom struct {
	GeneratedBy      string              `json:"generatedBy"`
	GeneratedThrough string              `json:"generatedThrough"`
	Prompt           string              `json:"prompt"`
	CreatorWallet    string              `json:"creatorWallet"`
	License          CommitCustomLicense `json:"license"`
}
type Commit struct {
	AssetCid              string       `json:"assetCid"`
	AssetSha256           string       `json:"assetSha256"`
	EncodingFormat        string       `json:"encodingFormat"`
	AssetTimestampCreated int64        `json:"assetTimestampCreated"`
	AssetCreator          string       `json:"assetCreator"`
	Abstract              string       `json:"abstract"`
	Custom                CommitCustom `json:"custom"`
	Testnet               bool         `json:"testnet"`
}

func Init(logger *zap.Logger, options ConnectorOptions) base.IConnector {
	once.Do(func() {
		connDefs := []*connectorPB.DestinationConnectorDefinition{}

		loader := configLoader.InitJSONSchema(logger)

		connector = &Connector{
			BaseConnector: base.BaseConnector{Logger: logger},
			Options:       options,
		}
		if options.SharedToken.Enabled && options.SharedToken.Token != "" {
			loader.Load(destinationDefinitionsYaml, destinationSpecsSharedTokenYaml, &connDefs)
			for idx := range connDefs {
				err := connector.AddConnectorDefinition(uuid.FromStringOrNil(connDefs[idx].GetUid()), connDefs[idx].GetId(), connDefs[idx])
				if err != nil {
					logger.Warn(err.Error())
				}
			}
		} else if !options.SharedToken.Enabled {
			loader.Load(destinationDefinitionsYaml, destinationSpecsYaml, &connDefs)
			for idx := range connDefs {
				err := connector.AddConnectorDefinition(uuid.FromStringOrNil(connDefs[idx].GetUid()), connDefs[idx].GetId(), connDefs[idx])
				if err != nil {
					logger.Warn(err.Error())
				}
			}
		}
	})
	return connector
}

func (con *Connection) getToken() string {
	if !con.connector.Options.SharedToken.Enabled {
		return fmt.Sprintf("token %s", con.config.GetFields()["captureToken"].GetStringValue())
	}
	return fmt.Sprintf("token %s", con.connector.Options.SharedToken.Token)
}

func (con *Connection) getCreatorName() string {
	return con.config.GetFields()["creatorName"].GetStringValue()
}

func (con *Connection) getLicense() string {
	return con.config.GetFields()["license"].GetStringValue()
}

func (con *Connection) pinFile(b64str string) (string, string, error) {

	var b bytes.Buffer

	w := multipart.NewWriter(&b)
	var fw io.Writer

	data, err := base64.StdEncoding.DecodeString(b64str)
	if err != nil {
		return "", "", err
	}

	if fw, err = w.CreateFormFile("file", "file.jpg"); err != nil {
		return "", "", err
	}

	if _, err = io.Copy(fw, bytes.NewReader(data)); err != nil {
		return "", "", err
	}

	h := sha256.New()

	if _, err := io.Copy(h, bytes.NewReader(data)); err != nil {
		log.Fatal(err)
	}

	w.Close()
	sha256hash := fmt.Sprintf("%x", h.Sum(nil))

	req, err := http.NewRequest("POST", ApiUrlPin, &b)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", con.getToken())

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return "", "", err
	}

	defer res.Body.Close()

	if res.StatusCode == http.StatusOK {
		bodyBytes, err := io.ReadAll(res.Body)
		if err != nil {
			return "", "", err
		}
		var jsonRes map[string]interface{}
		_ = json.Unmarshal(bodyBytes, &jsonRes)
		if cid, ok := jsonRes["cid"]; ok {
			return cid.(string), sha256hash, nil
		} else {
			return "", "", fmt.Errorf("pinFile failed")
		}

	}
	return "", "", fmt.Errorf("pinFile failed")

}

func (con *Connection) commit(commit Commit) (string, string, error) {

	marshalled, err := json.Marshal(commit)
	if err != nil {
		return "", "", nil
	}

	// return "", "", nil
	req, err := http.NewRequest("POST", ApiUrlCommit, bytes.NewReader(marshalled))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", con.getToken())

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return "", "", err
	}

	defer res.Body.Close()

	if res.StatusCode == http.StatusOK {
		bodyBytes, err := io.ReadAll(res.Body)
		if err != nil {
			return "", "", err
		}
		var jsonRes map[string]interface{}
		_ = json.Unmarshal(bodyBytes, &jsonRes)

		var assetCid string
		var assetTreeCid string
		if val, ok := jsonRes["assetCid"]; ok {
			assetCid = val.(string)
		} else {
			return "", "", fmt.Errorf("assetCid failed")
		}
		if val, ok := jsonRes["assetTreeCid"]; ok {
			assetTreeCid = val.(string)
		} else {
			return "", "", fmt.Errorf("assetTreeCid failed")
		}
		return assetCid, assetTreeCid, nil

	}
	return "", "", fmt.Errorf("commit failed")

}

func (c *Connector) CreateConnection(defUid uuid.UUID, config *structpb.Struct, logger *zap.Logger) (base.IConnection, error) {
	return &Connection{
		BaseConnection: base.BaseConnection{Logger: logger},
		config:         config,
		connector:      c,
	}, nil
}

func (con *Connection) Execute(input interface{}) (interface{}, error) {

	param := input.(common.WriteDestinationConnectorParam)

	dataOutputMap := map[string]interface{}{}

	for _, modelOutput := range param.ModelOutputs {
		if modelOutput.Task == modelv1alpha.Model_TASK_TEXT_TO_IMAGE {
			for taskIdx := range modelOutput.TaskOutputs {
				imageBase64 := modelOutput.TaskOutputs[taskIdx].Output.(*pipelinev1alpha.TaskOutput_TextToImage).TextToImage.Images[taskIdx]

				cid, sha256hash, err := con.pinFile(imageBase64)
				if err != nil {
					return nil, err
				}

				assetCid, _, err := con.commit(Commit{
					AssetCid:              cid,
					AssetSha256:           sha256hash,
					EncodingFormat:        "image/jpeg",
					AssetTimestampCreated: time.Now().Unix(),
					AssetCreator:          con.getCreatorName(),
					Abstract:              "Image Generation",
					Custom: CommitCustom{
						GeneratedBy:      strings.Split(modelOutput.Model, "/")[1],
						GeneratedThrough: "https://console.instill.tech",
						Prompt:           "",
						CreatorWallet:    "",
						License: CommitCustomLicense{
							Name: con.getLicense(),
						},
					},
					Testnet: false,
				})

				if err != nil {
					return nil, err
				}
				dataOutputMap[param.DataMappingIndices[taskIdx]] = map[string]string{}
				dataOutputMap[param.DataMappingIndices[taskIdx]].(map[string]string)["assetUrl"] = fmt.Sprintf("https://nftsearch.site/asset-profile?cid=%s", assetCid)

			}

		}

	}
	output := common.WriteDestinationConnectorOutput{
		DataOutputMap: dataOutputMap,
	}
	return output, nil

}

func (con *Connection) Test() (connectorPB.Connector_State, error) {

	req, err := http.NewRequest("GET", ApiUrlMe, nil)
	if err != nil {
		return connectorPB.Connector_STATE_ERROR, nil
	}
	req.Header.Set("Authorization", con.getToken())

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return connectorPB.Connector_STATE_ERROR, nil
	}
	if res.StatusCode == http.StatusOK {
		return connectorPB.Connector_STATE_CONNECTED, nil
	}
	return connectorPB.Connector_STATE_ERROR, nil
}
