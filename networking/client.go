package main

import (
	"net/http"
	"github.com/gorilla/websocket"
	"log"
)

type Client struct {
	manager *Manager
	conn  *websocket.Conn
	send chan []byte
}


var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true; } }

func (c *Client) die() {
	c.manager.die <- c
	c.conn.Close()
}

func (c *Client) read() {
	defer c.die()

	for {
		messageType, message, err := c.conn.ReadMessage()
		if err != nil {
			log.Println("Client.read() error:", err)
			break
		}
		if messageType != websocket.BinaryMessage {
			continue
		}
		parsedMessage := ParseMessage(message)
		fn := Handlers[parsedMessage.header]
		if fn == nil {
			log.Println("Client.read() unrecognized header:", parsedMessage.header)
			continue
		}
		go fn(c, parsedMessage.data)
	}
}

func (c *Client) write() {
	defer c.die()
	for {
		message, ok := <- c.send
		if !ok {
			c.conn.WriteMessage(websocket.CloseMessage, []byte{})
			return
		}

		w, err := c.conn.NextWriter(websocket.BinaryMessage)
		if err != nil {
			return
		}
		log.Printf("Sending: [% x]", message)
		w.Write(message)
		numMessages := len(c.send)
		for i := 0; i < numMessages; i++ {
			w.Write(<-c.send)
		}

		if err := w.Close(); err != nil {
			return
		}
	}
}

func serveWs(manager *Manager, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("serveWs() error during upgrading:", err)
		return
	}
	client := &Client{manager: manager, conn: conn, send: make(chan []byte, 8096)}
	client.manager.register <- client

	go client.write()
	go client.read()
}
