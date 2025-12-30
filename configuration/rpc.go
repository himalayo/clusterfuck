package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand/v2"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"

	"database/sql"

	"github.com/google/uuid"
	api "github.com/himalayo/clusterfuck/api/configuration"
	pb "github.com/himalayo/clusterfuck/api/configuration/proto"
	"google.golang.org/grpc"
)

var (
	grpc_port = flag.Int("grpc_port", 50057, "The gRPC server port")
)

type server struct {
	pb.UnimplementedConfigurationServer
	data *Database
}

func unpackBoolResult(def *pb.ConfigurationResult) *pb.BooleanConfiguration {
	if def == nil {
		return nil
	}
	return def.GetBooleanConfig()
}

func unpackIntResult(def *pb.ConfigurationResult) *pb.IntConfiguration {
	if def == nil {
		return nil
	}
	return def.GetIntConfig()
}

func unpackStringResult(def *pb.ConfigurationResult) *pb.StringConfiguration {
	if def == nil {
		return nil
	}
	return def.GetStringConfig()
}

func unpackDoubleResult(def *pb.ConfigurationResult) *pb.DoubleConfiguration {
	if def == nil {
		return nil
	}
	return def.GetDoubleConfig()
}

func (s *server) GetBoolean(_ context.Context, in *pb.ConfigurationRequest) (*pb.BooleanConfiguration, error) {
	key := in.GetKey()
	def := unpackBoolResult(in.GetDefaultResult())
	config, err := s.data.GetBoolean(key)
	if err != nil {
		if err == sql.ErrNoRows {
			return def, nil
		}
		return nil, err
	}
	return &pb.BooleanConfiguration{Config: config}, nil
}

func (s *server) GetInt(_ context.Context, in *pb.ConfigurationRequest) (*pb.IntConfiguration, error) {
	key := in.GetKey()
	def := unpackIntResult(in.GetDefaultResult())
	config, err := s.data.GetInt(key)
	if err != nil {
		if err == sql.ErrNoRows {
			return def, nil
		}
		return nil, err
	}
	return &pb.IntConfiguration{Config: int32(config)}, nil
}

func (s *server) GetDouble(_ context.Context, in *pb.ConfigurationRequest) (*pb.DoubleConfiguration, error) {
	key := in.GetKey()
	def := unpackDoubleResult(in.GetDefaultResult())
	config, err := s.data.GetDouble(key)
	if err != nil {
		if err == sql.ErrNoRows {
			return def, nil
		}
		return nil, err
	}
	return &pb.DoubleConfiguration{Config: config}, nil
}

func (s *server) GetString(_ context.Context, in *pb.ConfigurationRequest) (*pb.StringConfiguration, error) {
	key := in.GetKey()
	def := unpackStringResult(in.GetDefaultResult())
	config, err := s.data.GetString(key)
	if err != nil {
		if err == sql.ErrNoRows {
			return def, nil
		}
		return nil, err
	}
	return &pb.StringConfiguration{Config: config}, nil
}

type NetworkingUpdateEvent struct {
	Id     string
	Type   string
	Values map[string]interface{}
}

func (e *NetworkingUpdateEvent) GetId() string {
	return e.Id
}

func (e *NetworkingUpdateEvent) GetType() string {
	return e.Type
}

func (e *NetworkingUpdateEvent) GetValues() map[string]interface{} {
	return e.Values
}

func newNetworkingUpdateEvent(in *pb.InstanceRegistration, eventType string) *NetworkingUpdateEvent {
	var evt NetworkingUpdateEvent
	id_uuid, err := uuid.NewRandom()
	var id string
	if err != nil {
		id = fmt.Sprintf("%v", rand.Float64())
	} else {
		id = id_uuid.String()
	}
	evt.Id = id
	evt.Type = eventType
	evt.Values = make(map[string]interface{})
	evt.Values["id"] = id
	evt.Values["type"] = evt.Type
	evt.Values["incoming_service"] = in.Incoming.Service
	evt.Values["incoming_stream"] = in.Incoming.Stream.GetStream()
	evt.Values["incoming_group"] = in.Incoming.Stream.GetGroup()
	evt.Values["incoming_redis_address"] = in.Incoming.GetStream().Instance.GetAddress()
	evt.Values["incoming_redis_password"] = in.Incoming.GetStream().Instance.GetPassword()
	evt.Values["incoming_redis_db"] = fmt.Sprintf("%d", in.Incoming.GetStream().Instance.GetDb())

	headers_string_list := make([]string, len(in.Incoming.Headers))
	for i, header := range in.Incoming.Headers {
		headers_string_list[i] = fmt.Sprintf("%d", header)
	}

	evt.Values["incoming_headers"] = strings.Join(headers_string_list, ";")

	evt.Values["outgoing_serivce"] = in.Outgoing.Service
	evt.Values["outgoing_redis_address"] = in.Outgoing.Instance.GetAddress()
	evt.Values["outgoing_redis_password"] = in.Outgoing.Instance.GetPassword()
	evt.Values["outgoing_redis_db"] = in.Outgoing.Instance.GetDb()

	return &evt
}

