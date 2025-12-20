package main

import (
	"context"
	"math"
	"time"
)

func (sub SubscriptionData) GetRemaining() int {
	return int(int64(sub.TimestampStart+sub.Duration) - time.Now().Unix())
}

func calculatePastTime(subscriptions []SubscriptionData) int {
	pastTime := 0
	for _, sub := range subscriptions {
		pastTime += sub.Duration - sub.GetRemaining()
	}
	return pastTime
}

func isValidSubscription(sub SubscriptionData, subType string) bool {
	return sub.Type == subType && sub.Active && sub.GetRemaining() > 0
}

func getCurrentSubscription(subscriptions []SubscriptionData, subType string) *SubscriptionData {
	for _, sub := range subscriptions {
		if isValidSubscription(sub, subType) {
			return &sub
		}
	}
	return nil
}

type SubscriptionTime struct {
	TimeRemaining int
	Days          int
	Minutes       int
}

func calculateDaysAndMinutes(timeRemaining int) (int, int) {
	days := int(math.Floor(float64(timeRemaining) / 86400.0))
	minutes := int(math.Ceil(float64(timeRemaining) / 60.0))

	if days < 1 && minutes > 0 {
		days = 1
	}
	return days, minutes
}

func getSubscriptionTime(sub *SubscriptionData) *SubscriptionTime {
	if sub != nil {
		timeRemaining := sub.GetRemaining()
		days, minutes := calculateDaysAndMinutes(timeRemaining)
		return &SubscriptionTime{TimeRemaining: timeRemaining, Days: days, Minutes: minutes}
	}
	return &SubscriptionTime{TimeRemaining: 0, Days: 0, Minutes: 0}
}

type UserClubData struct {
	Type                     string
	TimeToExpire             *SubscriptionTime
	MemberPeriods            int
	PeriodsSubscribedAhead   int
	ResponseType             int
	PastTime                 int
	MinutesSinceLastModified int
	HasEverBeenMember        bool
	IsVip                    bool
	PastClubDays             int
	PastVipDays              int
}

func NewUserClubData(ctx context.Context, subscriptions []SubscriptionData, responseType int) *UserClubData {
	pastTime := calculatePastTime(subscriptions)
	currentSubscription := getCurrentSubscription(subscriptions, "HABBO_CLUB")
	subscriptionTime := getSubscriptionTime(currentSubscription)
	hasEverBeenMember := pastTime > 0
	isVip := true
	minutesSinceLastModified := int(float32(int(time.Now().Unix())-currentSubscription.LastModified(ctx)) / 60.0)
	currentSubscription.SetLastModified(ctx, int(time.Now().Unix()))
	pastClubDays := 0
	pastVipDays := int(float32(pastTime) / 86400.0)
	memberPeriods := 0
	periodsSubscribedAhead := 0
	t := ""
	if currentSubscription != nil {
		t = currentSubscription.Type
	}
	return &UserClubData{
		Type:                     t,
		PastTime:                 pastTime,
		ResponseType:             responseType,
		TimeToExpire:             subscriptionTime,
		MinutesSinceLastModified: minutesSinceLastModified,
		HasEverBeenMember:        hasEverBeenMember,
		IsVip:                    isVip,
		PastClubDays:             pastClubDays,
		PastVipDays:              pastVipDays,
		MemberPeriods:            memberPeriods,
		PeriodsSubscribedAhead:   periodsSubscribedAhead,
	}
}

func (club *UserClubData) Serialize() []byte {
	return serializeValues(club.Type,
		club.TimeToExpire.Days,
		club.MemberPeriods,
		club.PeriodsSubscribedAhead,
		club.ResponseType,
		club.HasEverBeenMember,
		club.IsVip,
		club.PastClubDays,
		club.PastVipDays,
		club.TimeToExpire.Minutes,
		club.MinutesSinceLastModified)
}
func (club *UserClubData) SerializeWithType(t string) []byte {
	return serializeValues(t,
		club.TimeToExpire.Days,
		club.MemberPeriods,
		club.PeriodsSubscribedAhead,
		club.ResponseType,
		club.HasEverBeenMember,
		club.IsVip,
		club.PastClubDays,
		club.PastVipDays,
		club.TimeToExpire.Minutes,
		club.MinutesSinceLastModified)
}

func UserClubComposerWithType(ctx context.Context, subscriptions []SubscriptionData, responseType int, t string) []byte {
	return compose(954, NewUserClubData(ctx, subscriptions, responseType).SerializeWithType(t))
}
func UserClubComposer(ctx context.Context, subscriptions []SubscriptionData, responseType int) []byte {
	return compose(954, NewUserClubData(ctx, subscriptions, responseType).Serialize())
}
