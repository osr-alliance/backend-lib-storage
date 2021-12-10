package storage

import (
	"context"
	"errors"
	"fmt"
)

func (s *storage) insert(ctx context.Context, obj interface{}, conn InsertInterface) error {
	err, configKey := s.insertOrUpdate(ctx, obj, conn)
	if err != nil {
		return err
	}

	go func() {
		// now take action on the obj / key
		for _, key := range configKey.Keys {
			fmt.Println("updating key ", key.Key)
			var err error

			s.action(obj, key.InsertAction)

			if err != nil {
				// do not return; we want to update all the keys
				fmt.Println("err in update: ", err)
			}
		}
	}()

	return nil
}

func (s *storage) update(ctx context.Context, obj interface{}, conn InsertInterface) error {
	err, configKey := s.insertOrUpdate(ctx, obj, conn)
	if err != nil {
		return err
	}

	go func() {
		// now take action on the obj / key
		for _, key := range configKey.Keys {
			fmt.Println("updating key ", key.Key)
			var err error

			s.action(obj, key.UpdateAction)

			if err != nil {
				// do not return; we want to update all the keys
				fmt.Println("err in update: ", err)
			}
		}
	}()

	return nil
}

// QUERY SHOULD BE SELECT BUT GOLANG IS A FUCKING BITCH AND WON'T LET ME USE THAT RESERVED KEYWORD AS A FUNCTION NAME EVEN THOUGH IT'S ON TOP OF A STRUCT
func (s *storage) query(ctx context.Context, obj interface{}, key *Key) error {
	fmt.Println("in s.get()")

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

		go s.action(obj, key.SelectAction)

		if err != nil {
			fmt.Println("err in set on s.cache.set: ", err.Error())
			return err
		}
		fmt.Printf("results: %+v\n", obj)
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

		go s.action(obj, key.InsertAction)

		if err != nil {
			// do not return; we want to update all the keys
			fmt.Println("err in update: ", err)
		}
	}

	return nil
}

func (s *storage) insertOrUpdate(ctx context.Context, obj interface{}, conn InsertInterface) (error, *ConfigKey) {
	fmt.Println("inserting")

	// get the struct's string name to get config key
	structName := getStructName(obj)
	if structName == "" {
		fmt.Println("struct name cannot be blank")
		return errors.New("struct name cannot be blank"), nil
	}

	// get config key
	configKey, ok := s.insertKeys[structName]
	if !ok {
		fmt.Println("no config key found for " + structName)
		return errors.New("no config key found for " + structName), nil
	}

	// cool, now let's make the actual insert
	fmt.Println("query is: ", configKey.Insert.Query)
	fmt.Printf("obj: %+v\n", obj)
	rows, err := conn.NamedQuery(configKey.Insert.Query, obj)
	if err != nil {
		fmt.Println("err in insert: ", err)
		return err, nil
	}
	defer rows.Close()

	// all inserts & updates should have returning * thus we can just use the obj to scan into
	// there should only be one row but still do rows.Next()
	for rows.Next() {
		err = rows.StructScan(obj)
		fmt.Printf("scanned obj: %+v\n", obj)
		if err != nil {
			fmt.Println("err in scan: ", err)
			return err, nil
		}
	}

	return nil, configKey
}

func (s *storage) action(obj interface{}, action CacheAction) error {
	/*
		There's a slight race condition that we might hit if we don't make a new context related to how the grpc connection could
		close due to being done with the query & this happening given enough keys and this usually being in a go routine.

		Since we don't need any of the metadata, let's just
	*/

	ctx := context.Background()

	fmt.Println("in s.action")
	structName := getStructName(obj)
	if structName == "" {
		return errors.New("struct name cannot be blank")
	}

	configKey, ok := s.insertKeys[structName]
	if !ok {
		return errors.New("no config key found for " + structName)
	}

	for _, key := range configKey.Keys {
		fmt.Println("action on key ", key.Key)
		var err error

		switch action {
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

/*
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
*/
