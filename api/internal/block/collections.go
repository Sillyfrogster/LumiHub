package block

import "reflect"

// withEmptyCollections keeps a collection in stored content a list, because a slice nobody filled in would otherwise be written as null.
func withEmptyCollections(value any) any {
	if value == nil {
		return nil
	}
	return collectionsFilled(reflect.ValueOf(value)).Interface()
}

func collectionsFilled(value reflect.Value) reflect.Value {
	switch value.Kind() {
	case reflect.Pointer, reflect.Interface:
		if value.IsNil() {
			return value
		}
		replaced := collectionsFilled(value.Elem())
		if value.Kind() == reflect.Interface {
			return replaced
		}
		held := reflect.New(replaced.Type())
		held.Elem().Set(replaced)
		return held
	case reflect.Slice:
		if value.Type().Elem().Kind() == reflect.Uint8 {
			return value
		}
		if value.IsNil() {
			return reflect.MakeSlice(value.Type(), 0, 0)
		}
		replaced := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for i := range value.Len() {
			replaced.Index(i).Set(collectionsFilled(value.Index(i)))
		}
		return replaced
	case reflect.Map:
		if value.IsNil() {
			return reflect.MakeMap(value.Type())
		}
		replaced := reflect.MakeMapWithSize(value.Type(), value.Len())
		iterator := value.MapRange()
		for iterator.Next() {
			replaced.SetMapIndex(iterator.Key(), collectionsFilled(iterator.Value()))
		}
		return replaced
	case reflect.Struct:
		replaced := reflect.New(value.Type()).Elem()
		replaced.Set(value)
		for i := range value.NumField() {
			if !replaced.Field(i).CanSet() {
				continue
			}
			replaced.Field(i).Set(collectionsFilled(value.Field(i)))
		}
		return replaced
	default:
		return value
	}
}
