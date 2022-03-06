package storage

import (
	"errors"
	"fmt"
)

func (s *storage) validate() error {
	if s.serviceName == "" {
		return errors.New("serviceName must be set")
	}

	err := s.validatePrimaryQueryStored()
	if err != nil {
		return err
	}

	return s.validateQueries()
}

func (s *storage) parse() {

}

// validatePrimaryQueryStored makes sure that the key in CachePrimaryQueryStored is actually a query that queries based off primary key
func (s *storage) validatePrimaryQueryStored() error {

	for _, q := range s.queries {
		// we're only checking lists
		if q.cacheDataStructure != CacheDataStructureList {
			continue
		}

		if q.CachePrimaryQueryStored == "" {
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

func (s *storage) validateQueries() error {

	for _, q := range s.queries {
		if q.InsertAction == CacheNoAction && q.UpdateAction == CacheNoAction && q.SelectAction == CacheNoAction {
			continue
		}

		m := s.queryToMap[q.Name]
		m["limit"] = 0
		m["offset"] = 0

		explainQuery := fmt.Sprintf("EXPLAIN %s", q.queryLimitOffset)

		_, err := s.db.readConnection.NamedQuery(explainQuery, m)
		if err != nil {
			return fmt.Errorf("error in query: %s. Query: %s", err.Error(), q.queryLimitOffset)
		}

	}

	return nil
}
