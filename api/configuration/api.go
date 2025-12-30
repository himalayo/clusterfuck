package api

import (
	"context"
	"log"
	"strconv"
	"strings"
	"time"

	pb "github.com/himalayo/clusterfuck/api/configuration/proto"
	"github.com/himalayo/clusterfuck/api/events"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type ConfigurationClient struct {
	client pb.ConfigurationClient
	sub    *events.RedisSubscriber
}

func NewClient() *ConfigurationClient {
	return &ConfigurationClient{}
}

func DefaultIntResult(value int) *pb.ConfigurationResult {
	return &pb.ConfigurationResult{Value: &pb.ConfigurationResult_IntConfig{
		IntConfig: &pb.IntConfiguration{
			Config: int32(value),
		},
	},
	}
}

func DefaultDoubleResult(value float64) *pb.ConfigurationResult {
	return &pb.ConfigurationResult{Value: &pb.ConfigurationResult_DoubleConfig{
		DoubleConfig: &pb.DoubleConfiguration{
			Config: value,
		},
	},
	}
}

func DefaultStringResult(value string) *pb.ConfigurationResult {
	return &pb.ConfigurationResult{Value: &pb.ConfigurationResult_StringConfig{
		StringConfig: &pb.StringConfiguration{
			Config: value,
		},
	},
	}
}

func DefaultBooleanResult(value bool) *pb.ConfigurationResult {
	return &pb.ConfigurationResult{Value: &pb.ConfigurationResult_BooleanConfig{
		BooleanConfig: &pb.BooleanConfiguration{
			Config: value,
		},
	},
	}
}

func (n *ConfigurationClient) GetInt(key string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := n.client.GetInt(ctx, &pb.ConfigurationRequest{Key: key})
	if err != nil {
		return 0, err
	}
	return int(result.GetConfig()), nil
}

func (n *ConfigurationClient) GetIntOrDefault(key string, def int) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := n.client.GetInt(ctx, &pb.ConfigurationRequest{
		Key:           key,
		DefaultResult: DefaultIntResult(def),
	})
	if err != nil {
		return 0, err
	}
	return int(result.GetConfig()), nil
}

func (n *ConfigurationClient) GetDouble(key string) (float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := n.client.GetDouble(ctx, &pb.ConfigurationRequest{Key: key})
	if err != nil {
		return 0, err
	}
	return result.GetConfig(), nil
}

func (n *ConfigurationClient) GetDoubleOrDefault(key string, def float64) (float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := n.client.GetDouble(ctx, &pb.ConfigurationRequest{
		Key:           key,
		DefaultResult: DefaultDoubleResult(def),
	})
	if err != nil {
		return 0, err
	}
	return result.GetConfig(), nil
}

func (n *ConfigurationClient) GetBoolean(key string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := n.client.GetBoolean(ctx, &pb.ConfigurationRequest{
		Key: key,
	})
	if err != nil {
		return false, err
	}
	return result.GetConfig(), nil
}

func (n *ConfigurationClient) GetBooleanOrDefault(key string, def bool) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := n.client.GetBoolean(ctx, &pb.ConfigurationRequest{
		Key:           key,
		DefaultResult: DefaultBooleanResult(def),
	})
	if err != nil {
		return false, err
	}
	return result.GetConfig(), nil
}

func (n *ConfigurationClient) GetString(key string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := n.client.GetString(ctx, &pb.ConfigurationRequest{
		Key: key,
	})
	if err != nil {
		return "", err
	}
	return result.GetConfig(), nil
}

func (n *ConfigurationClient) GetStringOrDefault(key string, def string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := n.client.GetString(ctx, &pb.ConfigurationRequest{
		Key:           key,
		DefaultResult: DefaultStringResult(def),
	})
	if err != nil {
		return "", err
	}
	return result.GetConfig(), nil
}

type RedisStreamData struct {
	Stream   string
	Group    string
	Instance *redis.Options
}

