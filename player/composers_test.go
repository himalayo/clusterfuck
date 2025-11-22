package main

import (
	"testing"
	"bytes"
	"math"
)

func testComposer(t *testing.T, name string, current []byte, expected []byte) {
	if !bytes.Equal(current, expected) {
		t.Errorf("%s: [% x] [% x]", name, current, expected)
	}
}

func TestLoginOK(t *testing.T) {
	testComposer(t, "LoginOK", LoginOKComposer(), emptyPacket(2491))
}

func TestNoobStatus(t *testing.T) {
	testComposer(t,"UserNoobStatus", UserNoobStatusComposer(1), addSize(appendInt(shortToBytes(3738), 1)))
}

func TestUserPermissions(t *testing.T) {
	testComposer(t,"UserPermissions", UserPermissionsComposer(2, 7, true), addSize(appendBool(appendInt(appendInt(shortToBytes(411), 2), 7), true)))
}

func TestAvailabilityStatus(t *testing.T) {
	testComposer(t, "AvailabilityStatus", AvailabilityStatusComposer(true, false, true), addSize(appendBool(appendBool(appendBool(shortToBytes(2033),true),false),true)))
}

func TestEnableNotifications(t *testing.T) {
	testComposer(t, "EnableNotifications", EnableNotificationsComposer(true), addSize(appendBool(shortToBytes(3284),true)))
}

func TestAchievementScore(t *testing.T) {
	testComposer(t, "AchievementScore", AchievementScoreComposer(2), addSize(appendInt(shortToBytes(1968), 2)))
}

func TestMysteryBox(t *testing.T) {
	testComposer(t, "MysteryBox", MysteryBoxComposer(), addSize(appendString(appendString(shortToBytes(2833),""),"")))
}

func TestBuildersClubExpired(t *testing.T) {
	testComposer(t, "BuildersClubExpired", BuildersClubExpiredComposer(), addSize(appendInt(appendInt(appendInt(appendInt(appendInt(shortToBytes(1452),0),math.MaxInt32),100),0),math.MaxInt32)))
}
