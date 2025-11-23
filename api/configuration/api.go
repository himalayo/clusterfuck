package api

import (
	"context"
	"log"
	"time"

	pb "github.com/himalayo/clusterfuck/api/configuration/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type ConfigurationClient struct {
	client pb.ConfigurationClient
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

func (n *ConfigurationClient) Listen(addr string) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("could not connect to networking module: %v", err)
	}
	n.client = pb.NewConfigurationClient(conn)
}
