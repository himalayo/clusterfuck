package main

import "time"

type HandlerManager struct {
	count             int
	handlers          map[int16][]handler
	handlerIdtoHeader map[int]int16
}

type handler struct {
	id       int
	function func(*Client, []byte, []byte)
}

func NewHandlerManager() *HandlerManager {
	return &HandlerManager{
		count:             0,
		handlers:          make(map[int16][]handler),
		handlerIdtoHeader: make(map[int]int16),
	}
}

var Handlers = NewHandlerManager()

func RegisterHandler(header int16, h func(*Client, []byte, []byte)) int {
	Handlers.handlers[header] = append(Handlers.handlers[header], handler{
		id:       Handlers.count,
		function: h,
	})
	Handlers.handlerIdtoHeader[Handlers.count] = header
	Handlers.count++
	return Handlers.count - 1
}

func RemoveHandler(id int) {
	header := Handlers.handlerIdtoHeader[id]
	h := Handlers.handlers[header]
	for i := range h {
		if h[i].id == id {
			Handlers.handlers[header] = append(Handlers.handlers[header][:i], Handlers.handlers[header][i+1:]...)
		}
	}
}

func PongComposer(id int) []byte {
	return compose(10, id)
}

func handlePingEvent(c *Client, msg []byte, _ []byte) {
	id, _ := ReadInt(msg)
	c.send <- PongComposer(id)
}

func handlePongEvent(c *Client, _ []byte, _ []byte) {
	c.last_pong = time.Now()
}

func RegisterHandlers() {
	registerAuthHandlers()
	registerPingPongHandlers()
}

func registerAuthHandlers() {
	RegisterHandler(2419, handleLoginEvent)
}

func registerPingPongHandlers() {
	RegisterHandler(295, handlePingEvent)
	RegisterHandler(2596, handlePongEvent)
}
