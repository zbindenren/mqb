package mqb

import (
	"bytes"
	"net/http"
	"reflect"
	"testing"
)

func TestCreateValidParametersMap(t *testing.T) {
	params := map[string]reflect.Kind{
		"floatmember":    reflect.Float64,
		"uintmember":     reflect.Uint,
		"intMember":      reflect.Int64,
		"mybool":         reflect.Bool,
		"stringmember":   reflect.String,
		"field":          reflect.String,
		"embeddedbool":   reflect.Bool,
		"embeddedint":    reflect.Int64,
		"strSliceMember": reflect.String,
		"intslicemember": reflect.Int,
	}

	keys := []string{}
	m := createValidParametersMap(TestStruct{})
	for k, v := range params {
		keys = append(keys, k)
		if params[k] != m[k] {
			t.Errorf("parameter %s map should should be %s", k, v)
		}
	}

	m = createValidParametersMap(TestStruct{}, keys...)
	for _, p := range keys {
		if _, ok := m[p]; ok {
			t.Errorf("parameter map should not contain %s", p)
		}
	}
}

func TestGetMemberNameFromTag(t *testing.T) {
	tags := map[string]string{
		`bson:"membername,omitempty"`:  "membername",
		`bson:",omitempty"`:            "",
		"membername,omitempty,minsize": "membername",
		"membername":                   "membername",
		",minsize":                     "",
		`json:"name" binding:"required" validate:"nonzero"`: "",
	}

	for tag, name := range tags {
		m := getFieldNameFromTag(reflect.StructTag(tag))
		if m != name {
			t.Errorf("wrong membername detected: '%s'", m)
		}
	}
}

func TestGetUInt(t *testing.T) {
	req, _ := http.NewRequest("GET", "/?limit=11&page=11", bytes.NewBufferString(""))
	for _, param := range []string{"limit", "page"} {
		v, ok, err := getUint(req, param)
		if err != nil {
			t.Fatalf("error occured: %s, but should not", err)
		}
		if !ok {
			t.Error("ok value should be true")
		}
		if v != 11 {
			t.Errorf("value is %d, but should be 10", v)
		}
	}

	req, _ = http.NewRequest("GET", "/?foo=10", bytes.NewBufferString(""))
	_, ok, err := getUint(req, "bar")
	if err != nil {
		t.Fatalf("error occured: %s", err)
	}
	if ok {
		t.Error("ok value should be false")
	}
}
