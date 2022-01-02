package storage

import (
	"context"

	"github.com/jmoiron/sqlx"
)

type InsertInterface interface {
	NamedQuery(query string, arg interface{}) (*sqlx.Rows, error)
}

type db struct {
	writeConnection *sqlx.DB
	readConnection  *sqlx.DB
}

func newDB(conf *Config) *db {
	return &db{
		writeConnection: conf.WriteOnlyDbConn,
		readConnection:  conf.ReadOnlyDbConn,
	}
}

func (db *db) query(ctx context.Context, objMap map[string]interface{}, query string, conn InsertInterface) ([]map[string]interface{}, error) {
	// let's now execute the query
	rows, err := conn.NamedQuery(query, objMap)
	if err != nil {
		return nil, err
	}
	// Let's make sure we don't have a memory leak!! :)
	defer rows.Close()

	objs := []map[string]interface{}{}

	for rows.Next() {
		row := map[string]interface{}{}
		err = rows.MapScan(row)
		if err != nil {
			return nil, err
		}

		// set the struct name
		row[objMapStructNameKey] = objMap[objMapStructNameKey]
		objs = append(objs, row)
	}

	return objs, nil
}

func (db *db) writeConn() *sqlx.DB {
	return db.writeConnection
}

func (db *db) readConn() *sqlx.DB {
	return db.readConnection
}
