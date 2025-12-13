package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	player "github.com/himalayo/clusterfuck/api/player"
)

var (
	addr       = flag.String("addr", ":2096", "http service address")
	playerAddr = flag.String("player_addr", "localhost:50053", "player service address")
	Man        = newManager()
	Player     = player.NewClient()
	Auth       = newAuth()
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
