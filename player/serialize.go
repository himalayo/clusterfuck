package main


type Serializeable interface {
	Serialize() []byte
}

func SerializeAll[T Serializeable](slice []T) []byte {
	packet := integerToBytes(len(slice))
	for _, value := range slice {
		packet = appendValue(packet, value.Serialize())
	}
	return packet
}
