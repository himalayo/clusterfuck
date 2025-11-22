package main

import (
	"testing"
	_ "github.com/joho/godotenv/autoload"
)

func TestActionParsing(t *testing.T) {
	result := parseCfhActionType("mods")
	if result != 0 {
		t.Errorf("ActionParsing: result was: %d", result)
	}
}

func TestDefaultPreset(t *testing.T) {
	data := NewDatabase(ConfigDatabaseFromEnv())
	result, err := data.GetIssuePreset(0)
	if err != nil {
		t.Errorf("DefaultPreset: %v", err)
	}
	if result.Id != 0 {
		t.Errorf("DefaultPreset: Id was not 0")
	}
}

func TestGetIssuePresetErr(t *testing.T) {
	data := NewDatabase(ConfigDatabaseFromEnv())
	_, err := data.GetIssuePreset(1)
	if err != nil {
		t.Errorf("GetIssuePreset: %v", err)
	}
}

func TestGetIssuePresetNil(t *testing.T) {
	data := NewDatabase(ConfigDatabaseFromEnv())
	result, _ := data.GetIssuePreset(1)
	if result == nil {
		t.Errorf("GetIssuePreset: Returned nil")
	}
}

func TestGetIssuePresetId(t *testing.T) {
	data := NewDatabase(ConfigDatabaseFromEnv())
	result, _ := data.GetIssuePreset(1)
	if result.Id != 1 {
		t.Errorf("GetIssuePreset: Id was not 1: %d", result.Id)
	}
}

func TestGetIssuePresetName(t *testing.T) {
	data := NewDatabase(ConfigDatabaseFromEnv())
	result, _ := data.GetIssuePreset(1)
	if result.Name != "1 hour mute" {
		t.Errorf("GetIssuePreset: Name was not 1 hour mute: %s", result.Name)
	}
}

func TestGetCfhTopic(t *testing.T) {
	data := NewDatabase(ConfigDatabaseFromEnv())
	result, err := data.GetCfhTopic(1)
	if err != nil {
		t.Errorf("GetCfhTopic: %v", err)
	}
	if result.Id != 1 {
		t.Errorf("GetCfhTopic: Id was not 1: %d", result.Id)
	}
}

func TestGetPartialCategoriesErr(t *testing.T) {
	data := NewDatabase(ConfigDatabaseFromEnv())
	_, err := data.GetPartialCategories()
	if err != nil {
		t.Errorf("PartialCategories: Returned error: %v", err)
	}
}
