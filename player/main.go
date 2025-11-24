package main

import (
	"flag"
	"os"

	achievements "github.com/himalayo/clusterfuck/api/achievements"
	configuration "github.com/himalayo/clusterfuck/api/configuration"
	modtool "github.com/himalayo/clusterfuck/api/modtool"
	networking "github.com/himalayo/clusterfuck/api/networking"
	listener "github.com/himalayo/clusterfuck/api/networking/listener/server"
	permission "github.com/himalayo/clusterfuck/api/permission"
	subscription "github.com/himalayo/clusterfuck/api/subscription"
	_ "github.com/joho/godotenv/autoload"
)

var (
	networkingAddr    = flag.String("net_addr", "localhost:50051", "networking gRPC API address")
	subscriptionAddr  = flag.String("sub_addr", "localhost:50052", "subscription gRPC API address")
	permissionAddr    = flag.String("per_addr", "localhost:50054", "permission gRPC API address")
	modtoolAddr       = flag.String("mod_addr", "localhost:50056", "modtool gRPC API address")
	configurationAddr = flag.String("cfg_addr", "localhost:50057", "configuration gRPC API address")
	achievementsAddr  = flag.String("ach_addr", "localhost:50058", "achievements gRPC API address")
	Net               = networking.NewClient()
	Sub               = subscription.NewClient()
	Perm              = permission.NewClient()
	Mod               = modtool.NewClient()
	Cfg               = configuration.NewClient()
	Ach               = achievements.NewClient()
	Incoming          = listener.NewIncomingServer()
	Data              = NewDatabase(ConfigDatabaseFromEnv())
	Events            = NewEventListener(Data, Net, Sub, Perm, Mod, Cfg, Ach)
)

func main() {
	flag.Parse()

	netAddr, present := os.LookupEnv("NETWORKING_HOST")
	if !present {
		netAddr = *networkingAddr
	}

	subAddr, present := os.LookupEnv("SUBSCRIPTION_HOST")
	if !present {
		subAddr = *subscriptionAddr
	}

	permAddr, present := os.LookupEnv("PERMISSION_HOST")
	if !present {
		permAddr = *permissionAddr
	}

	modAddr, present := os.LookupEnv("MODTOOL_HOST")
	if !present {
		modAddr = *modtoolAddr
	}

	confAddr, present := os.LookupEnv("CONFIGURATION_HOST")
	if !present {
		confAddr = *configurationAddr
	}

	achAddr, present := os.LookupEnv("ACHIEVEMENTS_HOST")
	if !present {
		achAddr = *achievementsAddr
	}

	go Net.Listen(netAddr)
	go Sub.Listen(subAddr)
	go Perm.Listen(permAddr)
	go Mod.Listen(modAddr)
	go Cfg.Listen(confAddr)
	go Ach.Listen(achAddr)
	go Data.Listen()
	go Events.Listen()
	StartServer(Events)
}
