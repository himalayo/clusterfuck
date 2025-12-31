package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
)

type Database struct {
	db  *sql.DB
	rdb *redis.Client
}

type DatabaseConfig struct {
	mysql_conf *mysql.Config
	redis_conf *redis.Options
}

func ConfigDatabaseFromEnv() *DatabaseConfig {
	cfg := mysql.NewConfig()
	cfg.User = os.Getenv("DB_USER")
	cfg.Passwd = os.Getenv("DB_PASS")
	cfg.Net = "tcp"
	cfg.Addr = os.Getenv("DB_ADDR")
	cfg.DBName = os.Getenv("DB_NAME")

	return &DatabaseConfig{
		mysql_conf: cfg,
		redis_conf: &redis.Options{
			Addr:     os.Getenv("CATALOG_REDIS_ADDR"),
			Password: os.Getenv("CATALOG_REDIS_PASSWORD"),
			DB:       0,
		},
	}
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

func NewDatabase(cfg *DatabaseConfig) *Database {
	db, err := sql.Open("mysql", cfg.mysql_conf.FormatDSN())
	if err != nil {
		log.Fatal(err)
	}
	rdb := redis.NewClient(cfg.redis_conf)

	return &Database{
		db:  db,
		rdb: rdb,
	}
}

type CatalogItem struct {
	Id           int    `redis:"id"`
	PageId       int    `redis:"page_id"`
	ItemIds      string `redis:"item_ids"`
	Name         string `redis:"catalog_name"`
	Credits      int    `redis:"credits"`
	Points       int    `redis:"points"`
	PointsType   int    `redis:"points_type"`
	Amount       int    `redis:"amount"`
	LimitedStack int    `redis:"limited_stack"`
	LimitedSells int    `redis:"limited_sells"`
	Extradata    string `redis:"extradata"`
	ClubOnly     bool   `redis:"club_only"`
	HaveOffer    bool   `redis:"have_offer"`
	OfferId      int    `redis:"offer_id"`
	OrderNumber  int    `redis:"order_number"`
}

func (data *Database) loadCatalogItemsFromDB(ctx context.Context) ([]CatalogItem, error) {
	items := make([]CatalogItem, 0)
	rows, err := data.db.QueryContext(ctx, "SELECT `id`, `page_id`, `item_ids`, `catalog_name`, `cost_credits`, `cost_points`, `points_type`, `amount`, `limited_stack`, `limited_sells`, `extradata`, `club_only`, `have_offer`, `offer_id`, `order_number` FROM catalog_items")
	if err != nil {
		return nil, err
	}
	pipe := data.rdb.Pipeline()
	for rows.Next() {
		var item CatalogItem
		if err := rows.Scan(&item.Id, &item.PageId, &item.ItemIds, &item.Name, &item.Credits, &item.Points, &item.PointsType, &item.Amount, &item.LimitedStack, &item.LimitedSells, &item.Extradata, &item.ClubOnly, &item.HaveOffer, &item.OfferId, &item.OrderNumber); err != nil {
			log.Printf("loadCatalogItemsFromDB(): Got error while scanning: %v", err)
			continue
		}
		items = append(items, item)
		pipe.HSet(ctx, fmt.Sprintf("catalog_items:%d", item.Id), item)
		if strings.Contains(item.Name, "HABBO_CLUB_") {
			pipe.SAdd(ctx, "club_items", item.Id)
			continue
		}

		pipe.SAdd(ctx, fmt.Sprintf("catalog_items_by_page_id:%d", item.PageId), item.Id)
		if item.OfferId != -1 {
			pipe.SAdd(ctx, fmt.Sprintf("catalog_offers_by_page_id:%d", item.PageId), item.OfferId)
			pipe.SAdd(ctx, fmt.Sprintf("catalog_offers_id_to_item_id:%d", item.OfferId), item.Id)
		}
	}
	cmds, err := pipe.Exec(ctx)
	if err != nil {
		log.Printf("loadCatalogItemsFromDB(): Got error while executing pipeline: %v", err)
	} else {
		for _, c := range cmds {
			if err := c.Err(); err != nil {
				log.Printf("loadCatalogItemsFromDB(): Got error while caching: %v", err)
			}
		}
	}

	return items, nil
}

type CatalogPage struct {
	Id           int    `redis:"id"`
	ParentId     int    `redis:"parent_id"`
	Rank         int    `redis:"rank"`
	Caption      string `redis:"caption"`
	PageName     string `redis:"page_name"`
	IconColor    int    `redis:"icon_color"`
	IconImage    int    `redis:"icon_image"`
	OrderNum     int    `redis:"order_num"`
	Visible      bool   `redis:"visible"`
	Enabled      bool   `redis:"enabled"`
	ClubOnly     bool   `redis:"club_only"`
	Layout       string `redis:"layout"`
	HeaderImage  string `redis:"header_image"`
	TeaserImage  string `redis:"teaser_imager"`
	SpecialImage string `redis:"special_image"`
	TextOne      string `redis:"text_one"`
	TextTwo      string `redis:"text_two"`
	TextDetails  string `redis:"text_details"`
	TextTeaser   string `redis:"text_teaser"`
}

func (data *Database) LoadCatalogPagesFromDB(ctx context.Context) (map[int]CatalogPage, error) {
	pages := make(map[int]CatalogPage)
	pages[-1] = CatalogPage{
		Id:       -1,
		Caption:  "root",
		PageName: "root",
		OrderNum: -10,
		Visible:  true,
		Enabled:  true,
	}

	rows, err := data.db.QueryContext(ctx, "SELECT `id`, `parent_id`, `min_rank`, `min_rank`, `caption`, `caption_save`, `icon_color`, `icon_image`, `order_num`, `visible`, `enabled`, `club_only`, `page_layout`, `page_headline`, `page_teaser`, `page_special`, `page_text1`, `page_text2`, `page_text_details`, `page_text_teaser` FROM catalog_pages ORDER BY parent_id, id")
	if err != nil {
		return pages, err
	}
	pipe := data.rdb.Pipeline()

	for rows.Next() {
		var page CatalogPage
		if err := rows.Scan(&page.Id, &page.ParentId, &page.Rank, &page.Caption, &page.PageName, &page.IconColor, &page.IconImage, &page.OrderNum, &page.Visible, &page.Enabled, &page.ClubOnly, &page.Layout, &page.HeaderImage, &page.TeaserImage, &page.SpecialImage, &page.TextOne, &page.TextTwo, &page.TextDetails, &page.TextTeaser); err != nil {
			log.Printf("LoadCatalogPagesdFromDB(): Got error while scanning result: %v", err)
			continue
		}
		pages[page.Id] = page
		pipe.HSet(ctx, fmt.Sprintf("catalog_pages:%d", page.Id), page)
		pipe.SAdd(ctx, fmt.Sprintf("catalog_pages_by_parent:%d", page.ParentId), page.Id)
		pipe.SAdd(ctx, fmt.Sprintf("catalog_pages_by_page_name:%s", page.PageName), page.Id)
		if page.Layout != "" {
			pipe.SAdd(ctx, fmt.Sprintf("catalog_pages_by_layout:%s", page.Layout), page.Id)
		}
	}

	cmds, err := pipe.Exec(ctx)
	if err != nil {
		log.Printf("LoadCatalogPagesFromDB(): Got error when executing caching pipeline: %v", err)
	}

	for _, c := range cmds {
		if err := c.Err(); err != nil {
			log.Printf("LoadCatalogPagesFromDB(): Got error when caching: %v", err)
		}
	}

	return pages, nil
}

func (data *Database) loadCatalogPageByIdFromDB(ctx context.Context, pageId int) (*CatalogPage, error) {
	var page CatalogPage
	row := data.db.QueryRowContext(ctx, "SELECT `id`, `parent_id`, `min_rank`, `min_rank`, `caption`, `caption_save`, `icon_color`, `icon_image`, `order_num`, `visible`, `enabled`, `club_only`, `page_layout`, `page_headline`, `page_teaser`, `page_special`, `page_text1`, `page_text2`, `page_text_details`, `page_text_teaser` FROM catalog_pages WHERE `id` = ? ", pageId)
	if err := row.Scan(&page.Id, &page.ParentId, &page.Rank, &page.Caption, &page.PageName, &page.IconColor, &page.IconImage, &page.OrderNum, &page.Visible, &page.Enabled, &page.ClubOnly, &page.Layout, &page.HeaderImage, &page.TeaserImage, &page.SpecialImage, &page.TextOne, &page.TextTwo, &page.TextDetails, &page.TextTeaser); err != nil {
		log.Printf("loadCatalogPageByIdFromDB(): Got error while scanning result: %v", err)
		return nil, err
	}
	if err := data.rdb.HSet(ctx, fmt.Sprintf("catalog_pages:%d", page.Id), page).Err(); err != nil {
		log.Printf("loadCatalogPageByIdFromDB(): Got error while caching result: %v", err)
	}
	if err := data.rdb.SAdd(ctx, fmt.Sprintf("catalog_pages_by_parent:%d", page.ParentId), page.Id).Err(); err != nil {
		log.Printf("loadCatalogPageByIdFromDB(): Got error while indexing by parent: %v", err)
	}
	if err := data.rdb.SAdd(ctx, fmt.Sprintf("catalog_pages_by_page_name:%s", page.PageName), page.Id).Err(); err != nil {
		log.Printf("loadCatalogPageByIdFromDB(): Got error while indexing by page name: %v", err)
	}
	if page.Layout != "" {
		if err := data.rdb.SAdd(ctx, fmt.Sprintf("catalog_pages_by_layout:%s", page.Layout), page.Id).Err(); err != nil {
			log.Printf("loadCatalogPageByIdFromDB(): Got error while indexing by page layout: %v", err)
		}
	}
	return &page, nil
}

func (data *Database) GetCatalogPageByPageId(ctx context.Context, pageId int) (*CatalogPage, error) {
	exists, err := data.rdb.Exists(ctx, fmt.Sprintf("catalog_pages:%d", pageId)).Result()
	if err != nil {
		if exists != 0 {
			var page CatalogPage
			err := data.rdb.HGetAll(ctx, fmt.Sprintf("catalog_pages:%d", pageId)).Scan(&page)
			if err != nil {
				return nil, err
			}
			return &page, nil
		}
	}
	return data.loadCatalogPageByIdFromDB(ctx, pageId)
}

func (data *Database) loadCatalogPageByNameFromDB(ctx context.Context, pageName string) (*CatalogPage, error) {
	var page CatalogPage
	row := data.db.QueryRowContext(ctx, "SELECT `id`, `parent_id`, `min_rank`, `min_rank`, `caption`, `caption_save`, `icon_color`, `icon_image`, `order_num`, `visible`, `enabled`, `club_only`, `page_layout`, `page_headline`, `page_teaser`, `page_special`, `page_text1`, `page_text2`, `page_text_details`, `page_text_teaser` FROM catalog_pages WHERE `caption_save` = ? LIMIT 1", pageName)
	if err := row.Scan(&page.Id, &page.ParentId, &page.Rank, &page.Caption, &page.PageName, &page.IconColor, &page.IconImage, &page.OrderNum, &page.Visible, &page.Enabled, &page.ClubOnly, &page.Layout, &page.HeaderImage, &page.TeaserImage, &page.SpecialImage, &page.TextOne, &page.TextTwo, &page.TextDetails, &page.TextTeaser); err != nil {
		log.Printf("loadCatalogPageByIdFromDB(): Got error while scanning result: %v", err)
		return nil, err
	}
	go data.LoadCatalogPagesFromDB(ctx)
	return &page, nil
}

func (data *Database) GetCatalogPageByName(ctx context.Context, pageName string) (*CatalogPage, error) {
	page_ids, err := data.rdb.SMembers(ctx, fmt.Sprintf("catalog_pages_by_page_name:%s", pageName)).Result()
	if err != nil || len(page_ids) == 0 {
		return data.loadCatalogPageByNameFromDB(ctx, pageName)
	}
	var page CatalogPage
	err = data.rdb.HGetAll(ctx, fmt.Sprintf("catalog_pages:%s", page_ids[0])).Scan(&page)
	return &page, err
}

func (data *Database) loadCatalogPageByLayoutFromDB(ctx context.Context, layout string) (*CatalogPage, error) {
	var page CatalogPage
	row := data.db.QueryRowContext(ctx, "SELECT `id`, `parent_id`, `min_rank`, `min_rank`, `caption`, `caption_save`, `icon_color`, `icon_image`, `order_num`, `visible`, `enabled`, `club_only`, `page_layout`, `page_headline`, `page_teaser`, `page_special`, `page_text1`, `page_text2`, `page_text_details`, `page_text_teaser` FROM catalog_pages WHERE `page_layout` = ? LIMIT 1", layout)
	if err := row.Scan(&page.Id, &page.ParentId, &page.Rank, &page.Caption, &page.PageName, &page.IconColor, &page.IconImage, &page.OrderNum, &page.Visible, &page.Enabled, &page.ClubOnly, &page.Layout, &page.HeaderImage, &page.TeaserImage, &page.SpecialImage, &page.TextOne, &page.TextTwo, &page.TextDetails, &page.TextTeaser); err != nil {
		log.Printf("loadCatalogPageByIdFromDB(): Got error while scanning result: %v", err)
		return nil, err
	}
	go data.LoadCatalogPagesFromDB(ctx)
	return &page, nil
}

func (data *Database) GetCatalogPageByLayout(ctx context.Context, layout string) (*CatalogPage, error) {
	page_ids, err := data.rdb.SMembers(ctx, fmt.Sprintf("catalog_pages_by_layout:%s", layout)).Result()
	if err != nil || len(page_ids) == 0 {
		return data.loadCatalogPageByLayoutFromDB(ctx, layout)
	}
	var page CatalogPage
	err = data.rdb.HGetAll(ctx, fmt.Sprintf("catalog_pages:%s", page_ids[0])).Scan(&page)
	return &page, err
}
