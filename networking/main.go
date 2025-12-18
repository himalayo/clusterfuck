package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	player "github.com/himalayo/clusterfuck/api/player"
	"github.com/redis/go-redis/v9"
)

var (
	addr       = flag.String("addr", ":2096", "http service address")
	playerAddr = flag.String("player_addr", "localhost:50053", "player service address")
	Man        = newManager()
	Player, _  = player.NewClient("networking-service", &redis.Options{
		Addr:     os.Getenv("PLAYER_EVENTS_REDIS_ADDR"),
		Password: os.Getenv("PLAYER_EVENTS_REDIS_PASSWORD"),
		DB:       0,
	})
	Auth = newAuth()
)

func main() {
	flag.Parse()
	log.SetFlags(0)

	plAddr, present := os.LookupEnv("PLAYER_HOST")
	if !present {
		plAddr = *playerAddr
	}

	go Man.run()
	go Player.Listen(plAddr)
	go Auth.run(Player, Man)
	RegisterHandlers()
	go StartRpc()
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		serveWs(Man, w, r)
	})
	log.Fatal(http.ListenAndServe(*addr, nil))
}

func init() {
	initResolving()
}
