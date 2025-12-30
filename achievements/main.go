package main

import (
	"context"
	"flag"
	"log"
	"os"

	configuration "github.com/himalayo/clusterfuck/api/configuration"
	networking "github.com/himalayo/clusterfuck/api/networking"
	player "github.com/himalayo/clusterfuck/api/player"
	_ "github.com/joho/godotenv/autoload"
	"github.com/redis/go-redis/v9"
)

var (
	networkingAddr    = flag.String("net_addr", "localhost:50051", "networking gRPC API address")
	playerAddr        = flag.String("player_addr", "localhost:50053", "player gRPC API address")
	Net               = networking.NewClient()
	configurationAddr = flag.String("cfg_addr", "localhost:50057", "configuration gRPC API address")
	Player, _         = player.NewClient("achievements-service", &redis.Options{
		Addr:     os.Getenv("PLAYER_EVENTS_REDIS_ADDR"),
		Password: os.Getenv("PLAYER_EVENTS_REDIS_PASSWORD"),
		DB:       0,
	})
	data       = NewDatabase(ConfigDatabaseFromEnv())
	events_cfg = redis.Options{
		Addr:     os.Getenv("ACHIEVEMENTS_EVENTS_REDIS_ADDR"),
		Password: os.Getenv("ACHIEVEMENTS_EVENTS_REDIS_PASSWORD"),
		DB:       0,
	}
	Cfg = configuration.NewClient()
)
var NetPub *networking.NetworkingPublisher

func main() {
	plAddr, present := os.LookupEnv("PLAYER_HOST")
	if !present {
		plAddr = *playerAddr
	}

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

		res, err := Cfg.RegisterService(context.Background(), "achievements-service", configuration.RedisStreamData{
			Stream: "networking-events",
			Group:  "achievements-service",
			Instance: &redis.Options{
				Addr:     events_cfg.Addr,
				Password: events_cfg.Password,
				DB:       events_cfg.DB,
			},
		}, []int{219}, NetPub.RedisConfig)
		if err != nil {
			log.Printf("Got error while registering service: %v", err)
		}
		log.Printf("Success: %v Status: %v", res.Success, res.Status)
	}()
	NetPub = networking.NewNetworkingPublisher(&redis.Options{
		Addr:     os.Getenv("ACHIEVEMENTS_OUTGOING_REDIS_ADDR"),
		Password: os.Getenv("ACHIEVEMENTS_OUTGOING_REDIS_PASSWORD"),
		DB:       0,
	})
	go Net.Listen(netAddr)

	Net.SetRedisSubscriber(&redis.Options{
		Addr:     events_cfg.Addr,
		Password: events_cfg.Password,
		DB:       0,
	}, "networking-events", "achievements-service")
	RegisterPacketHandlers()
	RegisterLoginHandlers()
	go Player.Listen(plAddr)
	StartServer(data)
}
