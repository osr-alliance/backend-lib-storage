package storage

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-redis/redis/v8"
	"github.com/jmoiron/sqlx"
	"github.com/jmoiron/sqlx/reflectx"
)

/*
TODO:
- No fetching list
- Cache is deleted multiple times on tx.intsert & tx.commit
*/
// Interface defines our API for this package
type Storage interface {
	// TXBegin starts a transaction
	TXBegin(ctx context.Context) (TxInterface, error)

	Insert(ctx context.Context, obj interface{}) error
	Update(ctx context.Context, obj interface{}) error
	Select(ctx context.Context, obj interface{}, key int32) error // Select fills out the obj for its response

	/*
		SelectAll fills out the objs as the response
		Note: you could actually take the key and find the struct, make it a slice of structs, etc but that's actually not the most
		developer-friendly way because then you'd have to type-cast and check the objs being returned. Much easier to do it this
		way from a developer's standpoint
	*/
	SelectAll(ctx context.Context, obj interface{}, objs interface{}, key int32, opts *SelectOptions) error

	DeleteKeys(ctx context.Context, objs ...interface{}) error // Deletes the object's keys from the cache

	// clear's out all of this service's stuff such as during a migration
	Clear(ctx context.Context, serviceName string) error
}

// storage is the private implements the API
type storage struct {
	cache              *cache
	db                 *db
	debugger           bool
	doNotUseCache      bool
	disableConcurrency bool

	/*
		queries is used to *GET* a query from the query.Name
		This is a map to the query.Name -> query
	*/
	queries map[int32]*Query

	/*
		This is a map of query.Name -> table.Struct's name
	*/
	queryToStruct map[int32]string

	/*
		queryToMap takes the query.Name and returns the empty map with the keys that are needed for the query's struct
	*/
	queryToMap map[int32]map[string]interface{}

	/*
		This is a map of the Table.Struct's name -> Table
	*/
	structToTable map[string]*Table // maps struct to the insert key

	queryToTable map[int32]*Table // maps query to the table
}

type Config struct {
	ReadOnlyDbConn     *sqlx.DB
	WriteOnlyDbConn    *sqlx.DB
	Redis              *redis.Client
	Tables             []*Table
	Debugger           bool // turn on / off the debugger
	DoNotUseCache      bool // make sure defaults to bool
	DisableConcurrency bool // used to disable concurrency for testing
}

// New returns group which implements the interface
func New(conf *Config) (Storage, error) {
	// create storage

	// use the json tag instead of the DB tag
	conf.ReadOnlyDbConn.Mapper = reflectx.NewMapperFunc("json", strings.ToLower)
	conf.WriteOnlyDbConn.Mapper = reflectx.NewMapperFunc("json", strings.ToLower)

	// first, set debug up so that we don't get a nil pointer err
	debug = &logger{
		debuggerEnabled: conf.Debugger,
	}

	s := &storage{
		cache:              newCache(conf.Redis),
		db:                 newDB(conf),
		queries:            make(map[int32]*Query),
		queryToStruct:      make(map[int32]string),
		structToTable:      make(map[string]*Table),
		queryToTable:       make(map[int32]*Table),
		queryToMap:         make(map[int32]map[string]interface{}),
		debugger:           conf.Debugger,
		doNotUseCache:      conf.DoNotUseCache,
		disableConcurrency: conf.DisableConcurrency,
	}

	// TODO: validate cache keys
	for _, t := range conf.Tables {

		err := t.validateInsertAndUpdateQueries()
		if err != nil {
			return nil, err
		}

		structName := getStructName(t.Struct)

		// first add the key to keys & keysToStruct
		for _, query := range t.Queries {
			structMap, err := structToMap(t.Struct)
			if err != nil {
				return nil, fmt.Errorf("error getting struct map for %s: %s", structName, err)
			}

			// validate and set the cacheDataStructure
			err = query.parseCacheDatastructure()
			if err != nil {
				return nil, fmt.Errorf("error parsing cache datastructure for %s: %s", query.CacheKey, err)
			}

			// validate and set the cacheFields
			err = query.parseCacheFields()
			if err != nil {
				return nil, fmt.Errorf("error parsing cache fields for %s: %s", query.CacheKey, err)
			}

			// set the cache's cacheListKey
			query.parseCacheListKey()

			structMap[objMapStructPrimaryKey] = t.PrimaryKeyField

			s.queryToMap[query.Name] = structMap

			s.queries[query.Name] = query
			s.queryToStruct[query.Name] = structName
			s.queryToTable[query.Name] = t
		}

		// then add the configKey to the structToTable
		s.structToTable[structName] = t
	}

	return s, s.validatePrimaryQueryStored()
}

func (s *storage) Update(ctx context.Context, obj interface{}) error {
	debug.init(ctx)
	defer debug.clean()
	d("Update() with obj: %+v", obj)

	objMap, err := structToMap(obj)
	if err != nil {
		return err
	}

	// set objMap to the return value
	objMap, err = s.update(ctx, objMap, s.db.writeConn())
	if err != nil {
		return err
	}

	err = s.actionNonSelect(objMap, actionUpdate)
	if err != nil {
		return err
	}

	return mapToStruct(objMap, obj)
}

func (s *storage) Insert(ctx context.Context, obj interface{}) error {
	debug.init(ctx)
	defer debug.clean()
	d("Insert() with obj: %+v", obj)

	objMap, err := structToMap(obj)
	if err != nil {
		return err
	}

	// set objMap to the return value
	objMap, err = s.insert(ctx, objMap, s.db.writeConn())
	if err != nil {
		return err
	}

	err = s.actionNonSelect(objMap, actionInsert)
	if err != nil {
		return err
	}

	return mapToStruct(objMap, obj)
}

func (s *storage) Clear(ctx context.Context, serviceName string) error {
	debug.init(ctx)
	defer debug.clean()
	d("Clear called for service: %s", serviceName)
	return nil
}

func (s *storage) DeleteKeys(ctx context.Context, objs ...interface{}) error {
	debug.init(ctx)
	defer debug.clean()
	d("DeleteKeys() called")

	// really should chain together errors and keep deleting even if an error occurs
	for _, obj := range objs {
		objMap, err := structToMap(obj)
		if err != nil {
			return err
		}
		err = s.delete(ctx, objMap)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *storage) Select(ctx context.Context, obj interface{}, query int32) error {
	debug.init(ctx)
	defer debug.clean()
	d("Select() with obj: %+v, query: %d", obj, query)

	return s.selectOne(ctx, obj, query, s.db.readConn())
}

func (s *storage) SelectAll(ctx context.Context, obj interface{}, dest interface{}, query int32, opts *SelectOptions) error {
	debug.init(ctx)
	defer debug.clean()
	d("SelectAll() with obj: %+v, query: %d, opts: %+v", obj, query, opts)

	return s.selectAll(ctx, obj, dest, query, opts, s.db.readConn())
}
