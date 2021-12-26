package storage

import (
	"context"
	"errors"
)

/*
	actionRow takes an action on a specific row (not rows) that has been selected, inserted, or updated. This will cause:
	1. new individual structs to be updated in cache (e.g. if the row should be cached such as a lead by lead_id)
	2. If insert or update action, a list can be RPushX or LPushX
		NOTE: if action is a select then lists will NOT be set; a list is only set with actionRows &&

	This is very different than actionRows which will take the queried rows and actually set them in a list
*/
func (s *storage) actionNonSelect(objMap map[string]interface{}, action actionTypes) error {
	if action == actionSelect {
		return errors.New("cannot do actionSelect in actionNonSelect...")
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

	for _, k := range table.Queries {
		var actionToTake CacheAction
		switch action {
		case actionInsert:
			actionToTake = k.InsertAction
		case actionUpdate:
			actionToTake = k.UpdateAction
		case actionDelete:
			actionToTake = CacheDel
		}

		var err error

		switch actionToTake {
		case CacheNoAction:
			//fmt.Println("insert action is CacheNoAction")
			// don't do anything;

		case CacheSet:
			err = s.cache.set(ctx, k.getKeyName(objMap), objMap, k.CacheTTL)

		case CacheDel:
			err = s.cache.Del(ctx, k.getKeyName(objMap)).Err()

		case CacheLPush:
			// get the map of the struct that's the basis for what's stored in this LRange
			m := s.queryToMap[k.CachePrimaryQueryStored]
			// get the primary key (such as "lead_id" for the lead table) from k.CachePrimaryQueryStored
			pkField, ok := m[objMapStructPrimaryKey].(string)
			if !ok {
				return errors.New("issue getting k.CachePrimaryQueryStored of m")
			}

			// do the insert / update actions with is just going to LPushX (note the X: don't push if key doesn't exist)
			err = s.cache.LPushX(ctx, k.getKeyName(objMap), objMap[pkField]).Err()

		case CacheRPush:
			// get the map of the struct that's the basis for what's stored in this LRange
			m := s.queryToMap[k.CachePrimaryQueryStored]
			// get the primary key (such as "lead_id" for the lead table) from k.CachePrimaryQueryStored
			pkField, ok := m[objMapStructPrimaryKey].(string)
			if !ok {
				return errors.New("issue getting k.CachePrimaryQueryStored of m")
			}
			// do the insert / update actions with is just going to RPushX (note the X: don't push if key doesn't exist)
			err = s.cache.RPushX(ctx, k.getKeyName(objMap), objMap[pkField]).Err()

		default:
			err = errors.New("unknown update action")
		}

		if err != nil {
			// do not return; we want to update all the queries
			return err
		}
	}

	return nil
}

func (s *storage) cacheActionSelect(objMap map[string]interface{}, objs []map[string]interface{}, query *Query) error {
	ctx := context.Background()

	// valueToStore represents what will be put into the cache
	objsToInsert := []interface{}{}
	if query.cacheDataStructure == CacheDataStructureList {
		// get the map of the struct that's the basis for what's stored in this LRange
		m := s.queryToMap[query.CachePrimaryQueryStored]
		// get the primary key (such as "lead_id" for the lead table) from k.CachePrimaryQueryStored
		pkField, ok := m[objMapStructPrimaryKey].(string)
		if !ok {
			return errors.New("issue getting k.CachePrimaryQueryStored of m")
		}

		for _, v := range objs {
			objsToInsert = append(objsToInsert, v[pkField])
		}
	}

	var err error
	keyName := query.getKeyName(objMap)

	switch query.SelectAction {
	case CacheNoAction:
		//fmt.Println("insert action is CacheNoAction")
		// don't do anything;

	case CacheSet:
		err = s.cache.set(ctx, keyName, objMap, query.CacheTTL)

	case CacheDel:
		err = s.cache.Del(ctx, keyName).Err()

	case CacheLPush:
		if len(objsToInsert) == 0 {
			break
		}
		err = s.cache.LPush(ctx, keyName, objsToInsert...).Err()

	case CacheRPush:
		if len(objsToInsert) == 0 {
			break
		}
		err = s.cache.RPush(ctx, keyName, objsToInsert...).Err()

	default:
		err = errors.New("unknown update action")
	}
	return err
}
