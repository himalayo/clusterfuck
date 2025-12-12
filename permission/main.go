package main

import (
	"flag"
	"os"

	networking "github.com/himalayo/clusterfuck/api/networking"
	listener "github.com/himalayo/clusterfuck/api/networking/listener/server"
	player "github.com/himalayo/clusterfuck/api/player"
	_ "github.com/joho/godotenv/autoload"
)

var (
	networkingAddr = flag.String("net_addr", "localhost:50051", "networking gRPC API address")
	playerAddr     = flag.String("player_addr", "localhost:50053", "player gRPC API address")
	Net            = networking.NewClient()
	Incoming       = listener.NewIncomingServer()
	Player         = player.NewClient()
	db             = NewDatabase(ConfigDatabaseFromEnv())
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
	go Player.Listen(plAddr)
	RegisterIncomingHandlers()
	db.GetAllRanks()
	StartServer(db)
}
