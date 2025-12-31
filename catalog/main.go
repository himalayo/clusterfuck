package main

import (
	"context"
	"flag"
	"log"
	"os"

	configuration "github.com/himalayo/clusterfuck/api/configuration"
	networking "github.com/himalayo/clusterfuck/api/networking"
	_ "github.com/joho/godotenv/autoload"
	"github.com/redis/go-redis/v9"
)

var (
	networkingAddr    = flag.String("net_addr", "localhost:50051", "networking gRPC API address")
	configurationAddr = flag.String("cfg_addr", "localhost:50057", "configuration gRPC API address")
	Net               = networking.NewClient()
	events_cfg        = redis.Options{
		Addr:     os.Getenv("CATALOG_EVENTS_REDIS_ADDR"),
		Password: os.Getenv("CATALOG_EVENTS_REDIS_PASSWORD"),
		DB:       0,
	}
	data = NewDatabase(ConfigDatabaseFromEnv())
	Cfg  = configuration.NewClient()
)

var NetPub *networking.NetworkingPublisher

func main() {
	go data.LoadCatalogPagesFromDB(context.Background())
	go data.loadCatalogItemsFromDB(context.Background())
	netAddr, present := os.LookupEnv("NETWORKING_HOST")
	if !present {
		netAddr = *networkingAddr
	}
	confAddr, present := os.LookupEnv("CONFIGURATION_HOST")
	if !present {
		confAddr = *configurationAddr
	}
	go func() {
		Cfg.Listen(confAddr)

		res, err := Cfg.RegisterService(context.Background(), "catalog-service", configuration.RedisStreamData{
			Stream: "networking-events",
			Group:  "catalog-service",
			Instance: &redis.Options{
				Addr:     events_cfg.Addr,
				Password: events_cfg.Password,
				DB:       events_cfg.DB,
			},
		}, []int{}, NetPub.RedisConfig)
		if err != nil {
			log.Printf("Got error while registering service: %v", err)
		}
		log.Printf("Success: %v Status: %v", res.Success, res.Status)
	}()
	NetPub = networking.NewNetworkingPublisher(&redis.Options{
		Addr:     os.Getenv("CATALOG_OUTGOING_REDIS_ADDR"),
		Password: os.Getenv("CATALOG_OUTGOING_REDIS_PASSWORD"),
		DB:       0,
	})
	go Net.Listen(netAddr)

	Net.SetRedisSubscriber(&redis.Options{
		Addr:     events_cfg.Addr,
		Password: events_cfg.Password,
		DB:       0,
	}, "networking-events", "catalog-service")

	RegisterPacketHandlers()
	StartServer(data)
}
