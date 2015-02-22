package mqb

import (
	"reflect"
	"strings"

	"github.com/deckarep/golang-set"
)

func contains(list []string, value string) bool {
	for _, v := range list {
		if v == value {
			return true
		}
	}
	return false
}

func newSetFromSlice(slice []string) mapset.Set {
	s := mapset.NewSet()
	for _, e := range slice {
		s.Add(e)
	}
	return s
}

func structName(structObj interface{}) string {
	typ := reflect.TypeOf(structObj)
	val := reflect.ValueOf(structObj)
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
		val = val.Elem()
	}
	return strings.ToLower(typ.Name())
}
