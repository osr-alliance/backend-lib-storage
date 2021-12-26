package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"

	"github.com/go-redis/redis/v8"
)

func (s *storage) selectOne(ctx context.Context, obj interface{}, query int32, conn InsertInterface) error {
	v := reflect.ValueOf(obj)
	if v.Kind() != reflect.Ptr {
		return fmt.Errorf("obj not pointer; is %T", obj)
	}

	q, ok := s.queries[query]
	if !ok {
		return errors.New("config query not found; have you configured storage properly?")
	}

	objMap, err := structToMap(obj)
	if err != nil {
		return err
	}

	// get the cache key namme
	keyName := q.getKeyName(objMap)

	// get the cache value
	// the obj should be of the value that the cache is expecting so we can then just unmarshal into that
	err = s.cache.get(ctx, keyName, obj)
	if err == nil {
		// we found the value in the cache
		// object should already be set in the obj
		return nil
	}

	// check to see if there's a real error
	if err != nil && err != redis.Nil {
		return err
	}

	// we have an err and it's a redis.Nil which means the value wasn't found in the cache
	// let's get from the database and then set the cache
	res, err := s.query(ctx, objMap, q, conn)
	if err == nil && len(res) == 0 {
		return sql.ErrNoRows
	}
	if err != nil {
		return err
	}
	if len(res) == 1 {
		objMap = res[0]
	}

	// update the cache
	err = s.cacheActionSelect(objMap, res, s.queries[query])
	if err != nil {
		return err
	}

	return mapToStruct(res[0], obj)
}

func (s *storage) selectAll(ctx context.Context, obj interface{}, dest interface{}, query int32, opts *SelectOptions, conn InsertInterface) error {
	v := reflect.ValueOf(dest)
	if v.Kind() != reflect.Ptr {
		return fmt.Errorf("dest not pointer; is %T", dest)
	}

	q, ok := s.queries[query]
	if !ok {
		return errors.New("table config not found; have you configured storage properly?")
	}

	objMap, err := structToMap(obj)
	if err != nil {
		return err
	}

	// get the cache key namme
	keyName := q.getKeyName(objMap)

	// get the cache value
	// the obj should be of the value that the cache is expecting so we can then just unmarshal into that
	ints := []int32{}
	err = s.cache.LRange(ctx, keyName, int64(opts.Offset), int64(opts.Limit)).ScanSlice(&ints)
	// LRange doens't throw err when key doesn't exist for some fucking reason (TODO: check with s.cache.Exists())
	if err == nil && len(ints) > 0 {

		res := []map[string]interface{}{}
		for _, i := range ints {

			// get the row that corresponds to the primary key's id stored -> row
			row := map[string]interface{}{}
			for query, val := range s.queryToMap[q.CachePrimaryQueryStored] {
				row[query] = val
			}

			// set the CachePrimaryQueryStored's primary_key field to i
			row[s.queryToTable[q.CachePrimaryQueryStored].PrimaryKeyField] = i // set the primary key's value

			// only fetch row from database if it's required
			if opts.FetchAll {
				err = s.Select(ctx, &row, q.CachePrimaryQueryStored)
				if err != nil {
					return err
				}
			}

			res = append(res, row)
		}

		// put the res into the dest (type of []interface to dest's type)
		return mapsToStruct(res, dest)
	}

	// check to see if there's a real error
	if err != nil && err != redis.Nil {
		return err
	}

	// todo: validate limit and offset fields are not set before setting the keys

	// we have an err and it's a redis.Nil which means the value wasn't found in the cache
	// let's get from the database and then set the cache
	objs, err := s.query(ctx, objMap, q, conn)
	if len(objs) == 0 {
		return sql.ErrNoRows
	}
	if err != nil {
		return err
	}

	// update the cache
	err = s.cacheActionSelect(objMap, objs, s.queries[query])
	if err != nil {
		return err
	}

	// this is dangerous...
	return s.SelectAll(ctx, obj, dest, query, opts)
}

func (s *storage) insert(ctx context.Context, objMap *map[string]interface{}, conn InsertInterface) error {
	return s.insertOrUpdate(ctx, objMap, conn)
}

func (s *storage) update(ctx context.Context, objMap *map[string]interface{}, conn InsertInterface) error {
	return s.insertOrUpdate(ctx, objMap, conn)
}

// delete takes action on all the keys and referenced keys associated with this object
func (s *storage) delete(ctx context.Context, obj map[string]interface{}) error {
	return s.actionNonSelect(obj, actionDelete)
}

func (s *storage) insertOrUpdate(ctx context.Context, objMap *map[string]interface{}, conn InsertInterface) error {
	// get the struct's string name to get config key
	structName := (*objMap)[objMapStructNameKey].(string)
	if structName == "" {
		return errors.New("struct name cannot be blank")
	}

	// get config key
	table, ok := s.structToTable[structName]
	if !ok {
		return errors.New("no config key found for " + structName)
	}

	// cool, now let's make the actual insert
	rows, err := conn.NamedQuery(table.InsertQuery.Query, *objMap)
	if err != nil {
		return err
	}
	defer rows.Close()

	// all inserts & updates should have returning * thus we can just use the obj to scan into
	// there should only be one row but still do rows.Next()

	for rows.Next() {
		err = rows.MapScan(*objMap)
		if err != nil {
			return err
		}
	}

	return nil
}

// QUERY SHOULD BE SELECT BUT GOLANG IS A FUCKING BITCH AND WON'T LET ME USE THAT RESERVED KEYWORD AS A FUNCTION NAME EVEN THOUGH IT'S ON TOP OF A STRUCT
func (s *storage) query(ctx context.Context, objMap map[string]interface{}, query *Query, conn InsertInterface) ([]map[string]interface{}, error) {
	// let's now execute the query
	rows, err := conn.NamedQuery(query.Query, objMap)
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