func (n *ConfigurationClient) RegisterService(ctx context.Context, service string, incomingStreamData RedisStreamData, incomingHeaders []int, outgoingRedis *redis.Options) (*pb.RegistrationResult, error) {
	headers := make([]int32, len(incomingHeaders))
	for i, h := range incomingHeaders {
		headers[i] = int32(h)
	}

	result, err := n.client.RegisterService(ctx, &pb.InstanceRegistration{
		Incoming: &pb.IncomingService{
			Service: service,
			Stream: &pb.RedisStream{
				Stream: incomingStreamData.Stream,
				Group:  incomingStreamData.Group,
				Instance: &pb.RedisInstance{
					Address:  incomingStreamData.Instance.Addr,
					Password: incomingStreamData.Instance.Password,
					Db:       int32(incomingStreamData.Instance.DB),
				},
			},
			Headers: headers,
		},
		Outgoing: &pb.OutgoingService{
			Service: service,
			Instance: &pb.RedisInstance{
				Address:  outgoingRedis.Addr,
				Password: outgoingRedis.Password,
				Db:       int32(outgoingRedis.DB),
			},
		},
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

const (
	NetworkingUpdateEventType    = "NETWORKING_CONFIGURATION_UPDATE"
	NetworkingRegisterEventType  = "NETWORKING_CONFIGURATION_REGISTER"
	NetworkingUnchangedEventType = "NETWORKING_CONFIGURATION_NOOP"
)

func GenericEventToNetworkConfigurationEvent(evt events.Event) *pb.NetworkingConfigurationEvent {
	id := evt.GetId()
	eventType := evt.GetType()

	if (eventType != NetworkingRegisterEventType) && (eventType != NetworkingUpdateEventType) {
		return nil
	}

	values := evt.GetValues()
	headers_strings := strings.Split(values["incoming_headers"].(string), ";")
	headers := make([]int32, 0, len(headers_strings))
	for _, header_string := range headers_strings {
		header_i64, err := strconv.ParseInt(header_string, 10, 32)
		if err == nil {
			headers = append(headers, int32(header_i64))
		}
	}
	incoming_db_i64, err := strconv.ParseInt(values["incoming_redis_db"].(string), 10, 32)
	incoming_db := int32(0)
	if err == nil {
		incoming_db = int32(incoming_db_i64)
	}
	outgoing_db_i64, err := strconv.ParseInt(values["outgoing_redis_db"].(string), 10, 32)
	outgoing_db := int32(0)
	if err == nil {
		outgoing_db = int32(outgoing_db_i64)
	}

	return &pb.NetworkingConfigurationEvent{
		Id:   id,
		Type: eventType,
		Incoming: &pb.IncomingService{
			Service: values["incoming_service"].(string),
			Stream: &pb.RedisStream{
				Stream: values["incoming_stream"].(string),
				Group:  values["incoming_group"].(string),
				Instance: &pb.RedisInstance{
					Address:  values["incoming_redis_address"].(string),
					Password: values["incoming_redis_password"].(string),
					Db:       incoming_db,
				},
			},
			Headers: headers,
		},
		Outgoing: &pb.OutgoingService{
			Service: values["outgoing_service"].(string),
			Instance: &pb.RedisInstance{
				Address:  values["outgoing_redis_address"].(string),
				Password: values["outgoing_redis_password"].(string),
				Db:       outgoing_db,
			},
		},
	}
}

func (n *ConfigurationClient) GetNetworkingConfiguration(ctx context.Context, group string) (*pb.NetworkingConfiguration, error) {
	result, err := n.client.GetNetworkingConfiguration(ctx, &pb.NetworkingConfigurationRequest{})
	if err != nil {
		return nil, err
	}

	if n.sub == nil {
		n.sub, err = events.NewRedisSubscriber(&redis.Options{
			Addr:     result.UpdateStream.Instance.Address,
			Password: result.UpdateStream.Instance.Password,
			DB:       int(result.UpdateStream.Instance.Db),
		}, result.UpdateStream.Stream, group)
		go n.sub.Listen(ctx)
	}

	return result, err
}

func (n *ConfigurationClient) RegisterNetworkingConfigurationEventHandler(eventType string, handler func(context.Context, *pb.NetworkingConfigurationEvent)) {
	n.sub.Subscribe(eventType, func(ctx context.Context, evt events.Event) {
		if (evt.GetType() != NetworkingRegisterEventType) && (evt.GetType() != NetworkingUpdateEventType) {
			return
		}
		handler(ctx, GenericEventToNetworkConfigurationEvent(evt))
	})
}

func (n *ConfigurationClient) Listen(addr string) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("could not connect to networking module: %v", err)
	}
	n.client = pb.NewConfigurationClient(conn)
}
