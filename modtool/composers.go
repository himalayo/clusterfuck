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

func (obj PartialTopic) Serialize() []byte {
	return appendValue(appendInt(appendValue([]byte{}, obj.Name), obj.Id), obj.Action)
}

func (obj PartialCategory) Serialize() []byte {
	return appendValue(appendValue([]byte{},obj.Name), SerializeAll(obj.Topics))
}

func CfhTopicsComposer(categories []PartialCategory) []byte {
	return compose(325, SerializeAll(categories))
}
