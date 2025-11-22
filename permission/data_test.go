package main

import (
	"testing"
	//"fmt"
	_ "github.com/joho/godotenv/autoload"
)

func TestGetRank(t *testing.T) {
	data := NewDatabase(ConfigDatabaseFromEnv())
	rank, err := data.GetRank(2)
	if err != nil {
		t.Errorf("GetRank(2) got an error: %v", err)
	}
	if rank.Id != 2 {
		t.Errorf("GetRank(2): got id: %d", rank.Id)
	}
}


func TestGetRankByName(t *testing.T) {
	data := NewDatabase(ConfigDatabaseFromEnv())
	rank, err := data.GetRankByName("Member")
	if err != nil {
		t.Errorf("GetRankByName('Member') got an error: %v", err)
	}
	if rank.Name != "Member" {
		t.Errorf("GetRankByName('Member'): got name: %s", rank.Name)
	}
}

func TestGetAllRanks(t *testing.T) {
	data := NewDatabase(ConfigDatabaseFromEnv())
	ranks, err := data.GetAllRanks()
	if err != nil {
		t.Errorf("GetAllRanks() got an error: %v", err)
	}
	if len(ranks) != 8 {
		t.Errorf("GetAllRanks() size was: %d", len(ranks))
	}
}

func TestRankExistsCheck(t *testing.T) {
	data := NewDatabase(ConfigDatabaseFromEnv())
	rank, err := data.GetRank(9999)
	if err != nil {
		t.Errorf("GetRank(9999) got an error: %v", err)
	}
	if rank != nil {
		t.Errorf("GetRank(9999) was not null: %s", rank)
	}
}

func TestRankLevel(t *testing.T) {
	data := NewDatabase(ConfigDatabaseFromEnv())
	level, err := data.GetRankLevel(2)
	if err != nil {
		t.Errorf("GetRankLevel(2) got an error: %v", err)
	}
	if level != 2 {
		t.Errorf("GetRankLevel(2) got an unexpected result: %d", level)
	}
}

func TestGetPermission(t *testing.T) {
	data := NewDatabase(ConfigDatabaseFromEnv())
	permission, err := data.GetPermission(1, "acc_ambassador")
	if err != nil {
		t.Errorf("GetPermission(1, 'acc_ambassador') got an error: %v", err)
	}
	if permission != 0 {
		t.Errorf("GetPermission(1, 'acc_ambassador') got an unexpected value: %d", permission)
	}
}

func TestUnexistentPermission(t *testing.T) {
	data := NewDatabase(ConfigDatabaseFromEnv())
	perm, err := data.GetPermission(1, "test")
	if err == nil {
		t.Errorf("GetPermission(1, 'test') did not fail and returned: %d", perm)
	}
}

func TestSqlInjection(t *testing.T) {
	data := NewDatabase(ConfigDatabaseFromEnv())
	perm, err := data.GetPermission(1, "id FROM users WHERE id=1")
	if err == nil {
		t.Errorf("GetPermission(1, 'id FROM users WHERE id=1') did not fail and returned: %d", perm)
	}
}
