package main

var Handlers = make(map[int16][]func(*Client, []byte))

func registerHandler(header int16, handler func(*Client, []byte)) {
	Handlers[header] = append(Handlers[header], handler)
}

func RegisterHandlers() {
	registerAuthHandlers()
}

func registerAuthHandlers() {
	registerHandler(2419, handleLoginEvent)
}
