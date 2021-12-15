package storage

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/go-redis/redis/v8"
	"github.com/jmoiron/sqlx"
)

/*
TODO:
- No fetching list
- Cache is deleted multiple times on tx.intsert & tx.commit
*/
// Interface defines our API for this package
type Storage interface {
	// TXBegin starts a transaction
	TXBegin(ctx context.Context) (*sqlx.Tx, error)
	// TXCommit commits a transaction and updates the obj
	TXInsert(ctx context.Context, tx *sqlx.Tx, obj interface{}) error
	// TXUpdate commits a transaction and updates the obj
	TXUpdate(ctx context.Context, tx *sqlx.Tx, obj interface{}) error
	// TXEnd ends a transaction && updates all the keys in the cache after commit
	TXEnd(ctx context.Context, tx *sqlx.Tx, obj ...interface{}) error

	Insert(ctx context.Context, obj interface{}) error
	Update(ctx context.Context, obj interface{}) error
	Select(ctx context.Context, obj interface{}, key int32) error // Select fills out the obj for its response

	/*
		SelectAll fills out the objs as the response
		Note: you could actually take the key and find the struct, make it a slice of structs, etc but that's actually not the most
		developer-friendly way because then you'd have to type-cast and check the objs being returned. Much easier to do it this
		way from a developer's standpoint
	*/
	SelectAll(ctx context.Context, obj interface{}, objs interface{}, key int32, offset int64, limit int64) error

	DeleteKeys(ctx context.Context, objs ...interface{}) error // Deletes the object's keys from the cache

	// clear's out all of this service's stuff such as during a migration
	Clear(ctx context.Context, serviceName string) error
}

// storage is the private implements the API
type storage struct {
	readConn  *sqlx.DB
	writeConn *sqlx.DB
	cache     *cache

	/*
		keys is used to *GET* a key from the key.Name
		This is a map to the key.Name -> key
	*/
	keys map[int32]*Key

	/*
		keys is used to get the struct name of the struct that the key is for
		This is a map of key.Name -> struct name
	*/
	keysToStruct map[int32]string

	/*
		insertKeys is used to *INSERT* a key or query into the database
		This is a map of the configKey.Type -> configKey
	*/
	insertKeys map[string]*ConfigKey // maps struct to the insert key

	keysToConfigKey map[int32]*ConfigKey // maps key to the config key
}

type Config struct {
	ReadOnlyDbConn  *sqlx.DB
	WriteOnlyDbConn *sqlx.DB
	Redis           *redis.Client
	ConfigKeys      []*ConfigKey
	DoNotUseCache   bool // make sure defaults to bool
}

// New returns group which implements the interface
func New(conf *Config) Storage {
	// create storage
	s := &storage{
		readConn:        conf.ReadOnlyDbConn,
		writeConn:       conf.WriteOnlyDbConn,
		cache:           newCache(conf.Redis),
		keys:            make(map[int32]*Key),
		keysToStruct:    make(map[int32]string),
		insertKeys:      make(map[string]*ConfigKey),
		keysToConfigKey: make(map[int32]*ConfigKey),
	}

	// TODO: validate cache keys
	for _, confKey := range conf.ConfigKeys {
		structName := getStructName(confKey.Struct)

		// first add the key to keys & keysToStruct
		for _, key := range confKey.Keys {
			s.keys[key.Name] = key
			s.keysToStruct[key.Name] = structName
			s.keysToConfigKey[key.Name] = confKey
		}

		// then add the configKey to the insertKeys
		s.insertKeys[structName] = confKey
	}

	return s
}

func (s *storage) TXBegin(ctx context.Context) (*sqlx.Tx, error) {
	return s.writeConn.BeginTxx(ctx, nil)
}

func (s *storage) TXInsert(ctx context.Context, tx *sqlx.Tx, obj interface{}) error {
	return s.insert(ctx, obj, tx)
}

