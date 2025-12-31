package main

import "log"

func GuildPartsComposer(parts [][]GuildPart) []byte {
	packet := make([]byte, 0)
	for _, partsList := range parts {
		log.Printf("GuildPartsComposer: [% x]", partsList[0].Packet)
		packet = appendValue(packet, SerializeAll(partsList))
	}
	return compose(2238, packet)
}
