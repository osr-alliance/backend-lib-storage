package storage

import (
	"errors"
	"fmt"
	"reflect"
)

func getStructName(myvar interface{}) string {
	if t := reflect.TypeOf(myvar); t.Kind() == reflect.Ptr {
		return t.Elem().Name()
	} else {
		return t.Name()
	}
}

func getFieldValueByName(name string, s interface{}) interface{} {
	v := reflect.ValueOf(s)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	f := v.FieldByName(name)
	if !f.IsValid() {
		return nil
	}
	return f.Interface()
}

func scanToDest(rows []interface{}, dest interface{}) error {
	value := reflect.ValueOf(dest)

	// need dest to be a pointer to a slice
	if value.Kind() != reflect.Ptr {
		return errors.New("dest must be a pointer to a slice")
	}
	if value.IsNil() {
		return errors.New("dest cannot be a nil pointer")
	}

	slice := getValue(value.Type())
	if slice.Kind() != reflect.Slice {
		return fmt.Errorf("expected slice but got %s", slice.Kind())
	}

	direct := reflect.Indirect(value)
	isPointer := (slice.Elem().Kind() == reflect.Ptr)

	for _, row := range rows {
		// append
		if isPointer {
			direct.Set(reflect.Append(direct, reflect.ValueOf(row)))
		} else {
			direct.Set(reflect.Append(direct, reflect.Indirect(reflect.ValueOf(row))))
		}
	}

	return nil
}

// getValue
func getValue(t reflect.Type) reflect.Type {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t
}
