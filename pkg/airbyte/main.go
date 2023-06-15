package airbyte

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "embed"

	"github.com/allegro/bigcache"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/gofrs/uuid"
	"go.uber.org/zap"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"

	dockerclient "github.com/docker/docker/client"

	"github.com/instill-ai/connector/pkg/base"
	"github.com/instill-ai/connector/pkg/configLoader"

	connectorPB "github.com/instill-ai/protogen-go/vdp/connector/v1alpha"
)

//go:embed config/seed/destination_definitions.yaml
var destinationDefinitionsYaml []byte

//go:embed config/seed/destination_specs.yaml
var destinationSpecsYaml []byte

var once sync.Once
var connector base.IConnector

// TODO: should be refactor using vdp protocol

type Connector struct {
	base.BaseConnector
	dockerClient *dockerclient.Client
	cache        *bigcache.BigCache
	options      ConnectorOptions
}

type ConnectorOptions struct {
	MountSourceVDP     string
	MountTargetVDP     string
	MountSourceAirbyte string
	MountTargetAirbyte string
	VDPProtocolPath    string
}

type Connection struct {
	base.BaseConnection
	connector *Connector
	defUid    uuid.UUID
	config    *structpb.Struct
}

func Init(logger *zap.Logger, options ConnectorOptions) base.IConnector {
	once.Do(func() {
		connDefs := []*connectorPB.DestinationConnectorDefinition{}

		loader := configLoader.InitJSONSchema(logger)
		loader.Load(destinationDefinitionsYaml, destinationSpecsYaml, &connDefs)

		dockerClient, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
		if err != nil {
			logger.Error(err.Error())
		}
		// defer dockerClient.Close()
		cache, err := bigcache.NewBigCache(bigcache.DefaultConfig(60 * time.Minute))
		if err != nil {
			logger.Error(err.Error())
		}

		connector = &Connector{
			BaseConnector: base.BaseConnector{Logger: logger},
			dockerClient:  dockerClient,
			cache:         cache,
			options:       options,
		}
		for idx := range connDefs {
			err := connector.AddConnectorDefinition(uuid.FromStringOrNil(connDefs[idx].GetUid()), connDefs[idx].GetId(), connDefs[idx])
			if err != nil {
				logger.Warn(err.Error())
			}
		}
		InitAirbyteCatalog(logger, options.VDPProtocolPath)

	})
	return connector
}

func (c *Connector) CreateConnection(defUid uuid.UUID, config *structpb.Struct, logger *zap.Logger) (base.IConnection, error) {

	return &Connection{
		BaseConnection: base.BaseConnection{Logger: logger},
		connector:      c,
		defUid:         defUid,
		config:         config,
	}, nil
}

