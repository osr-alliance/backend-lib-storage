package storage

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

func (t *Table) validate() error {
	if t.Struct == nil {
		return fmt.Errorf("Struct must be set")
	}

	t.parseTableName()

	// you can have no primary key only if you have no insert query
	if t.PrimaryKeyField == "" && t.InsertQuery != "" {
		return fmt.Errorf("Table: %s Err: PrimaryKeyField must be set", t.tableName)
	}

	if t.PrimaryQueryName == "" {
		return fmt.Errorf("Table: %s Err: PrimaryQueryName must be set", t.tableName)
	}

	if len(t.Queries) == 0 {
		return fmt.Errorf("Table: %s Err: Queries must be set", t.tableName)
	}

	err := t.validateInsertAndUpdateQueries()
	if err != nil {
		return err
	}

	err = t.validateAndParseObjMap()
	if err != nil {
		return err
	}

	return t.parseSlicesInQueries()
}

func (t *Table) parse() {
	t.parseTableName()
}

func (t *Table) validateAndParseObjMap() error {
	objMap, err := structToMap(t.Struct)
	if err != nil {
		return fmt.Errorf("error getting struct map for %s: %s", t.tableName, err)
	}

	objMap[objMapStructPrimaryKey] = t.PrimaryKeyField

	t.objMap = objMap
	return nil
}

func (t *Table) validateInsertAndUpdateQueries() error {
	// insert query & update query shouldn't be required e.g. UserIsAuthorized doens't have an insert or update

	if !strings.HasSuffix(strings.ToLower(t.InsertQuery), "returning *") && t.InsertQuery != "" {
		return errors.New("InsertQuery must end with `returning *`")
	}

	if !strings.HasSuffix(strings.ToLower(t.UpdateQuery), "returning *") && t.UpdateQuery != "" {
		return errors.New("UpdateQuery must end with `returning *`")
	}
	return nil
}

func (t *Table) parseTableName() {
	// optimization but this is used so many times that it's worth it given it uses reflection
	t.tableName = getStructName(t.Struct)
}

// parseSlicesInSqlQuery takes in a query and parses out the slices in the query for use in the IN clause in postgres
func (t *Table) parseSlicesInQueries() error {
	if t.objMap == nil {
		return errors.New("storage: objMap must not be nil before parsing slices in queries")
	}

	val := reflect.Indirect(reflect.ValueOf(t.Struct))
	// get all the slices in the objMap
	slices := map[string]reflect.Type{}

	for i := 0; i < val.Type().NumField(); i++ {
		field := val.Type().Field(i)

		if field.Type.Kind() == reflect.Slice {
			s := strings.Split(field.Tag.Get("json"), ",") // in case there are options like omitempty
			slices[s[0]] = field.Type
		}
	}

	slicesUsed := map[string]reflect.Type{}
	// check if any of the queries have a named parameter in the query that's also in the slices
	for _, q := range t.Queries {
		// get all the named parameter slices
		for slice := range slices {

			// should probably be a little bit better but it's assuming it's a slice so it's ({parameter})
			if strings.Contains(q.Query, fmt.Sprintf("(:%s)", slice)) {
				// must have no action on the cache when using IN in a query because you'll get cache invalidation errors
				if q.InsertAction != CacheNoAction || q.UpdateAction != CacheNoAction || q.SelectAction != CacheNoAction {
					return fmt.Errorf("storage: %s must use CacheNoAction when using IN in your query", q.Name)
				}

				if q.slicesInQuery == nil {
					q.slicesInQuery = map[string]reflect.Type{}
				}
				q.slicesInQuery[slice] = slices[slice]
				slicesUsed[slice] = slices[slice]
			}
		}
	}

	// validate the slices used in the queries
	for slice, k := range slicesUsed {
		// check to make sure they're of the correct type
		// check to make sure the slice is of the proper type
		switch k.String() {
		case "[]string":
		case "[]int":
		case "[]int32":
		case "[]int64":
		case "[]float32":
		case "[]float64":
		default:
			return fmt.Errorf("cannot have slice %s of type %s in a query", slice, k.String())
		}
	}

	return nil
}
