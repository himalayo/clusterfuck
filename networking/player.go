package main

import (
	"log"
	player "github.com/himalayo/clusterfuck/player/api"
)

type AuthSystem struct {
	pending map[string]*Client
}

func newAuth() *AuthSystem {
	return &AuthSystem{pending: make(map[string]*Client)}
}

func handleLoginEvent(c *Client, msg []byte) {
	sso, _ := ReadString(msg)
	log.Printf("handleLoginEvent: %s", sso)
	Auth.pending[sso] = c
	Player.LoginRequest(sso)
}


func (a *AuthSystem) run(p *player.PlayerClient, m *Manager) {
	for {
		select {
		case sso := <- p.Login:
			client := a.pending[sso]
			if client != nil {
				m.login <- &Login{sso: sso, client: client}
			} else {
				log.Printf("AuthSystem.run: client was nil")
			}
		}
	}
}
