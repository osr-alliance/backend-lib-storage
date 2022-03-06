package storage

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/go-redis/redis/v8"
	"golang.org/x/sync/errgroup"
)

func (s *storage) selectOne(ctx context.Context, obj interface{}, queryName string, conn InsertInterface) error {
	v := reflect.ValueOf(obj)
	if v.Kind() != reflect.Ptr {
		return fmt.Errorf("obj not pointer; is %T", obj)
	}

	q, ok := s.queries[queryName]
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
	res, err := s.db.query(ctx, objMap, q.getQuery(objMap), conn)
	if err != nil {
		return err
	}
	if len(res) == 1 {
		objMap = res[0]
	}

	// update the cache
	err = s.cacheActionSelect(objMap, res, s.queries[queryName])
	if err != nil {
		return err
	}

	return mapToStruct(res[0], obj)
}

func (s *storage) selectAll(ctx context.Context, obj interface{}, dest interface{}, queryName string, opts *SelectOptions, conn InsertInterface) error {
	v := reflect.ValueOf(dest)
	if v.Kind() != reflect.Ptr {
		return fmt.Errorf("dest not pointer; is %T", dest)
	}

	err := opts.validateAndParse()
	if err != nil {
		return err
	}

	q, ok := s.queries[queryName]
	if !ok {
		return errors.New("table config not found; have you configured storage properly?")
	}

	objMap, err := structToMapWithOptions(obj, opts)
	if err != nil {
		return err
	}

	// there's no action to take on select so just do the query and return
	if q.SelectAction == CacheNoAction {
		objs, err := s.db.query(ctx, objMap, q.getQuery(objMap), conn)
		if err != nil {
			d("error: %+v", err)
			return err
		}

		return mapsToStruct(objs, dest)
	}

	if opts.FetchAllData {
		err = s.cache.getList(ctx, q, objMap, dest, opts)
		// return if there is a real err. If it's redis.Nil then just keep moving forward
		if err != nil && err != redis.Nil {
			return err
		}
	}

	// get the cache key namme
	keyName := q.getKeyName(objMap)

	exists, err := s.cache.Exists(ctx, keyName).Result()
	if err != nil {
		return err
	}

	if exists == 1 {
		// get the cache value
		// the obj should be of the value that the cache is expecting so we can then just unmarshal into that
		ints := []int32{}
		err = s.cache.LRange(ctx, keyName, int64(opts.Offset), int64(opts.cacheLimit)).ScanSlice(&ints)
		if err != nil {
			return err
		}

		g, ctx := errgroup.WithContext(ctx)

		d("found data in LRange; values: %+v", ints)

		res := []map[string]interface{}{}
		for _, i := range ints {

			// get the row that corresponds to the primary key's id stored -> row
			row := map[string]interface{}{}
			for query, val := range s.queryToMap[q.CachePrimaryQueryStored] {
				row[query] = val
			}

			// set the CachePrimaryQueryStored's primary_key field to i
			row[s.queryToTable[q.CachePrimaryQueryStored].PrimaryKeyField] = i // set the primary key's value

			if opts.FetchAllData {
				if s.disableConcurrency {
					d("fetching without concurrency")
					err = s.selectOne(ctx, &row, q.CachePrimaryQueryStored, conn)
					if err != nil {
						return err
					}
				} else {
					d("fetching with concurrency")
					g.Go(func() error {
						return s.selectOne(ctx, &row, q.CachePrimaryQueryStored, conn)
					})
				}
			}
			res = append(res, row)
		}

		g.Wait()
		d("returning data (unmarshalled): %+v")
		// put the res into the dest (type of []interface to dest's type)

		if opts.FetchAllData {
			s.cache.setList(q, objMap, res, opts)
		}
		return mapsToStruct(res, dest)

	}

	// shouldn't have err here since we've already checked above
	objMap, _ = structToMap(obj)

	// we have an err and it's a redis.Nil which means the value wasn't found in the cache
	// let's get from the database and then set the cache
	objs, err := s.db.query(ctx, objMap, q.getQuery(objMap), conn)
	if err != nil {
		d("error: %+v", err)
		return err
	}

	d("returning data (unmarshalled): %+v", objs)

	d("updating cache")
	// update the cache
	err = s.cacheActionSelect(objMap, objs, s.queries[queryName])
	if err != nil {
		d("error: %+v", err)
		return err
	}

	d("about to selectAll recursively\nObj: %+v\ndest: %+v", obj, dest)

	// this is dangerous...
	return s.selectAll(ctx, obj, dest, queryName, opts, conn)
}

func (s *storage) insert(ctx context.Context, objMap map[string]interface{}, conn InsertInterface) (map[string]interface{}, error) {
	// get the struct's string name to get config key
	structName := objMap[objMapStructNameKey].(string)
	if structName == "" {
		return nil, errors.New("struct name cannot be blank")
	}

	// get config key
	table, ok := s.structToTable[structName]
	if !ok {
		return nil, errors.New("no config key found for " + structName)
	}

	// TODO: set objMap
	res, err := s.db.query(ctx, objMap, table.InsertQuery, conn)
	if err != nil {
		return nil, err
	}

	if len(res) != 1 {
		return nil, errors.New("insert did not return a single row; returned: " + fmt.Sprintf("%d", len(res)))
	}

	// objMap probably has stuff we need, such as private keys, so we'll just overwrite the fields we have and return objMap
	for k, v := range res[0] {
		objMap[k] = v
	}
	return objMap, nil
}

func (s *storage) update(ctx context.Context, objMap map[string]interface{}, conn InsertInterface) (map[string]interface{}, error) {
	// get the struct's string name to get config key
	structName := objMap[objMapStructNameKey].(string)
	if structName == "" {
		return nil, errors.New("struct name cannot be blank")
	}

	// get config key
	table, ok := s.structToTable[structName]
	if !ok {
		return nil, errors.New("no config key found for " + structName)
	}

	res, err := s.db.query(ctx, objMap, table.UpdateQuery, conn)
	if err != nil {
		return nil, err
	}

	if len(res) != 1 {
		return nil, errors.New("update did not return a single row; returned: " + fmt.Sprintf("%d", len(res)))
	}

	// objMap probably has stuff we need, such as private keys, so we'll just overwrite the fields we have and return objMap
	for k, v := range res[0] {
		objMap[k] = v
	}
	return objMap, nil
}

// delete takes action on all the keys and referenced keys associated with this object
func (s *storage) delete(ctx context.Context, obj map[string]interface{}) error {
	return s.actionNonSelect(obj, actionDelete)
}
