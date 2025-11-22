package main

import (
	"encoding/binary"
	"testing"
	"errors"
	"fmt"
)

type preprocessedPacket struct {
	length int
	data []byte
}

func preprocessPacket(data []byte) *preprocessedPacket {
	length := int(binary.BigEndian.Uint32(data[0:4]))
	output := preprocessedPacket{length: length, data: data[4:]}
	return &output
}

func (p *preprocessedPacket) String() string {
	return fmt.Sprintf("%d [% x]", p.length, p.data)
}

type ClientMessage struct {
	header int16
	data []byte
}

func toClientMessage(p *preprocessedPacket) *ClientMessage {
	header := int16(binary.BigEndian.Uint16(p.data[0:2]))
	output := ClientMessage{header: header, data: p.data[2:]}
	return &output
}

func (msg *ClientMessage) String() string {
	return fmt.Sprintf("%d [% x]", msg.header, msg.data)
}

func ParseMessage(unparsedData []byte) *ClientMessage {
	return toClientMessage(preprocessPacket(unparsedData))
}

func ReadShort(data []byte) (int16, []byte) {
	return int16(binary.BigEndian.Uint16(data[0:2])), data[2:]
}

func ReadString(data []byte) (string, []byte) {
	length, remainingData := ReadShort(data)
	return string(remainingData[:length]), remainingData[length:]
}

func ReadInt(data []byte) (int, []byte) {
	return int(int32(binary.BigEndian.Uint32(data[0:4]))), data[4:]
}

func ReadCfhTopic(data []byte) (PartialTopic, []byte) {
	name, d := ReadString(data)
	id, d := ReadInt(d)
	action, d := ReadString(d)
	return PartialTopic{Name: name, Id: id, Action: action}, d
}

func ReadCfhCategory(data []byte) (PartialCategory, []byte) {
	name, d := ReadString(data)
	topicsLen, d := ReadInt(d)
	topics := make([]PartialTopic, topicsLen)
	for i := range topics {
		var topic PartialTopic
		topic, d = ReadCfhTopic(d)
		topics[i] = topic
	}
	return PartialCategory{Name: name, Topics: topics}, d
}

func DeserializeCfhTopicsData(data []byte) []PartialCategory {
	categoriesLen, d := ReadInt(data)
	categories := make([]PartialCategory, categoriesLen)
	for i := range categories {
		var category PartialCategory
		category, d = ReadCfhCategory(d)
		categories[i] = category
	}
	return categories
}

func ConsumeCfhTopicsMessageComposer(rawPacket []byte) ([]PartialCategory, error) {
	preprocessedPacket := preprocessPacket(rawPacket)
	if preprocessedPacket.length != len(preprocessedPacket.data) {
		return nil, errors.New(fmt.Sprintf("Expected length was: %d, data length was: %d", preprocessedPacket.length, len(preprocessedPacket.data)))
	}
	clientMessage := toClientMessage(preprocessedPacket)
	if clientMessage.header != 325 {
		return nil, errors.New(fmt.Sprintf("Expected header: %d, got %d instead", 325, clientMessage.header))
	}
	return DeserializeCfhTopicsData(clientMessage.data), nil
}

var dummyData = []PartialCategory{
		PartialCategory{
			Name: "test1",
			Topics: []PartialTopic{
				PartialTopic{
					Id: 1,
					Name: "topic1",
					Action: "auto_ignore",
				},
				PartialTopic{
					Id: 2,
					Name: "topic2",
					Action: "auto_ignore",
				},
				PartialTopic{
					Id: 3,
					Name: "topic3",
					Action: "mods",
				},
			},
		},
		PartialCategory{
			Name: "test2",
			Topics: []PartialTopic{
				PartialTopic{
					Id: 4,
					Name: "topic4",
					Action: "mods",
				},
				PartialTopic{
					Id: 5,
					Name: "topic5",
					Action: "auto_reply",
				},
			},
		},
	}

func TestCfhTopicsComposerErr(t *testing.T) {
	_, err := ConsumeCfhTopicsMessageComposer(CfhTopicsComposer(dummyData))
	if err != nil {
		t.Errorf("CfhTopicsComposer: Could not deserialize: %v", err)
	}
}

func TestCfhTopicsComposerCategoryNames(t *testing.T) {
	categories, _ := ConsumeCfhTopicsMessageComposer(CfhTopicsComposer(dummyData))
	for i := range categories {
		if categories[i].Name != dummyData[i].Name {
			t.Errorf("CfhTopicsComposer: Category name mismatch on index %d: Expected %s, got %s", i, dummyData[i].Name, categories[i].Name)
		}
	}
}

func TestCfhTopicsComposerCategoryTopicIds(t *testing.T) {
	categories, _ := ConsumeCfhTopicsMessageComposer(CfhTopicsComposer(dummyData))
	for i, category := range categories {
		for j := range category.Topics {
			if category.Topics[j].Id != dummyData[i].Topics[j].Id {
				t.Errorf("CfhTopicsComposer: Topic Id mismatch on ( %d , %d ): Expected Id %d, got Id %d", i, j, category.Topics[j].Id, dummyData[i].Topics[j].Id)
			}
		}
	}
}

func TestCfhTopicsComposerCategoryTopicNames(t *testing.T) {
	categories, _ := ConsumeCfhTopicsMessageComposer(CfhTopicsComposer(dummyData))
	for i, category := range categories {
		for j := range category.Topics {
			if category.Topics[j].Name != dummyData[i].Topics[j].Name {
				t.Errorf("CfhTopicsComposer: Topic name mismatch on ( %d , %d ): Expected %s, got %s", i, j, category.Topics[j].Name, dummyData[i].Topics[j].Name)
			}
		}
	}
}

func TestCfhTopicsComposerCategoryTopicActions(t *testing.T) {
	categories, _ := ConsumeCfhTopicsMessageComposer(CfhTopicsComposer(dummyData))
	for i, category := range categories {
		for j := range category.Topics {
			if category.Topics[j].Action != dummyData[i].Topics[j].Action {
				t.Errorf("CfhTopicsComposer: Topic name mismatch on ( %d , %d ): Expected %s, got %s", i, j, category.Topics[j].Action, dummyData[i].Topics[j].Action)
			}
		}
	}
}
