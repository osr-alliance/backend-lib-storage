package storage

import (
	"context"
	"errors"
	"fmt"
)

func (s *storage) insert(ctx context.Context, objMap *map[string]interface{}, conn InsertInterface) error {
	configKey, err := s.insertOrUpdate(ctx, objMap, conn)
	if err != nil {
		return err
	}

	// now take action on the obj / key
	for _, key := range configKey.Keys {
		fmt.Println("updating key ", key.Key)

		err = s.action(*objMap, actionInsert)
		if err != nil {
			return err
		}

		if err != nil {
			// do not return; we want to update all the keys
			fmt.Println("err in update: ", err)
		}
	}

	return nil
}

func (s *storage) update(ctx context.Context, objMap *map[string]interface{}, conn InsertInterface) error {
	configKey, err := s.insertOrUpdate(ctx, objMap, conn)
	if err != nil {
		return err
	}

	// now take action on the objMap / key
	for _, key := range configKey.Keys {
		fmt.Println("updating key ", key.Key)

		err = s.action(*objMap, actionUpdate)
		if err != nil {
			return err
		}

		if err != nil {
			// do not return; we want to update all the keys
			fmt.Println("err in update: ", err)
		}
	}

	return nil
}

// QUERY SHOULD BE SELECT BUT GOLANG IS A FUCKING BITCH AND WON'T LET ME USE THAT RESERVED KEYWORD AS A FUNCTION NAME EVEN THOUGH IT'S ON TOP OF A STRUCT
func (s *storage) query(ctx context.Context, objMap map[string]interface{}, key *Key) ([]map[string]interface{}, error) {
	fmt.Printf("query: %+v\n", objMap)
	// let's now execute the query
	rows, err := s.readConn.NamedQuery(key.Query, objMap)
	if err != nil {
		fmt.Println("err in set on queryx: ", err.Error(), "query: ", key.Query)
		return nil, err
	}
	// Let's make sure we don't have a memory leak!! :)
	defer rows.Close()

	objs := []map[string]interface{}{}

	for rows.Next() {

		row := map[string]interface{}{}
		err = rows.MapScan(row)
		if err != nil {
			fmt.Println("err in set on rows.MapScan: ", err.Error())
			return nil, err
		}

		// set the struct name
		row[objMapStructNameKey] = objMap[objMapStructNameKey]

		objs = append(objs, row)

		err = s.action(row, actionSelect)
		if err != nil {
			fmt.Println("err in set on s.cache.set: ", err.Error())
			return nil, err
		}
	}

	fmt.Printf("results: %+v\n", objs)

	return objs, nil
}

// delete takes action on all the keys and referenced keys associated with this object
func (s *storage) delete(ctx context.Context, obj map[string]interface{}) error {
	structName := obj[objMapStructNameKey].(string)
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

		s.action(obj, actionDelete)

		if err != nil {
			// do not return; we want to update all the keys
			fmt.Println("err in update: ", err)
		}
	}

	return nil
}

func (s *storage) insertOrUpdate(ctx context.Context, objMap *map[string]interface{}, conn InsertInterface) (*ConfigKey, error) {
	// get the struct's string name to get config key
	structName := (*objMap)[objMapStructNameKey].(string)
	if structName == "" {
		fmt.Println("struct name cannot be blank")
		return nil, errors.New("struct name cannot be blank")
	}

	// get config key
	configKey, ok := s.insertKeys[structName]
	if !ok {
		fmt.Println("no config key found for " + structName)
		return nil, errors.New("no config key found for " + structName)
	}

	// cool, now let's make the actual insert
	rows, err := conn.NamedQuery(configKey.Insert.Query, *objMap)
	if err != nil {
		fmt.Println("err in insert: ", err)
		return nil, err
	}
	defer rows.Close()

	// all inserts & updates should have returning * thus we can just use the obj to scan into
	// there should only be one row but still do rows.Next()

	for rows.Next() {

		err = rows.MapScan(*objMap)
		fmt.Printf("scanned obj: %+v\n", objMap)
		if err != nil {
			fmt.Println("err in scan: ", err)
			return nil, err
		}
	}

	return configKey, nil
}

func (s *storage) action(objMap map[string]interface{}, action actionTypes) error {
	/*
		There's a slight race condition that we might hit if we don't make a new context related to how the grpc connection could
		close due to being done with the query & this happening given enough keys and this usually being in a routine.

		Since we don't need any of the metadata, let's just
	*/

	fmt.Printf("ACTION obj map: %+v\n", objMap)
	ctx := context.Background()

	structName := objMap[objMapStructNameKey].(string)
	if structName == "" {
		return errors.New("struct name cannot be blank")
	}

	configKey, ok := s.insertKeys[structName]
	if !ok {
		return errors.New("no config key found for " + structName)
	}

	for _, key := range configKey.Keys {
		keyName := key.getKeyName(objMap)
		fmt.Println("action on key ", keyName)

		var actionToTake CacheAction
		switch action {
		case actionInsert:
			actionToTake = key.InsertAction
		case actionUpdate:
			actionToTake = key.UpdateAction
		case actionDelete:
			actionToTake = CacheDel
		case actionSelect:
			actionToTake = key.SelectAction
		}

		var err error

		switch actionToTake {
		case CacheNoAction:
			fmt.Println("insert action is CacheNoAction")
			// don't do anything;

		case CacheSet:
			fmt.Println("setting key for ", keyName)
			err = s.cache.set(ctx, keyName, objMap, key.CacheTTL)

		case CacheDel:
			fmt.Println("deleting key for ", keyName)
			s.cache.Del(ctx, keyName)

		case CacheLPush:
			value := objMap[configKey.PrimaryKeyField]
			fmt.Println()
			fmt.Println("pushing value ", value, " to list ", keyName)
			fmt.Println()

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
