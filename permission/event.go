package main

func (p RankPerk) Serialize() []byte {
	return serializeValues(p.Key, p.Requirement, p.Value)
}

func (p RankPerk) PushBytes(data []byte) []byte {
	data = appendString(data, p.Key)
	data = appendString(data, p.Requirement)
	data = appendBool(data, p.Value)
	return data
}

func (ps Perks) Serialize() []byte {
	data := make([]byte, 0)
	data = appendInt(data, len(ps))
	for _, perk := range ps {
		data = perk.PushBytes(data)
	}
	return data
}

func UserPerksComposer(ps Perks) []byte {
	return compose(2586, ps)
}

func handleUserDataRequest(sso string, _ []byte) {
	userData := Player.GetUserData(sso)
	perks, err := db.GetRankPerks(int(userData.Rank))
	if err != nil {
		return
	}
	Net.Send(sso, UserPerksComposer(perks))
}

func RegisterIncomingHandlers() {
	Incoming.RegisterHandler(357, handleUserDataRequest)
}
