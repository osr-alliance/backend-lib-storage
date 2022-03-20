package store

import (
	"context"

	"github.com/go-redis/redis/v8"
	"github.com/jmoiron/sqlx"
	storage "github.com/osr-alliance/backend-lib-storage"
)

type Store interface {
	SetLead(ctx context.Context, lead *Leads) error
	GetLeadByID(ctx context.Context, id int32) (*Leads, error)
	GetLeadsByUserID(ctx context.Context, id int32) ([]Leads, error)
	UpdateLeadsNotes(ctx context.Context, id int32, note string) (*Leads, error)
}

type store struct {
	redis *redis.ClusterClient // note: it's completely acceptable to have a redis client in the store
	db    *sqlx.DB
	store storage.Storage
}

type Config struct {
	ReadConn  *sqlx.DB
	WriteConn *sqlx.DB
	Redis     *redis.ClusterClient
	Tables    []*storage.Table
}

func New(conf *Config) Store {
	tables := []*storage.Table{
		leadsTable,
	}

	// instantiate the storage
	c := &storage.Config{
		ReadOnlyDbConn:  conf.ReadConn,
		WriteOnlyDbConn: conf.WriteConn,
		Redis:           conf.Redis,
		Tables:          tables,
		Debugger:        true,
		ServiceName:     "basic_service",
	}

	s, err := storage.New(c)
	if err != nil {
		panic(err)
	}

	return &store{
		redis: conf.Redis,
		store: s,
		db:    conf.WriteConn,
	}
}
