package mqb

import (
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
)

var validMetaParameters = map[string]reflect.Kind{
	"page":  reflect.Uint,
	"limit": reflect.Uint,
	"field": reflect.String,
	"sort":  reflect.String,
}

var mongoTags = []string{
	"omitempty",
	"minsize",
	"inline",
}

// createValidParametersMap creates a map of valid query parameters where the keys represent
// valid field names in a collection, represented by endpointStruct and the values represent the
// corresponding type.
// If a fieldname is in the disabledParameters, then that fieldname will
// not be added to the map.
func createValidParametersMap(endPointStruct interface{}, disabledParameters ...string) map[string]reflect.Kind {
	validParametersMap := make(map[string]reflect.Kind)
	typ := reflect.TypeOf(endPointStruct)
	val := reflect.ValueOf(endPointStruct)
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
		val = val.Elem()
	}
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		fieldName := getFieldNameFromTag(field.Tag)
		if len(fieldName) == 0 {
			// mgo driver converts field names to lower case
			fieldName = strings.ToLower(field.Name)
		}
		if field.Type.Kind() == reflect.Struct {
			for k, v := range createValidParametersMap(val.Field(i).Interface(), disabledParameters...) {
				validParametersMap[k] = v
			}
			continue
		}
		if field.Type.Kind() == reflect.Slice && !contains(disabledParameters, fieldName) {
			validParametersMap[fieldName] = field.Type.Elem().Kind()
			continue
		}
		if !contains(disabledParameters, fieldName) {
			validParametersMap[fieldName] = field.Type.Kind()
		}
	}

	for k, v := range validMetaParameters {
		if !contains(disabledParameters, k) {
			validParametersMap[k] = v
		}
	}

	return validParametersMap
}

// getFieldNameFromTag returns the field name if it is overridden by a tag, otherwise it returns
// an empty string.
func getFieldNameFromTag(tag reflect.StructTag) string {
	fieldName := tag.Get("bson")
	if len(fieldName) > 1 {
		diff := newSetFromSlice(strings.Split(fieldName, ",")).Difference(newSetFromSlice(mongoTags))
		if len(diff.ToSlice()) > 0 {
			return diff.ToSlice()[0].(string)
		}
	}
	if strings.Contains(string(tag), ":") {
		// we have only other than bson keys present
		return ""
	}
	// we have a tag of the form "membername,omitempty" wich is supported by mgo
	diff := newSetFromSlice(strings.Split(string(tag), ",")).Difference(newSetFromSlice(mongoTags))
	if len(diff.ToSlice()) > 0 {
		return diff.ToSlice()[0].(string)
	}
	return ""

}

// getUint tries to convert the value of param to an uint and an error
// is returned if it fails. If param is not present the bool value is false
func getUint(req *http.Request, param string) (uint, bool, error) {
	if _uintVal, ok := req.URL.Query()[param]; ok {
		uintVal, err := strconv.ParseUint(_uintVal[0], 10, 0)
		if err != nil {
			return 0, true, fmt.Errorf("invalid value for %s", _uintVal[0])
		}
		return uint(uintVal), true, nil
	}
	return 0, false, nil
}
