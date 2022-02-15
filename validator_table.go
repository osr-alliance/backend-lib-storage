package storage

import (
	"errors"
	"fmt"
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

	return t.validateAndParseObjMap()
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
