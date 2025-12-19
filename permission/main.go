package main

import (
	"flag"
	"os"

	networking "github.com/himalayo/clusterfuck/api/networking"
	listener "github.com/himalayo/clusterfuck/api/networking/listener/server"
	player "github.com/himalayo/clusterfuck/api/player"
	subscription "github.com/himalayo/clusterfuck/api/subscription"
	_ "github.com/joho/godotenv/autoload"
	"github.com/redis/go-redis/v9"
)

var (
	networkingAddr   = flag.String("net_addr", "localhost:50051", "networking gRPC API address")
	playerAddr       = flag.String("player_addr", "localhost:50053", "player gRPC API address")
	subscriptionAddr = flag.String("sub_addr", "localhost:50052", "subscription gRPC API address")
	Net              = networking.NewClient()
	Sub              = subscription.NewClient()
	Incoming         = listener.NewIncomingServer()
	Player, _        = player.NewClient("permission-service", &redis.Options{
		Addr:     os.Getenv("PLAYER_EVENTS_REDIS_ADDR"),
		Password: os.Getenv("PLAYER_EVENTS_REDIS_PASSWORD"),
		DB:       0,
	})
	db = NewDatabase(ConfigDatabaseFromEnv())
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
	go Sub.Listen(subAddr)
	go Net.Listen(netAddr)
	go Player.Listen(plAddr)
	RegisterIncomingHandlers()
	db.GetAllRanks()
	StartServer(db)
}
