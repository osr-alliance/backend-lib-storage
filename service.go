package storage

import (
	"encoding/json"
	"reflect"
)

// structToMap converts a struct to a map and adds the struct name as a key
func structToMap(obj interface{}) (map[string]interface{}, error) {
	// lol...
	m := map[string]interface{}{}

	j, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}

	m[objMapStructNameKey] = getStructName(obj)

	return m, json.Unmarshal(j, &m)
}

func structToMapWithOptions(obj interface{}, opts *SelectOptions) (map[string]interface{}, error) {
	objMap, err := structToMap(obj)
	if err != nil {
		return nil, err
	}

	objMap["limit"] = opts.Limit
	objMap["offset"] = opts.Offset

	return objMap, nil
}

// mapToStruct converts a map to a struct
func mapToStruct(m map[string]interface{}, s interface{}) error {
	j, err := json.Marshal(m)
	if err != nil {
		return err
	}

	return json.Unmarshal(j, s)
}

// mapsToStruct converts a map to a struct
func mapsToStruct(m []map[string]interface{}, s interface{}) error {
	j, err := json.Marshal(m)
	if err != nil {
		return err
	}

	return json.Unmarshal(j, s)
}

func getStructName(myvar interface{}) string {
	if t := reflect.TypeOf(myvar); t.Kind() == reflect.Ptr {
		return t.Elem().Name()
	} else {
		return t.Name()
	}
}
