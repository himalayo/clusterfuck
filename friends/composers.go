package main

import "math"

var (
	infiniteFriendsMessengerInitComposerStart = serializeValues(math.MaxInt32, 1337, math.MaxInt32)
	normalMessengerInitComposerStart          = serializeValues(200, 1337, 500)
)

type MessengerInitComposerData struct {
	InfiniteFriends     bool
	MessengerCategories []*MessengerCategory
}

func (x *MessengerInitComposerData) Serialize() []byte {
	if x.InfiniteFriends {
		return serializeValues(infiniteFriendsMessengerInitComposerStart, SerializeAll(x.MessengerCategories))
	}
	return serializeValues(normalMessengerInitComposerStart, SerializeAll(x.MessengerCategories))
}

func MessengerInitComposer(composerData *MessengerInitComposerData) []byte {
	return compose(1605, composerData)
}