func (s *server) RegisterService(ctx context.Context, in *pb.InstanceRegistration) (*pb.RegistrationResult, error) {

	exists, err := data.rdb.Exists(ctx, fmt.Sprintf("incoming_service:%s", in.Incoming.Service)).Result()
	if err != nil {
		return nil, err
	}
	status := pb.RegistrationStatus_CREATED

	var wg sync.WaitGroup
	err_ch := make(chan error, 3)
	eventType := api.NetworkingRegisterEventType
	if exists != 0 {
		status = pb.RegistrationStatus_UPDATED
		eventType = api.NetworkingUpdateEventType
	} else {
		wg.Go(
			func() {
				headers_string_list := make([]string, len(in.Incoming.Headers))
				for i, header := range in.Incoming.Headers {
					headers_string_list[i] = fmt.Sprintf("%d", header)
				}

				incoming_headers := strings.Join(headers_string_list, ";")
				err := s.data.StoreIncomingService(ctx, &IncomingService{
					Service:  in.Incoming.Service,
					Stream:   in.Incoming.Stream.Stream,
					Address:  in.Incoming.Stream.Instance.Address,
					Password: in.Incoming.Stream.Instance.Password,
					DB:       int(in.Incoming.Stream.Instance.Db),
					Headers:  incoming_headers,
				})
				err_ch <- err
			},
		)
	}

	exists, err = data.rdb.Exists(ctx, fmt.Sprintf("outgoing_service:%s", in.Outgoing.Service)).Result()
	if err != nil {
		return nil, err
	}

	if exists != 0 {
		if status == pb.RegistrationStatus_CREATED {
			status = pb.RegistrationStatus_UPDATED
			eventType = api.NetworkingUpdateEventType
		} else {
			status = pb.RegistrationStatus_UNCHANGED
			eventType = api.NetworkingUnchangedEventType
		}
	} else {
		wg.Go(
			func() {
				err := s.data.StoreOutgoingService(ctx, &OutgoingService{
					Service:  in.Outgoing.Service,
					Address:  in.Outgoing.Instance.Address,
					Password: in.Outgoing.Instance.Password,
					DB:       int(in.Outgoing.Instance.Db),
				})
				err_ch <- err
			},
		)
	}

	wg.Go(
		func() {
			evt := newNetworkingUpdateEvent(in, eventType)
			err := s.data.networking_publisher.Publish(ctx, evt)
			err_ch <- err
		},
	)

	wg.Wait()
	close(err_ch)
	for e := range err_ch {
		if e != nil {
			return nil, e
		}
	}

	return &pb.RegistrationResult{
		Success: true,
		Status:  status,
	}, nil
}

func (s *server) GetNetworkingConfiguration(ctx context.Context, in *pb.NetworkingConfigurationRequest) (*pb.NetworkingConfiguration, error) {
	incoming, outgoing, err := s.data.GetServices(ctx)
	if err != nil {
		return nil, err
	}

	return &pb.NetworkingConfiguration{
		UpdateStream: &pb.RedisStream{
			Stream: s.data.networking_publisher.Stream,
			Group:  "",
			Instance: &pb.RedisInstance{
				Address:  s.data.networking_publisher.RedisConfig.Addr,
				Password: s.data.networking_publisher.RedisConfig.Password,
				Db:       int32(s.data.networking_publisher.RedisConfig.DB),
			},
		},
		Incoming: incoming,
		Outgoing: outgoing,
	}, nil
}

func StartServer(data *Database) {
	_, portString, _ := strings.Cut(os.Getenv("CONFIGURATION_HOST"), ":")
	port, err := strconv.Atoi(portString)
	if err != nil {
		port = *grpc_port
	}
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("Could not start gRPC socket: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterConfigurationServer(s, &server{data: data})
	if err := s.Serve(lis); err != nil {
		log.Fatalf("gRPC error: %v", err)
	}
}
