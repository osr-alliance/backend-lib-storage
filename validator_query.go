package storage

import (
	"errors"
	"fmt"
	"strings"
)

func (q *Query) validate() error {
	err := q.validateName()
	if err != nil {
		return err
	}

	err = q.validateAndParseCacheFields()
	if err != nil {
		return err
	}

	return q.validateAndParseCacheDataStructure()
}

func (q *Query) parseCacheListKey() {
	if q.cacheDataStructure != CacheDataStructureList {
		return
	}

	q.cacheListKey = q.CacheKey + cacheKeyCachedSelectAllModifier

	q.cacheListCachedSelectAllKey = q.CacheKey + cacheKeyListCachedSelectAll
}

func (q *Query) parseTableName(tableName string) {
	q.tableName = tableName
}

func (q *Query) parseFullCacheKey(service string, tableName string) {
	// this is an optimization so we don't need to sprintf extra keys and do the lookup
	// small but this is used so many times that it's worth it
	q.fullCacheKey = fmt.Sprintf("service:%s|%s", service, tableName)
}

// validateAndParseCacheDataStructure parses the Insert, Select, and Update actions and sets the cacheDataStructure based off of the actions
func (q *Query) validateAndParseCacheDataStructure() error {
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

// validateAndParseCacheFields takes in a generic key e.g. `lead_id=%v` and places the lead_id into the cacheKeyFields
func (q *Query) validateAndParseCacheFields() error {

	q.cacheKeyFields = []cacheKeyField{}

	keys := strings.Split(q.CacheKey, "|")

	fields := []cacheKeyField{}

	for _, key := range keys {

		if strings.Contains(key, `!=%v`) {
			return fmt.Errorf("CacheKey %s with `!=` operator must not end with `%v` but the value to not match agains", q.CacheKey)
		}

		if !strings.Contains(key, `=%v`) && !strings.Contains(key, `!=`) {
			// field doens't have a placeholder value; continue
			continue
		}

		field := cacheKeyField{}

		parts := strings.Split(key, "!=")
		if len(parts) == 2 {
			field.columnName = parts[0]
			field.operator = operatorNotEqual
			field.value = parts[1]

			// append it
			fields = append(fields, field)

			// set the operator
			q.cacheKeyContainsNotEqualsOperator = true

			continue
		}

		parts = strings.Split(key, "=")
		if len(parts) == 2 {
			field.columnName = parts[0]
			field.operator = operatorEqual

			// append it
			fields = append(fields, field)
			continue
		}
	}

	q.cacheKeyFields = fields

	return nil
}

func (q *Query) validateName() error {
	if q.Name == "" {
		return errors.New("name is required")
	}
	return nil
}

func (q *Query) parseTTL(defaultTTL int) {
	if q.CacheTTL == 0 {
		q.CacheTTL = defaultTTL
	}
}

func (q *Query) parseLimitOffsetQuery() {
	q.queryLimitOffset = q.Query + " LIMIT :limit OFFSET :offset"
}