func (con *Connection) Execute(input []*connectorPB.DataPayload) ([]*connectorPB.DataPayload, error) {

	// Create ConfiguredAirbyteCatalog
	cfgAbCatalog := ConfiguredAirbyteCatalog{
		Streams: []ConfiguredAirbyteStream{
			{
				Stream:              &TaskOutputAirbyteCatalog.Streams[0],
				SyncMode:            "full_refresh", // TODO: config
				DestinationSyncMode: "append",       // TODO: config
			},
		},
	}

	byteCfgAbCatalog, err := json.Marshal(&cfgAbCatalog)
	if err != nil {
		return nil, fmt.Errorf("marshal AirbyteMessage error: %w", err)
	}

	// Create AirbyteMessage RECORD type, i.e., AirbyteRecordMessage in JSON Line format
	var byteAbMsgs []byte

	// TODO: should define new vdp_protocol for this
	for idx, dataPayload := range input {

		for modelName, taskOutput := range dataPayload.Json.GetFields() {
			b, err := protojson.MarshalOptions{
				UseProtoNames:   true,
				EmitUnpopulated: true,
			}.Marshal(taskOutput)
			if err != nil {
				return nil, fmt.Errorf("task_outputs[%d] error: %w", idx, err)
			}

			dataStruct := structpb.Struct{}
			err = protojson.Unmarshal(b, &dataStruct)
			if err != nil {
				return nil, fmt.Errorf("task_outputs[%d] error: %w", idx, err)
			}
			dataStruct.GetFields()["index"] = structpb.NewStringValue(dataPayload.DataMappingIndex)
			dataStruct.GetFields()["model"] = structpb.NewStringValue(modelName)
			dataStruct.GetFields()["pipeline"] = dataPayload.GetMetadata().GetFields()["pipeline"]

			b, err = protojson.Marshal(&dataStruct)
			if err != nil {
				return nil, fmt.Errorf("task_outputs[%d] error: %w", idx, err)
			}

			abMsg := AirbyteMessage{}
			abMsg.Type = "RECORD"
			abMsg.Record = &AirbyteRecordMessage{
				Stream:    TaskOutputAirbyteCatalog.Streams[0].Name,
				Data:      b,
				EmittedAt: time.Now().UnixMilli(),
			}

			b, err = json.Marshal(&abMsg)
			if err != nil {
				return nil, fmt.Errorf("Marshal AirbyteMessage error: %w", err)
			}
			b = []byte(string(b) + "\n")
			byteAbMsgs = append(byteAbMsgs, b...)
		}

	}

	// Remove the last "\n"
	byteAbMsgs = byteAbMsgs[:len(byteAbMsgs)-1]

	connDef, err := con.connector.GetConnectorDefinitionByUid(con.defUid)
	if err != nil {
		return nil, err
	}
	imageName := fmt.Sprintf("%s:%s", connDef.GetConnectorDefinition().GetDockerRepository(), connDef.GetConnectorDefinition().GetDockerImageTag())
	containerName := fmt.Sprintf("%s.%d.write", con.defUid, time.Now().UnixNano())
	configFileName := fmt.Sprintf("%s.%d.write", con.defUid, time.Now().UnixNano())
	catalogFileName := fmt.Sprintf("%s.%d.write", con.defUid, time.Now().UnixNano())

	// If there is already a container run dispatched in the previous attempt, return exitCodeOK directly
	if _, err := con.connector.cache.Get(containerName); err == nil {
		return nil, nil
	}

	// Write config into a container local file (always overwrite)
	configFilePath := fmt.Sprintf("%s/connector-data/config/%s.json", con.connector.options.MountTargetVDP, configFileName)
	if err := os.MkdirAll(filepath.Dir(configFilePath), os.ModePerm); err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("unable to create folders for filepath %s", configFilePath), "WriteContainerLocalFileError", err)
	}

	configuration := func() []byte {
		if con.config != nil {
			b, err := con.config.MarshalJSON()
			if err != nil {
				con.Logger.Error(err.Error())
			}
			return b
		}
		return []byte{}
	}()
	if err := os.WriteFile(configFilePath, configuration, 0644); err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("unable to write connector config file %s", configFilePath), "WriteContainerLocalFileError", err)
	}

	// Write catalog into a container local file (always overwrite)
	catalogFilePath := fmt.Sprintf("%s/connector-data/catalog/%s.json", con.connector.options.MountTargetVDP, catalogFileName)
	if err := os.MkdirAll(filepath.Dir(catalogFilePath), os.ModePerm); err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("unable to create folders for filepath %s", catalogFilePath), "WriteContainerLocalFileError", err)
	}
	if err := os.WriteFile(catalogFilePath, byteCfgAbCatalog, 0644); err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("unable to write connector catalog file %s", catalogFilePath), "WriteContainerLocalFileError", err)
	}

	defer func() {
		// Delete config local file
		if _, err := os.Stat(configFilePath); err == nil {
			if err := os.Remove(configFilePath); err != nil {
				con.Logger.Error(fmt.Sprintln("Activity", "ImageName", imageName, "ContainerName", containerName, "Error", err))
			}
		}

		// Delete catalog local file
		if _, err := os.Stat(catalogFilePath); err == nil {
			if err := os.Remove(catalogFilePath); err != nil {
				con.Logger.Error(fmt.Sprintln("Activity", "ImageName", imageName, "ContainerName", containerName, "Error", err))
			}
		}
	}()

	out, err := con.connector.dockerClient.ImagePull(context.Background(), imageName, types.ImagePullOptions{})
	if err != nil {
		return nil, err
	}
	defer out.Close()

	if _, err := io.Copy(os.Stdout, out); err != nil {
		return nil, err
	}

	resp, err := con.connector.dockerClient.ContainerCreate(context.Background(),
		&container.Config{
			Image:        imageName,
			AttachStdin:  true,
			AttachStdout: true,
			OpenStdin:    true,
			StdinOnce:    true,
			Tty:          true,
			Cmd: []string{
				"write",
				"--config",
				configFilePath,
				"--catalog",
				catalogFilePath,
			},
		},
		&container.HostConfig{
			Mounts: []mount.Mount{
				{
					Type: func() mount.Type {
						if string(con.connector.options.MountSourceVDP[0]) == "/" {
							return mount.TypeBind
						}
						return mount.TypeVolume
					}(),
					Source: con.connector.options.MountSourceVDP,
					Target: con.connector.options.MountTargetVDP,
				},
				{
					Type: func() mount.Type {
						if string(con.connector.options.MountSourceVDP[0]) == "/" {
							return mount.TypeBind
						}
						return mount.TypeVolume
					}(),
					Source: con.connector.options.MountSourceAirbyte,
					Target: con.connector.options.MountTargetAirbyte,
				},
			},
		},
		nil, nil, containerName)
	if err != nil {
		return nil, err
	}

	hijackedResp, err := con.connector.dockerClient.ContainerAttach(context.Background(), resp.ID, types.ContainerAttachOptions{
		Stdout: true,
		Stdin:  true,
		Stream: true,
	})
	if err != nil {
		return nil, err
	}

	// need to append "\n" and "ctrl+D" at the end of the input message
	_, err = hijackedResp.Conn.Write(append(byteAbMsgs, 10, 4))
	if err != nil {
		return nil, err
	}

	if err := con.connector.dockerClient.ContainerStart(context.Background(), resp.ID, types.ContainerStartOptions{}); err != nil {
		return nil, err
	}

	var bufStdOut bytes.Buffer
	if _, err := bufStdOut.ReadFrom(hijackedResp.Reader); err != nil {
		return nil, err
	}

	if err := con.connector.dockerClient.ContainerRemove(context.Background(), resp.ID,
		types.ContainerRemoveOptions{
			RemoveVolumes: true,
			Force:         true,
		}); err != nil {
		return nil, err
	}

	// Set cache flag (empty value is fine since we need only the entry record)
	if err := con.connector.cache.Set(containerName, []byte{}); err != nil {
		return nil, err
	}

	con.Logger.Info(fmt.Sprintln("Activity",
		"ImageName", imageName,
		"ContainerName", containerName,
		"STDOUT", bufStdOut.String()))

	// Delete the cache entry only after the write completed
	if err := con.connector.cache.Delete(containerName); err != nil {
		con.Logger.Error(err.Error())
	}
	return nil, nil
}

