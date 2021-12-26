package storage

import (
	"fmt"

	"github.com/jmoiron/sqlx"
)

type actionTypes int32

const (
	actionSelect actionTypes = iota
	actionInsert
	actionUpdate
	actionDelete
)

const (
	objMapStructNameKey    = "_structName"
	objMapStructPrimaryKey = "_primaryKey"
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
	CacheRPush
)

type CacheDataStructure int32

const (
	CacheDataStructureDefault CacheDataStructure = iota
	CacheDataStructureStruct
	CacheDataStructureList
)

// Query is the struct that holds the config for a query and how it interacts with the cache & db
type Query struct {
	Name int32 // should be a const iota in the package calling this storage package

	Query string // sql query if key isn't in cache e.g. `select * from leads where lead_id=:lead_id`

	/*
		CacheKey defines the associated key name in the cache.
		Example: `service:lead|leads|lead_id:%v`
		NOTE:
		- All keys should have the following format: `service:<service_name>|<table_name>|<field_name>:<field_value>`
		- The table_name can be a custom table but should be filled out e.g. table_name can be `user_is_authorized` if necessary
		- You should use field_name -> field_values wherever you can and chain them if necessary
			`service:group|relation_groups_users|user_id:%v|group_id:%v|relation_type:%v`
				instead of
			`service:group|relation_groups_users|user_id:%v|group_id:%v|relation_type:MODERATOR`
		- use %v to store dynamic values. You can't assume it'll be a string, int32, etc going into the key
	*/
	CacheKey       string
	cacheKeyFields []string // tags of the db fields for the key e.g. if key is `service:lead|leads|lead_id:%v` then the fields would be []string{"lead_id"}

	CacheTTL           int                // time to live in seconds; 0 = default for the application; -1 = never expire
	cacheDataStructure CacheDataStructure // data structure to use for cache e.g. if it's a single object (struct) or a list of id's

	/*
		CachePrimaryKeyStored is the key that stores the data in a list (useful for tables that do joins)
		Note: only applicable if CacheDataStructure == CacheDataStructureList

		An example of where this would be used is if you have a relation table, such as relation_group_user
		and you want to store all the group_id's for a given user. The list would have the group_id's vs. relation_id's
		and fetch & return all the groups instead

		NOTE: this is really important to understand: when doing joins, the table that you insert which should cause the InsertAction is
		the table that this key should be on top of. Example: you would do relationGroupUsersGetGroupsByUserID instead of groupGetByUserID
		because when you insert a new group it wouldn't affect any of the relation_groups_users keys which is really where it should be.
	*/
	CachePrimaryQueryStored int32

	InsertAction CacheAction // action to take on this key when an insert happens to the key this struct is attached to e.g. Del, LPush, etc
	UpdateAction CacheAction // action to take on this key when an update happens to the key this struct is attached to e.g. Del, LPush, etc
	SelectAction CacheAction // action to take on this key when a set happens to the key this struct is attached to (most likely CacheSet)
}

// getKeyName takes a cache's abstract key, e.g. `service:lead|LeadID:%v` and returns the key name e.g. `service:lead|LeadID:1273`
func (q *Query) getKeyName(objMap map[string]interface{}) string {

	args := []interface{}{}
	for _, field := range q.cacheKeyFields {
		args = append(args, objMap[field])
	}

	return fmt.Sprintf(q.CacheKey, args...)
}

type Insert struct {
	Query string
}

type Table struct {
	Struct            interface{} // DB struct this is based off o
	InsertQuery       *Insert     // insert query for inserting data
	PrimaryKeyField   string      // field name of the primary key e.g. LeadID or UserID
	PrimaryQueryName  int32       // the query.Name of the one that fetches based off the primary key in the db e.g. LeadGetByID or OpportunityGetByID
	Queries           []*Query    // all the queries that are used to fetch the data from the db & cache
	ReferencedQueries []*Query    // the query that is used to fetch the data from the db & cache that reference *other* tables
}

type InsertInterface interface {
	NamedQuery(query string, arg interface{}) (*sqlx.Rows, error)
}

type SelectOptions struct {
	Limit    int32
	Offset   int32
	FetchAll bool // FetchAll determines if you return all data or just a list of int32's
}