func (s *storage) TXUpdate(ctx context.Context, tx *sqlx.Tx, obj interface{}) error {
	return s.update(ctx, obj, tx)
}

func (s *storage) TXEnd(ctx context.Context, tx *sqlx.Tx, obj ...interface{}) error {
	return tx.Commit()
}

func (s *storage) Update(ctx context.Context, obj interface{}) error {
	return s.update(ctx, obj, s.writeConn)
}

func (s *storage) Insert(ctx context.Context, obj interface{}) error {
	return s.insert(ctx, obj, s.writeConn)
}

func (s *storage) Select(ctx context.Context, obj interface{}, key int32) error {
	k, ok := s.keys[key]
	if !ok {
		return errors.New("config key not found; have you configured storage properly?")
	}

	// get the cache key namme
	keyName := k.getKeyName(obj)
	fmt.Printf("keyName in Select: %v\nObj: %+v\n", keyName, obj)

	// get the cache value
	// the obj should be of the value that the cache is expecting so we can then just unmarshal into that
	err := s.cache.get(ctx, keyName, obj)
	if err == nil {
		// we found the value in the cache
		fmt.Println("found in cache; returning")
		// object should already be set in the obj
		return nil
	}

	// check to see if there's a real error
	if err != nil && err != redis.Nil {
		fmt.Println("cache get error in select: ", err)
		return err
	}

	// we have an err and it's a redis.Nil which means the value wasn't found in the cache
	// let's get from the database and then set the cache
	_, err = s.query(ctx, obj, k)
	if err != nil {
		fmt.Println("err in setting the value: ", err)
		return err
	}

	// TODO: if key.CacheDataStructure == CacheDataStructureList then we need to get the actual values & not just the IDs

	return nil
}

func (s *storage) Clear(ctx context.Context, serviceName string) error {
	return nil
}

func (s *storage) DeleteKeys(ctx context.Context, objs ...interface{}) error {
	for _, obj := range objs {
		err := s.action(obj, actionDelete)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *storage) SelectAll(ctx context.Context, obj interface{}, dest interface{}, key int32, offset int64, limit int64) error {
	fmt.Printf("insertKeys: %+v\nkeys: %+v\n", s.insertKeys, s.keys)

	k, ok := s.keys[key]
	if !ok {
		return errors.New("key not found")
	}

	// get the cache key namme
	keyName := k.getKeyName(obj)
	fmt.Printf("keyName: %v\nObj: %+v\n", keyName, obj)

	// get the cache value
	// the obj should be of the value that the cache is expecting so we can then just unmarshal into that
	ints := []int32{}
	err := s.cache.LRange(ctx, keyName, offset, limit).ScanSlice(&ints)

	// LRange doens't throw err when key doesn't exist for some fucking reason
	if err == nil && len(ints) > 0 {
		confKey := s.keysToConfigKey[key]
		// create duplicate of the struct
		dup := reflect.New(reflect.TypeOf(confKey.Struct))

		res := []interface{}{}
		for _, i := range ints {
			dup.Elem().FieldByName(confKey.PrimaryKeyField).SetInt(int64(i))
			err = s.Select(ctx, dup.Interface(), confKey.PrimaryStorageKeyName)
			if err != nil {
				return err
			}
			res = append(res, dup.Interface())
		}

		// put the res into the dest (type of []interface to dest's type)
		return scanToDest(res, dest)
	}

	// check to see if there's a real error
	if err != nil && err != redis.Nil {
		fmt.Println("cache lrange error: ", err)
		return err
	}

	// todod: validate that the obj is a slice

	// we have an err and it's a redis.Nil which means the value wasn't found in the cache
	// let's get from the database and then set the cache
	res, err := s.query(ctx, obj, k)
	if err != nil {
		fmt.Println("err in setting the value: ", err)
		return err
	}

	// put the res into the dest (type of []interface to dest's type)
	return scanToDest(res, dest)
}
