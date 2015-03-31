package mqb

import (
	"bytes"
	"fmt"
	"net/http"
	"reflect"
	"testing"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type Embedded struct {
	EmbeddedBool bool
	EmbeddedInt  int64
}

type TestStruct struct {
	FloatMember       float64
	UintMember        uint
	IntMember         int64  `bson:"intMember"`
	BoolMember        bool   "mybool"
	StringMember      string `json:"name" binding:"required" validate:"nonzero"`
	EmbeddedMember    Embedded
	StringSliceMember []string `bson:"strSliceMember"`
	IntSliceMember    []int
}

func TestCreateDisableAndAddParameters(t *testing.T) {
	mq := NewMongoQuery(TestStruct{}, &mgo.Database{})
	paramsToDisable := []string{"floatmember", "mybool", "stringmember", "offset"}
	paramsToEnable := map[string]reflect.Kind{
		"test":  reflect.Bool,
		"limit": reflect.Bool,
	}
	mq.DisableParameters(paramsToDisable[0])
	mq.DisableParameters(paramsToDisable[1:3]...)
	mq.DisableParameters(paramsToDisable[3])
	mq.AddOrOverwriteValidParameter("test", reflect.Bool)
	mq.AddOrOverwriteValidParameter("limit", reflect.Bool)
	for _, p := range paramsToDisable {
		if _, ok := mq.supportedParameters[p]; ok {
			t.Errorf("disabled parameter %s in suppertedParameters", p)
		}
	}
	for k, v := range paramsToEnable {
		if _, ok := mq.supportedParameters[k]; !ok {
			t.Errorf("parameter %s not in supportedParameters", k)
			continue
		}
		value, _ := mq.supportedParameters[k]
		if value != v {
			t.Errorf("wrong value %v for parmater %s detected", v, k)
		}

	}
}

func TestCreateFieldsMap(t *testing.T) {
	mq := NewMongoQuery(TestStruct{}, &mgo.Database{})
	req, _ := http.NewRequest("GET", "/?field=mybool&field=floatmember", bytes.NewBufferString(""))
	p, err := mq.createFieldsMap(req)
	if err != nil {
		t.Errorf("error occured: %s", err)
	}
	if !reflect.DeepEqual(p, map[string]interface{}{
		"mybool":      1,
		"floatmember": 1,
	}) {
		t.Errorf("wrong pluck map generated: %v", p)
	}

	req, _ = http.NewRequest("GET", "/?field=notAMember", bytes.NewBufferString(""))
	if _, err := mq.createFieldsMap(req); err == nil {
		t.Errorf("invalid pluck paramenter did not generate an error")
	}
}

func TestCreateQueryFilter(t *testing.T) {
	mq := NewMongoQuery(TestStruct{}, &mgo.Database{})
	req, _ := http.NewRequest("GET", "/?mybool=true&intMember=2&floatmember=2.1&stringmember=foo", bytes.NewBufferString(""))
	q, err := mq.createQueryFilter(req)
	if err != nil {
		t.Errorf("error occured: %s", err)
	}
	if !reflect.DeepEqual(q, map[string]interface{}{
		"mybool":       true,
		"intMember":    2,
		"floatmember":  2.1,
		"stringmember": bson.RegEx{Pattern: "foo", Options: ""},
	}) {
		t.Errorf("wrong query filter generated: %v", q)
	}

	m := map[string]string{
		"mybool":      "notABool",
		"intMember":   "notAnInt",
		"floatmember": "notAFloat",
	}
	for k, v := range m {
		req, _ = http.NewRequest("GET", fmt.Sprintf("/?%s=%s", k, v), bytes.NewBufferString(""))
		if _, err = mq.createQueryFilter(req); err == nil {
			t.Errorf("wrong value '%s' for '%s' did not produce error", v, k)
		}
	}

	req, _ = http.NewRequest("GET", "/?myboo=true&intMember=2&floatmember=2.1&stringmember=foo", bytes.NewBufferString(""))
	_, err = mq.createQueryFilter(req)
	if err == nil {
		t.Error("unsuported parameter name did not produce error")
	}
}

func TestQueryFilterWithMultipleIdenticalParamaters(t *testing.T) {
	mq := NewMongoQuery(TestStruct{}, &mgo.Database{})
	req, _ := http.NewRequest("GET", "/?intMember=1&intMember=2&intMember=3", bytes.NewBufferString(""))
	q, err := mq.createQueryFilter(req)
	if err != nil {
		t.Errorf("error occured: %s", err)
	}
	if !reflect.DeepEqual(q, map[string]interface{}{
		"intMember": map[string]interface{}{
			"$in": []interface{}{1, 2, 3},
		},
	}) {
		t.Errorf("wrong filter map generated generated: %v", q)
	}
}

func TestFilterWithObjectIdString(t *testing.T) {
	mq := NewMongoQuery(TestStruct{}, &mgo.Database{})
	objID := "54e1b216a8f830ee6dead911"
	req, _ := http.NewRequest("GET", "/?stringmember="+objID, bytes.NewBufferString(""))
	if !bson.IsObjectIdHex(objID) {
		t.Fatalf("objectid %s is not an objectid", objID)
	}
	q, err := mq.createQueryFilter(req)
	if err != nil {
		t.Errorf("error occured: %s", err)
	}
	if !reflect.DeepEqual(q, map[string]interface{}{
		"stringmember": bson.ObjectIdHex(objID),
	}) {
		t.Errorf("wrong filter map generated generated: %v", q)
	}
}

func TestCreateSortFields(t *testing.T) {
	mq := NewMongoQuery(TestStruct{}, &mgo.Database{})
	req, _ := http.NewRequest("GET", "/?sort=mybool&sort=-intMember&sort=-floatmember&sort=stringmember", bytes.NewBufferString(""))
	s, err := mq.createSortFields(req)
	if err != nil {
		t.Errorf("error occured: %s", err)
	}
	if !reflect.DeepEqual(s, []string{"mybool", "-intMember", "-floatmember", "stringmember"}) {
		t.Errorf("wrong sort fields generated: %v", s)
	}
}
