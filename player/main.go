package main

import (
	"flag"

	networking "github.com/himalayo/clusterfuck/networking/api"
	subscription "github.com/himalayo/clusterfuck/subscription/api"
	permission "github.com/himalayo/clusterfuck/permission/api"
	modtool "github.com/himalayo/clusterfuck/modtool/api"
	configuration "github.com/himalayo/clusterfuck/configuration/api"
	achievements "github.com/himalayo/clusterfuck/achievements/api"
	_ "github.com/joho/godotenv/autoload"
)

var (
	networkingAddr = flag.String("net_addr", "localhost:50051", "networking gRPC API address")
	subscriptionAddr = flag.String("sub_addr", "localhost:50052", "subscription gRPC API address")
	permissionAddr = flag.String("per_addr", "localhost:50054", "permission gRPC API address")
	modtoolAddr = flag.String("mod_addr", "localhost:50056", "modtool gRPC API address")
	configurationAddr = flag.String("cfg_addr", "localhost:50057", "configuration gRPC API address")
	achievementsAddr = flag.String("ach_addr", "localhost:50058", "achievements gRPC API address")
	Net = networking.NewClient()
	Sub = subscription.NewClient()
	Perm = permission.NewClient()
	Mod = modtool.NewClient()
	Cfg = configuration.NewClient()
	Ach = achievements.NewClient()
	Data = NewDatabase(ConfigDatabaseFromEnv())
	Events = NewEventListener(Data, Net, Sub, Perm, Mod, Cfg, Ach)
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
