package storage

import (
	"fmt"
	"reflect"

	"github.com/jmoiron/sqlx"
)

// Define the cache actions you can take
type CacheAction int32

const (
	CacheDefault  CacheAction = iota
	CacheNoAction             // do nothing
	CacheDel
	CacheGet
	CacheSet
	CacheLPush
)

type CacheDataStructure int32

const (
	CacheDataStructureDefault CacheDataStructure = iota
	CacheDataStructureStruct
	CacheDataStructureList
)

// Key is the struct that holds the config for both cache & db and is associated to a specific DB Struct
type Key struct {
	Name int32 // should be a const iota in the package calling this storage package

	Key    string   // key name e.g. `service:lead|LeadID:%v` NOTE: use %v for dynamic values && the db's field names (e.g. LeadID, not lead_id)
	Fields []string // tags of the db fields for the key e.g. if key is `service:lead|LeadID:%v` then the fields would be []string{"LeadID"}
	Query  string   // sql query if key isn't in cache

	CacheTTL           int                // time to live in seconds; 0 = doesn't expire
	CacheDataStructure CacheDataStructure // data structure to use for cache e.g. if it's a single object (struct) or a list of id's

	InsertAction CacheAction // action to take on this key when an insert happens to the key this struct is attached to e.g. Del, LPush, etc
	UpdateAction CacheAction // action to take on this key when an update happens to the key this struct is attached to e.g. Del, LPush, etc
	SetAction    CacheAction // action to take on this key when a set happens to the key this struct is attached to (most likely CacheSet)
}

func (k *Key) getKeyName(obj interface{}) string {
	a := reflect.ValueOf(obj)
	b := reflect.Indirect(a)

	args := []interface{}{}

	for _, field := range k.Fields {
		args = append(args, b.FieldByName(field))
	}

	return fmt.Sprintf(k.Key, args...)
}

type Insert struct {
	Query string
}

type ConfigKey struct {
	Struct                interface{} // DB struct this is based off o
	Insert                *Insert     // insert query for inserting data
	PrimaryKeyField       string      // field name of the primary key e.g. LeadID or UserID
	PrimaryStorageKeyName int32       // the key.Name of the one that fetches based off the primary key in the db e.g. LeadGetByID or OpportunityGetByID
	Keys                  []*Key      // all the keys that are used to fetch the data from the db & cache
	ReferencedKeys        []*Key      // the key that is used to fetch the data from the db & cache that reference *other* tables
}

type InsertInterface interface {
	NamedQuery(query string, arg interface{}) (*sqlx.Rows, error)
}
