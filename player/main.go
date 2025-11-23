package main

import (
	"flag"

	achievements "github.com/himalayo/clusterfuck/api/achievements"
	configuration "github.com/himalayo/clusterfuck/api/configuration"
	modtool "github.com/himalayo/clusterfuck/api/modtool"
	networking "github.com/himalayo/clusterfuck/api/networking"
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
	Data              = NewDatabase(ConfigDatabaseFromEnv())
	Events            = NewEventListener(Data, Net, Sub, Perm, Mod, Cfg, Ach)
)

func main() {
	flag.Parse()
	go Net.Listen(*networkingAddr)
	go Sub.Listen(*subscriptionAddr)
	go Perm.Listen(*permissionAddr)
	go Mod.Listen(*modtoolAddr)
	go Cfg.Listen(*configurationAddr)
	go Ach.Listen(*achievementsAddr)
	go Data.Listen()
	go Events.Listen()
	StartServer(Events)
}
