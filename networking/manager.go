package main

import (
	"log"
)

type Login struct {
	sso    string
	client *Client
}

type Manager struct {
	clients  map[*Client]bool
	register chan *Client
	die      chan *Client
	sessions map[string]*Client
	login    chan *Login
}

func newManager() *Manager {
	return &Manager{
		clients:  make(map[*Client]bool),
		register: make(chan *Client),
		die:      make(chan *Client),
		sessions: make(map[string]*Client),
		login:    make(chan *Login),
	}
}

func (m *Manager) run() {
	for {
		select {
		case client := <-m.register:
			m.clients[client] = true
		case client := <-m.die:
			if _, ok := m.clients[client]; ok {
				delete(m.clients, client)
				close(client.send)
			}
		case loginRequest := <-m.login:
			m.sessions[loginRequest.sso] = loginRequest.client
			loginRequest.client.sso = loginRequest.sso
			log.Printf("Logged in user with sso: %s", loginRequest.sso)
		}
	}
}
