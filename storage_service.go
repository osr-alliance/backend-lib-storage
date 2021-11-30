package storage

import (
	"context"
	"errors"
	"fmt"
)

func (s *storage) insert(ctx context.Context, obj interface{}, conn InsertInterface) error {
	fmt.Println("inserting")

	// get the struct's string name to get config key
	structName := getStructName(obj)
	if structName == "" {
		fmt.Println("struct name cannot be blank")
		return errors.New("struct name cannot be blank")
	}

	// get config key
	configKey, ok := s.insertKeys[structName]
	if !ok {
		fmt.Println("no config key found for " + structName)
		return errors.New("no config key found for " + structName)
	}

	// cool, now let's make the actual insert
	fmt.Println("query is: ", configKey.Insert.Query)
	fmt.Printf("obj: %+v\n", obj)
	rows, err := conn.NamedQuery(configKey.Insert.Query, obj)
	if err != nil {
		fmt.Println("err in insert: ", err)
		return err
	}
	defer rows.Close()

	// all inserts should have returning * thus we can just use the obj to scan into
	for rows.Next() {
		err = rows.StructScan(obj)
		fmt.Printf("scanned obj: %+v\n", obj)
		if err != nil {
			fmt.Println("err in scan: ", err)
			return err
		}
	}

	fmt.Println("inserting into redis now")
	err = s.delete(ctx, obj)
	if err != nil {
		fmt.Println("err in update: ", err)
		return err
	}
	return nil
}

// delete takes action on all the keys and referenced keys associated with this object
func (s *storage) delete(ctx context.Context, obj interface{}) error {
	fmt.Println("in s.insert")
	structName := getStructName(obj)
	if structName == "" {
		return errors.New("struct name cannot be blank")
	}

	configKey, ok := s.insertKeys[structName]
	if !ok {
		return errors.New("no config key found for " + structName)
	}

	for _, key := range configKey.Keys {
		fmt.Println("updating key ", key.Key)
		var err error

		switch key.InsertAction {
		case CacheNoAction:
			fmt.Println("insert action is CacheNoAction")
			// don't do anything;

		case CacheSet:
			keyName := key.getKeyName(obj)
			fmt.Println("setting key for ", keyName)
			err = s.cache.set(ctx, keyName, obj, key.CacheTTL)

		case CacheDel:
			keyName := key.getKeyName(obj)
			fmt.Println("deleting key for ", keyName)
			s.cache.Del(ctx, keyName)

		case CacheLPush:
			value := getFieldValueByName(configKey.PrimaryKeyField, obj)
			keyName := key.getKeyName(obj)
			fmt.Println("LPushing key for ", keyName)
			if value == nil {
				err = errors.New("value is nil")
			}
			s.cache.LPush(ctx, keyName, value)

		default:
			err = errors.New("unknown update action")
		}

		if err != nil {
			// do not return; we want to update all the keys
			fmt.Println("err in update: ", err)
		}
	}

	return nil
}

// get gets the values from the db and populuates the cache
func (s *storage) get(ctx context.Context, obj interface{}, key *Key) error {
	fmt.Println("in s.get()")

	// first we need to get the primary key's field for this struct
	primaryKeyField, err := s.getPrimaryKeyField(obj, key)
	if err != nil {
		fmt.Println("err in getPrimaryKeyField: ", err)
	}

	// let's now execute the query
	rows, err := s.readConn.NamedQuery(key.Query, obj)
	if err != nil {
		fmt.Println("err in set on queryx: ", err.Error(), "query: ", key.Query)
		return err
	}
	// Let's make sure we don't have a memory leak!! :)
	defer rows.Close()

	for rows.Next() {
		err = rows.StructScan(obj)
		if err != nil {
			fmt.Println("err in set on rows.MapScan: ", err.Error())
			return err
		}

		// Next we want to figure out how to set this value in the cache
		// e.g. is this a list and do we push? do we set? etc
		switch key.SetAction {
		case CacheNoAction:
			fmt.Println("get action is CacheNoAction")
			// don't do anything

		case CacheSet:
			err = s.cache.set(ctx, key.getKeyName(obj), obj, key.CacheTTL)

		case CacheLPush:
			// TODO: getFieldValueByName actually does reflection on every loop. Really we should set the reflection.ValueOf and reuse it in
			// here to get the field value
			value := getFieldValueByName(primaryKeyField, obj)
			if value == nil {
				err = errors.New("value is nil")
			}
			s.cache.LPush(ctx, key.getKeyName(obj), value)

		default:
			return errors.New("unknown cache data structure")
		}

		if err != nil {
			fmt.Println("err in set on s.cache.set: ", err.Error())
			return err
		}
		fmt.Printf("results: %+v\n", obj)
	}

	return nil
}

func (s *storage) getPrimaryKeyField(obj interface{}, key *Key) (string, error) {
	if key.CacheDataStructure != CacheDataStructureList {
		// if it's not a list then you don't need the primary key
		return "", nil
	}

	structName := getStructName(obj)
	if structName == "" {
		return "", errors.New("struct name cannot be blank")
	}

	configKey, ok := s.insertKeys[structName]
	if !ok {
		return "", errors.New("no config key found for " + structName)
	}

	primaryKeyField := configKey.PrimaryKeyField

	if primaryKeyField == "" {
		return "", errors.New("no primary key field found for " + structName)
	}

	return primaryKeyField, nil
}