func (con *Connection) Test() (connectorPB.Connector_State, error) {

	def, err := con.connector.GetConnectorDefinitionByUid(con.defUid)
	if err != nil {
		return connectorPB.Connector_STATE_UNSPECIFIED, err
	}
	imageName := fmt.Sprintf("%s:%s", def.GetConnectorDefinition().DockerRepository, def.GetConnectorDefinition().DockerImageTag)
	containerName := fmt.Sprintf("%s.%d.check", con.defUid, time.Now().UnixNano())
	configFilePath := fmt.Sprintf("%s/connector-data/config/%s.json", con.connector.options.MountTargetVDP, containerName)

	// Write config into a container local file
	if err := os.MkdirAll(filepath.Dir(configFilePath), os.ModePerm); err != nil {
		return connectorPB.Connector_STATE_UNSPECIFIED, fmt.Errorf(fmt.Sprintf("unable to create folders for filepath %s", configFilePath), "WriteContainerLocalFileError", err)
	}

	configuration := func() []byte {
		if con.config != nil {
			b, err := con.config.MarshalJSON()
			if err != nil {
				con.Logger.Error(err.Error())
			}
			return b
		}
		return []byte{}
	}()

	if err := os.WriteFile(configFilePath, configuration, 0644); err != nil {
		return connectorPB.Connector_STATE_UNSPECIFIED, fmt.Errorf(fmt.Sprintf("unable to write connector config file %s", configFilePath), "WriteContainerLocalFileError", err)
	}

	defer func() {
		// Delete config local file
		if _, err := os.Stat(configFilePath); err == nil {
			if err := os.Remove(configFilePath); err != nil {
				con.Logger.Error(fmt.Sprintf("ImageName: %s, ContainerName: %s, Error: %v", imageName, containerName, err))
			}
		}
	}()

	out, err := con.connector.dockerClient.ImagePull(context.Background(), imageName, types.ImagePullOptions{})
	if err != nil {
		return connectorPB.Connector_STATE_UNSPECIFIED, err
	}
	defer out.Close()

	if _, err := io.Copy(os.Stdout, out); err != nil {
		return connectorPB.Connector_STATE_UNSPECIFIED, err
	}

	resp, err := con.connector.dockerClient.ContainerCreate(context.Background(),
		&container.Config{
			Image: imageName,
			Tty:   false,
			Cmd: []string{
				"check",
				"--config",
				configFilePath,
			},
		},
		&container.HostConfig{
			Mounts: []mount.Mount{
				{
					Type: func() mount.Type {
						if string(con.connector.options.MountSourceVDP[0]) == "/" {
							return mount.TypeBind
						}
						return mount.TypeVolume
					}(),
					Source: con.connector.options.MountSourceVDP,
					Target: con.connector.options.MountTargetVDP,
				},
			},
		},
		nil, nil, containerName)
	if err != nil {
		return connectorPB.Connector_STATE_UNSPECIFIED, err
	}

	if err := con.connector.dockerClient.ContainerStart(context.Background(), resp.ID, types.ContainerStartOptions{}); err != nil {
		return connectorPB.Connector_STATE_UNSPECIFIED, err
	}

	statusCh, errCh := con.connector.dockerClient.ContainerWait(context.Background(), resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return connectorPB.Connector_STATE_UNSPECIFIED, err
		}
	case <-statusCh:
	}

	if out, err = con.connector.dockerClient.ContainerLogs(context.Background(),
		resp.ID,
		types.ContainerLogsOptions{
			ShowStdout: true,
		},
	); err != nil {
		return connectorPB.Connector_STATE_UNSPECIFIED, err
	}

	if err := con.connector.dockerClient.ContainerRemove(context.Background(), containerName,
		types.ContainerRemoveOptions{
			RemoveVolumes: true,
			Force:         true,
		}); err != nil {
		return connectorPB.Connector_STATE_UNSPECIFIED, err
	}

	var bufStdOut, bufStdErr bytes.Buffer
	if _, err := stdcopy.StdCopy(&bufStdOut, &bufStdErr, out); err != nil {
		return connectorPB.Connector_STATE_UNSPECIFIED, err
	}

	var teeStdOut io.Reader = strings.NewReader(bufStdOut.String())
	var teeStdErr io.Reader = strings.NewReader(bufStdErr.String())
	teeStdOut = io.TeeReader(teeStdOut, &bufStdOut)
	teeStdErr = io.TeeReader(teeStdErr, &bufStdErr)

	var byteStdOut, byteStdErr []byte
	if byteStdOut, err = io.ReadAll(teeStdOut); err != nil {
		return connectorPB.Connector_STATE_UNSPECIFIED, err
	}
	if byteStdErr, err = io.ReadAll(teeStdErr); err != nil {
		return connectorPB.Connector_STATE_UNSPECIFIED, err
	}

	con.Logger.Info(fmt.Sprintf("ImageName, %s, ContainerName, %s, STDOUT, %v, STDERR, %v", imageName, containerName, byteStdOut, byteStdErr))

	scanner := bufio.NewScanner(&bufStdOut)
	for scanner.Scan() {

		if err := scanner.Err(); err != nil {
			return connectorPB.Connector_STATE_UNSPECIFIED, err
		}

		var jsonMsg map[string]interface{}
		if err := json.Unmarshal(scanner.Bytes(), &jsonMsg); err == nil {
			switch jsonMsg["type"] {
			case "CONNECTION_STATUS":
				switch jsonMsg["connectionStatus"].(map[string]interface{})["status"] {
				case "SUCCEEDED":
					return connectorPB.Connector_STATE_CONNECTED, nil
				case "FAILED":
					return connectorPB.Connector_STATE_ERROR, nil
				default:
					return connectorPB.Connector_STATE_ERROR, fmt.Errorf("UNKNOWN STATUS")
				}
			}
		}
	}
	return connectorPB.Connector_STATE_UNSPECIFIED, nil
}
