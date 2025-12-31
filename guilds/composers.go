package main

func GuildPartsComposer(parts [][]GuildPart) []byte {
	packet := make([]byte, 0)
	for _, partsList := range parts {
		packet = appendValue(packet, SerializeAll(partsList))
	}
	return compose(2238, packet)
}
