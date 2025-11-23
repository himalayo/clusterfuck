package main

import (
	"database/sql"
	"log"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/go-sql-driver/mysql"

	pb "github.com/himalayo/clusterfuck/api/modtool/proto"
)

const partialCfhTopicsQuery = `SELECT
support_cfh_topics.id,
support_cfh_topics.category_id,
support_cfh_topics.name_internal,
support_cfh_topics.action,
support_cfh_categories.name_internal AS category_name_internal
FROM support_cfh_topics
LEFT JOIN support_cfh_categories ON support_cfh_categories.id = support_cfh_topics.category_id`

type Database struct {
	db *sql.DB
}

func ConfigDatabaseFromEnv() *mysql.Config {
	cfg := mysql.NewConfig()
	cfg.User = os.Getenv("DB_USER")
	cfg.Passwd = os.Getenv("DB_PASS")
	cfg.Net = "tcp"
	cfg.Addr = os.Getenv("DB_ADDR")
	cfg.DBName = os.Getenv("DB_NAME")

	return cfg
}

func ConfigDatabase(user string, password string, address string, name string) *mysql.Config {
	cfg := mysql.NewConfig()
	cfg.User = user
	cfg.Passwd = password
	cfg.Net = "tcp"
	cfg.Addr = address
	cfg.DBName = name

	return cfg
}

func NewDatabase(cfg *mysql.Config) *Database {
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		log.Fatal(err)
	}

	return &Database{db: db}
}

func parseCfhActionType(actionType string) pb.CfhActionType {
	return pb.CfhActionType(pb.CfhActionType_value[strings.ToUpper(actionType)])
}

func parseBool(value string) bool {
	return value == "1"
}

func (data *Database) GetIssuePreset(id int) (*pb.IssuePreset, error) {
	if id == 0 {
		return &pb.IssuePreset{Id: 0}, nil
	}

	var preset pb.IssuePreset
	row := data.db.QueryRow("SELECT id, name, message, reminder, ban_for, mute_for FROM support_issue_presets WHERE id = ?", id)
	if err := row.Scan(&preset.Id, &preset.Name, &preset.Message, &preset.Reminder, &preset.BanLength, &preset.MuteLength); err != nil {
		return nil, err
	}
	return &preset, nil
}

func (data *Database) GetCfhTopic(id int) (*pb.CfhTopic, error) {
	topic := pb.CfhTopic{}
	var unparsedActionType string
	var unparsedIgnoreTarget string
	var defaultSanctionId int
	row := data.db.QueryRow("SELECT id, name_internal, action, ignore_target, auto_reply, default_sanction, name_external FROM support_cfh_topics WHERE id = ?", id)
	if err := row.Scan(&topic.Id, &topic.Name, &unparsedActionType, &unparsedIgnoreTarget, &topic.Reply, &defaultSanctionId, &topic.NameExternal); err != nil {
		return nil, err
	}

	topic.Action = parseCfhActionType(unparsedActionType)
	topic.IgnoreTarget = parseBool(unparsedIgnoreTarget)
	sanction, err := data.GetIssuePreset(defaultSanctionId)
	if err != nil {
		return nil, err
	}
	topic.DefaultSanction = sanction
	return &topic, nil
}

type PartialTopic struct {
	Name   string
	Id     int
	Action string
}

type PartialCategory struct {
	Name   string
	Topics []PartialTopic
}

func (data *Database) GetPartialCategories() ([]PartialCategory, error) {
	categories := make(map[int]PartialCategory)
	rows, err := data.db.Query(partialCfhTopicsQuery)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var topic PartialTopic
		var categoryId int
		var categoryName string
		if err := rows.Scan(&topic.Id, &categoryId, &topic.Name, &topic.Action, &categoryName); err != nil {
			return nil, err
		}
		category, ok := categories[categoryId]
		if !ok {
			categories[categoryId] = PartialCategory{Name: categoryName, Topics: []PartialTopic{}}
		}
		category = categories[categoryId]
		categories[categoryId] = PartialCategory{Name: categoryName, Topics: append(category.Topics, topic)}
	}

	result := make([]PartialCategory, 0)
	for key := range slices.Sorted(maps.Keys(categories)) {
		result = append(result, categories[key])
	}
	return result, nil
}
