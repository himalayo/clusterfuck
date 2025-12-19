package main

import (
	"flag"
	"os"

	networking "github.com/himalayo/clusterfuck/api/networking"
	player "github.com/himalayo/clusterfuck/api/player"
	_ "github.com/joho/godotenv/autoload"
	"github.com/redis/go-redis/v9"
)

var (
	networkingAddr = flag.String("net_addr", "localhost:50051", "networking gRPC API address")
	playerAddr     = flag.String("player_addr", "localhost:50053", "player gRPC API address")
	Net            = networking.NewClient()
	Player, _      = player.NewClient("modtool-service", &redis.Options{
		Addr:     os.Getenv("PLAYER_EVENTS_REDIS_ADDR"),
		Password: os.Getenv("PLAYER_EVENTS_REDIS_PASSWORD"),
		DB:       0,
	})
	data = NewDatabase(ConfigDatabaseFromEnv())
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

	go Net.Listen(netAddr)
	RegisterLoginHandlers()
	go Player.Listen(plAddr)
	StartServer(data)
}
