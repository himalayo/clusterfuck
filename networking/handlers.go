package main

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
		count:    0,
		handlers: make(map[int16][]handler),
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

func RegisterHandlers() {
	registerAuthHandlers()
}

func registerAuthHandlers() {
	RegisterHandler(2419, handleLoginEvent)
}
