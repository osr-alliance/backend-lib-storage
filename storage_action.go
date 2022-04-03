package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"
)

// NOTE: this would ideally be placed on the cache struct but there's too much on the storage struct that we need

/*
	actionRow takes an action on a specific row (not rows) that has been selected, inserted, or updated. This will cause:
	1. new individual structs to be updated in cache (e.g. if the row should be cached such as a lead by lead_id)
	2. If insert or update action, a list can be RPushX or LPushX
		NOTE: if action is a select then lists will NOT be set; a list is only set with actionRows &&

	This is very different than actionRows which will take the queried rows and actually set them in a list
*/
func (s *storage) actionNonSelect(objMap map[string]interface{}, action actionTypes) error {
	d("actionNonSelect")
	if action == actionSelect {
		return errors.New("cannot do actionSelect in actionNonSelect")
	}

	ctx := context.Background()

	structName := objMap[objMapStructNameKey].(string)
	if structName == "" {
		return errors.New("struct name cannot be blank")
	}

	table, ok := s.structToTable[structName]
	if !ok {
		return errors.New("no config key found for " + structName)
	}

	var err error
	for _, q := range append(table.Queries, table.ReferencedQueries...) {

		// check to see if all the cache's fields are what they're supposed to be
		// e.g. check to make sure if there's a != then the column's values don't match
		if !q.isValidQuery(objMap) {
			continue
		}

		var actionToTake CacheAction
		switch action {
		case actionInsert:
			actionToTake = q.InsertAction
		case actionUpdate:
			actionToTake = q.UpdateAction
		case actionDelete:
			actionToTake = CacheDel
		}

		d("taking action: %v on key: %v", actionToTake, q.getKeyName(objMap))
		fmt.Printf("taking action: %v on key: %v", actionToTake, q.getKeyName(objMap))

		if q.cacheDataStructure == CacheDataStructureList {
			err = s.cache.updateCachedSelectAll(q, objMap)
		}

		switch actionToTake {
		case CacheNoAction:
			d("action is CacheNoAction")
			// don't do anything;

		case CacheSet:
			d("action is CacheSet")
			err = s.cache.set(ctx, q.getKeyName(objMap), objMap, q.CacheTTL)

		case CacheDel:
			d("action is CacheDel")
			err = s.cache.Del(ctx, q.getKeyName(objMap)).Err()

		case CacheLPush:
			d("action is CacheLPush")
			// get the map of the struct that's the basis for what's stored in this LRange
			m := s.queryToMap[q.CachePrimaryQueryStored]
			// get the primary key (such as "lead_id" for the lead table) from q.CachePrimaryQueryStored
			pkField, ok := m[objMapStructPrimaryKey].(string)
			if !ok {
				return errors.New("issue getting q.CachePrimaryQueryStored of m")
			}

			// do the insert / update actions with is just going to LPushX (note the X: don't push if key doesn't exist)
			err = s.cache.LPushX(ctx, q.getKeyName(objMap), objMap[pkField]).Err()

		case CacheRPush:
			d("action is CacheRPush")
			// get the map of the struct that's the basis for what's stored in this LRange
			m := s.queryToMap[q.CachePrimaryQueryStored]
			// get the primary key (such as "lead_id" for the lead table) from q.CachePrimaryQueryStored
			pkField, ok := m[objMapStructPrimaryKey].(string)
			if !ok {
				return errors.New("issue getting q.CachePrimaryQueryStored of m")
			}
			// do the insert / update actions with is just going to RPushX (note the X: don't push if key doesn't exist)
			err = s.cache.RPushX(ctx, q.getKeyName(objMap), objMap[pkField]).Err()

		default:
			err = errors.New("unknown update action")
		}

		if err != nil {
			// actually log this error since it's in a loop
			logrus.Errorf("error in actionNonSelect: %s\nkeyName: %s\nAction"+err.Error(), q.getKeyName(objMap), actionToTake)
			// do not return; we want to update all the queries
		}
	}

	return err
}

func (s *storage) cacheActionSelect(objMap map[string]interface{}, objs []map[string]interface{}, query *Query) error {
	d("cacheActionSelect")
	ctx := context.Background()

	// valueToStore represents what will be put into the cache
	objsToInsert := []interface{}{}
	if query.cacheDataStructure == CacheDataStructureList {
		d("query is CacheDataStructureList")
		// get the map of the struct that's the basis for what's stored in this LRange
		m := s.queryToMap[query.CachePrimaryQueryStored]
		// get the primary key (such as "lead_id" for the lead table) from q.CachePrimaryQueryStored
		pkField, ok := m[objMapStructPrimaryKey].(string)
		if !ok {
			return errors.New("issue getting q.CachePrimaryQueryStored of m")
		}

		for _, v := range objs {
			objsToInsert = append(objsToInsert, v[pkField])
		}
	}

	var err error
	keyName := query.getKeyName(objMap)

	d("keyName: %s", keyName)

	switch query.SelectAction {
	case CacheNoAction:
		d("action is CacheNoAction")
		// don't do anything;

	case CacheSet:
		d("cacheActionSelect: CacheSet\nobjMap: %+v", objMap)
		err = s.cache.set(ctx, keyName, objMap, query.CacheTTL)

	case CacheDel:
		d("cacheActionSelect: CacheDel")
		err = s.cache.Del(ctx, keyName).Err()

	case CacheLPush:
		if len(objsToInsert) == 0 {
			break
		}
		d("cacheActionSelect: CacheLPush. objsToInsert:", objsToInsert)
		err = s.cache.LPush(ctx, keyName, objsToInsert...).Err()

	case CacheRPush:
		if len(objsToInsert) == 0 {
			break
		}
		d("cacheActionSelect: RPush. objsToInsert:", objsToInsert)
		err = s.cache.RPush(ctx, keyName, objsToInsert...).Err()

	default:
		err = errors.New("unknown update action")
	}
	return err
}
