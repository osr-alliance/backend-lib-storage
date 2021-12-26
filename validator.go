package storage

import (
	"errors"
	"strings"
)

func (q *Query) parseCacheFields() error {

	q.cacheKeyFields = []string{}

	keys := strings.Split(q.CacheKey, "|")
	if len(keys) <= 2 {
		return errors.New("invalid CacheKey; please see documentation for common format")
	}

	if !strings.HasPrefix(keys[0], "service:") {
		return errors.New("invalid CacheKey; must start with `service:` in first pipe. Please see documentation for common format")
	}

	if strings.Contains(keys[1], `%v`) {
		return errors.New("invalid CacheKey; must not contain `%v` in second pipe. Please see documentation for common format")
	}

	fields := []string{}

	for _, key := range keys[2:] {
		if !strings.Contains(key, `:%v`) {
			// field doens't have a placeholder value; continue
			continue
		}

		parts := strings.Split(key, ":")
		if len(parts) != 2 {
			return errors.New("invalid CacheKey; please see documentation for common format; a pipe can only have one colon & must be in the format `field:%v|field:%v`")
		}

		// append it
		fields = append(fields, parts[0])
	}

	q.cacheKeyFields = fields
	return nil
}

func (q *Query) parseCacheDatastructure() error {
	m := map[string]CacheDataStructure{}

	m["insert"] = CacheDataStructureStruct
	// check to see if it's a list
	switch q.InsertAction {
	case CacheLPush, CacheRPush:
		m["insert"] = CacheDataStructureList
	case CacheNoAction, CacheDel:
		delete(m, "insert")
	}

	m["update"] = CacheDataStructureStruct
	// check to see if it's a list
	switch q.UpdateAction {
	case CacheLPush, CacheRPush:
		m["update"] = CacheDataStructureList
	case CacheNoAction, CacheDel:
		delete(m, "update")
	}

	m["select"] = CacheDataStructureStruct
	// check to see if it's a list
	switch q.SelectAction {
	case CacheLPush, CacheRPush:
		m["select"] = CacheDataStructureList
	case CacheNoAction, CacheDel:
		delete(m, "select")
	}

	if len(m) == 0 {
		// everything is CacheNoAction
		return nil
	}

	c := CacheDataStructureDefault
	for _, v := range m {
		// first time through; set c to the first value
		if c == CacheDataStructureDefault {
			c = v
			continue
		}

		if c != v {
			return errors.New("all actions must be the same datastructure")
		}
	}

	q.cacheDataStructure = c // assign the cacheDataStructure a value

	return nil
}

func (s *storage) validatePrimaryQueryStored() error {

	for _, q := range s.queries {
		// we're only checking lists
		if q.cacheDataStructure != CacheDataStructureList {
			continue
		}

		if q.CachePrimaryQueryStored == 0 {
			return errors.New("CachePrimaryQueryStored must be set for lists in " + q.CacheKey)
		}

		// check to see if the primary query is the primary key of a table
		pkStored := q.CachePrimaryQueryStored

		t, ok := s.queryToTable[pkStored]
		if !ok || t.PrimaryQueryName != pkStored {
			return errors.New("CachePrimaryQueryStored must be the primary query of a table in " + q.CacheKey)
		}
	}
	return nil
}
