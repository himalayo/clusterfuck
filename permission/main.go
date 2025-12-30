package main

import (
	"context"
	"flag"
	"log"
	"os"

	configuration "github.com/himalayo/clusterfuck/api/configuration"
	networking "github.com/himalayo/clusterfuck/api/networking"
	listener "github.com/himalayo/clusterfuck/api/networking/listener/server"
	player "github.com/himalayo/clusterfuck/api/player"
	subscription "github.com/himalayo/clusterfuck/api/subscription"
	_ "github.com/joho/godotenv/autoload"
	"github.com/redis/go-redis/v9"
)

var NetPub *networking.NetworkingPublisher

var (
	networkingAddr    = flag.String("net_addr", "localhost:50051", "networking gRPC API address")
	playerAddr        = flag.String("player_addr", "localhost:50053", "player gRPC API address")
	configurationAddr = flag.String("cfg_addr", "localhost:50057", "configuration gRPC API address")
	subscriptionAddr  = flag.String("sub_addr", "localhost:50052", "subscription gRPC API address")
	Net               = networking.NewClient()
	Sub               = subscription.NewClient()
	Incoming          = listener.NewIncomingServer()
	Player, _         = player.NewClient("permission-service", &redis.Options{
		Addr:     os.Getenv("PLAYER_EVENTS_REDIS_ADDR"),
		Password: os.Getenv("PLAYER_EVENTS_REDIS_PASSWORD"),
		DB:       0,
	})
	events_cfg = redis.Options{
		Addr:     os.Getenv("PERMISSION_EVENTS_REDIS_ADDR"),
		Password: os.Getenv("PERMISSION_EVENTS_REDIS_PASSWORD"),
		DB:       0,
	}
	Cfg = configuration.NewClient()
	db  = NewDatabase(ConfigDatabaseFromEnv())
)

func main() {

	plAddr, present := os.LookupEnv("PLAYER_HOST")
	if !present {
		plAddr = *playerAddr
	}

	netAddr, present := os.LookupEnv("NETWORKING_HOST")
	if !present {
		netAddr = *networkingAddr
	}
	subAddr, present := os.LookupEnv("SUBSCRIPTION_HOST")
	if !present {
		subAddr = *subscriptionAddr
	}
	confAddr, present := os.LookupEnv("CONFIGURATION_HOST")
	if !present {
		confAddr = *configurationAddr
	}
	go func() {
		Cfg.Listen(confAddr)

		res, err := Cfg.RegisterService(context.Background(), "permission-service", configuration.RedisStreamData{
			Stream: "networking-events",
			Group:  "permission-service",
			Instance: &redis.Options{
				Addr:     events_cfg.Addr,
				Password: events_cfg.Password,
				DB:       events_cfg.DB,
			},
		}, []int{357}, NetPub.RedisConfig)
		if err != nil {
			log.Printf("Got error while registering service: %v", err)
		}
		log.Printf("Success: %v Status: %v", res.Success, res.Status)
	}()
	NetPub = networking.NewNetworkingPublisher(&redis.Options{
		Addr:     os.Getenv("PERMISSION_OUTGOING_REDIS_ADDR"),
		Password: os.Getenv("PERMISSION_OUTGOING_REDIS_PASSWORD"),
		DB:       0,
	})
	go Sub.Listen(subAddr)
	go Net.Listen(netAddr)
	Net.SetRedisSubscriber(&redis.Options{
		Addr:     events_cfg.Addr,
		Password: events_cfg.Password,
		DB:       0,
	}, "networking-events", "permission-service")
	go Player.Listen(plAddr)
	RegisterIncomingHandlers()
	db.GetAllRanks()
	StartServer(db)
}
