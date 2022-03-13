package storage

import (
	"fmt"
	"reflect"
	"strings"
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

	cacheKeyListModifier         = "|offset:%v|limit:%v"
	cacheKeyListMetadataModifier = "|metadata"
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

type cacheKeyFieldOperator int32

const (
	operatorDefault cacheKeyFieldOperator = iota
	operatorEqual
	operatorNotEqual
)

type CacheDataStructure int32

const (
	CacheDataStructureDefault CacheDataStructure = iota
	CacheDataStructureStruct
	CacheDataStructureList
)

// Query is the struct that holds the config for a query and how it interacts with the cache & db
type Query struct {
	Name string // should be a const string in the package calling this storage package

	Query            string // sql query if key isn't in cache e.g. `select * from leads where lead_id=:lead_id`
	queryLimitOffset string // Query but with limit & offset set in the query
	// Names of the parameters that are slices in the query
	// e.g. `select * from users where user_id in (:user_ids)` so :user_ids would be placed here
	slicesInQuery map[string]reflect.Type

	tableName string // the table name that this query is for; added for optimization purposes

	/*
		CacheKey defines the associated key name in the cache.
		- It is formatted `<column name><operator><column value>`
		- There are two operators: = and !=
		- You can have extra info in the key as long as it doesn't have an operator e.g. `lead_id=%v|filter:`

		Example: `lead_id=%v` (for the leads table) or `group_id=%v|role!=OWNER` (for the relation_groups_users table)

		NOTE:
			- no key that has a list cache datastructure can end in `|metadata`
			- no key that has a list cache datastructure can end in `|offset%v|limit%v`
			- There are two operators: = and !=
			- The = operator (e.g. `lead_id=%v`) is used for exact value matches from the column. If it is not matched then the key doesn't get a hit
			- The != operator (e.g. `group_id=%v|role!=OWNER`) is used for non-exact value matches from the column. If it is matched then the key doesn't get a hit.
				- The way this works is if there's an insert on the table with this key then it basically checks to see the column of `role`'s value
					and if not OWNER then it will do the insert action on this key





		- All keys should have the following format: `<field_name>:<field_value>`
		- The table_name can be a custom table but should be filled out e.g. table_name can be `user_is_authorized` if necessary
		- You should use field_name -> field_values wherever you can and chain them if necessary
			`service:group|relation_groups_users|user_id:%v|group_id:%v|relation_type:%v`
				instead of
			`service:group|relation_groups_users|user_id:%v|group_id:%v|relation_type:MODERATOR`
		-
	*/
	CacheKey     string
	fullCacheKey string // adds the service name & table name to the beginning of the cache key

	cacheKeyFields                    []cacheKeyField // tags of the db fields for the key e.g. if key is `lead_id=%v` then the fields would be []string{"lead_id"}
	cacheKeyContainsNotEqualsOperator bool            // if the key contains a `!=` operator
	cacheListKey                      string          // cacheListKey is the generated key name for when the cacheDataStructure is a list

	CacheTTL           int                // time to live in seconds; 0 = default for the application; -1 = never expire
	cacheDataStructure CacheDataStructure // data structure to use for cache e.g. if it's a single object (struct) or a list of id's

	//cacheListKeys        []string           // cacheListKeys stores the keys associated with SelectAll calls where selectOpts is defined
	cacheListMetadataKey string // cacheListMetadataKey is the key for the metadata associated with the list

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
	CachePrimaryQueryStored string

	InsertAction CacheAction // action to take on this key when an insert happens to the key this struct is attached to e.g. Del, LPush, etc
	UpdateAction CacheAction // action to take on this key when an update happens to the key this struct is attached to e.g. Del, LPush, etc
	SelectAction CacheAction // action to take on this key when a set happens to the key this struct is attached to (most likely CacheSet)
}

// getKeyName takes a cache's abstract key, e.g. `lead_id:%v` and returns the key name e.g. `service:lead|Lead|lead_id:1273`
func (q *Query) getKeyName(objMap map[string]interface{}) string {

	args := []interface{}{}
	for _, field := range q.cacheKeyFields {
		if field.operator == operatorNotEqual {
			continue
		}
		args = append(args, objMap[field.columnName])
	}

	return fmt.Sprintf(q.fullCacheKey+"|"+q.CacheKey, args...)
}

func (q *Query) getKeyNameSelectOpts(objMap map[string]interface{}, opts *SelectOptions) string {
	args := []interface{}{}
	for _, field := range q.cacheKeyFields {
		if field.operator == operatorNotEqual {
			continue
		}
		args = append(args, objMap[field.columnName])
	}

	args = append(args, opts.Offset, opts.Limit)

	return fmt.Sprintf(q.fullCacheKey+"|"+q.cacheListKey, args...)
}

func (q *Query) getKeyNameMetadata(objMap map[string]interface{}) string {
	args := []interface{}{}
	for _, field := range q.cacheKeyFields {
		if field.operator == operatorNotEqual {
			continue
		}
		args = append(args, objMap[field.columnName])
	}

	return fmt.Sprintf(q.fullCacheKey+"|"+q.cacheListMetadataKey, args...)
}

func (q *Query) getQuery(objMap map[string]interface{}) (string, error) {
	query := func() string {
		limit, ok := objMap["limit"]
		if !ok {
			// limit isn't here thus return Query
			return q.Query
		}

		// if the limit is > 0 for a list then return the queryLimitOffset
		if q.cacheDataStructure == CacheDataStructureList && limit.(int32) > 0 {
			return q.queryLimitOffset
		}
		return q.Query
	}()

	if len(q.slicesInQuery) != 0 {
		for parameter, sliceType := range q.slicesInQuery {
			parameterValue := ""
			switch sliceType {
			case reflect.TypeOf([]string{}):
				// check if slice is nil
				v, ok := objMap[parameter].([]interface{})
				if !ok {
					return "", fmt.Errorf("%v cannot be nil", parameter)
				}

				inStrings := []string{}
				for _, value := range v {
					inStrings = append(inStrings, value.(string))
				}
				parameterValue = fmt.Sprintf("'%s'", strings.Join(inStrings, "', '"))
			default:
				// check if slice is nil
				v, ok := objMap[parameter].([]interface{})
				if !ok {
					return "", fmt.Errorf("%v cannot be nil", parameter)
				}

				// if it's not a string then all the ints automatically get converted to int64
				inNumbers := []float64{}
				for _, value := range v {
					inNumbers = append(inNumbers, value.(float64))
				}
				parameterValue = strings.Join(strings.Split(fmt.Sprint(inNumbers), " "), ", ")
				parameterValue = strings.Replace(parameterValue, "[", "", -1)
				parameterValue = strings.Replace(parameterValue, "]", "", -1)
			}
			query = strings.Replace(query, fmt.Sprintf(":%s", parameter), parameterValue, -1)
		}
	}

	return query, nil
}

func (q *Query) isValidQuery(objMap map[string]interface{}) bool {
	if !q.cacheKeyContainsNotEqualsOperator {
		// if it doesn't have an equal operator then it's automatically valid
		return true
	}

	for _, field := range q.cacheKeyFields {
		if field.operator == operatorNotEqual && objMap[field.columnName] == field.value {
			d("not equal operator found for field %v", field.columnName)
			return false // the objMap has a field that matches the key's value
		}
	}
	return true
}

type cacheKeyField struct {
	columnName string
	operator   cacheKeyFieldOperator
	value      string
}

type Insert struct {
	Query string
}

type Table struct {
	Struct interface{}            // DB struct this is based off of
	objMap map[string]interface{} // generic map that has the struct's fields + other info as keys and the values unset

	InsertQuery       string // insert query for inserting data
	UpdateQuery       string
	PrimaryKeyField   string   // field name of the primary key e.g. LeadID or UserID
	PrimaryQueryName  string   // the query.Name of the one that fetches based off the primary key in the db e.g. LeadGetByID or OpportunityGetByID
	Queries           []*Query // all the queries that are used to fetch the data from the db & cache
	ReferencedQueries []*Query // the query that is used to fetch the data from the db & cache that reference *other* tables

	tableName string // defines the name of the table based off the struct name
}

type SelectOptions struct {
	Offset int32

	// Limit is the number of items to return; if all then use 0
	Limit int32

	cacheLimit int32 // the limit that is used for the cache

	FetchAllData bool // FetchAll determines if you return all data or just a list of int32's
}

func (s *SelectOptions) validateAndParse() error {
	if s.Limit < 0 {
		return fmt.Errorf("limit must be >= 0 & follows postgres convention")
	}

	if s.Offset < 0 {
		return fmt.Errorf("offset must be >= 0 & follows postgres convention")
	}

	s.cacheLimit = s.Limit - 1

	return nil
}
