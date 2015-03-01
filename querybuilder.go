package mqb

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

var (
	DefaultPageSize uint = 2 // DefaultPageSize defines how many elements a page contains per default.
)

// Page the paging information.
type Page struct {
	Size    uint `json:"size"`    // Size defines how many elements a page contains.
	Items   uint `json:"items"`   // Items defines the total number of items the corresponding query returns.
	Last    uint `json:"last"`    // Last represents total number of pages a query generates (depends on the page size and the total number of elements returned by the query).
	Current uint `json:"current"` // Current is the current page nuber for the query.
}

// Response contains the result of the query, including the Page information.
type Response struct {
	Content interface{} `json:"content"`
	Page    Page        `json:"page"`
}

// MongoQuery can be used to to create mgo.Query from http request parameters.
type MongoQuery struct {
	endPointStruct               interface{}
	dataBase                     *mgo.Database
	supportedParameters          map[string]reflect.Kind
	additionalSupportedParamters map[string]reflect.Kind
	disabledParameters           []string
	page                         Page
}

// NewMongoQuery returns a new MongoQuery.
func NewMongoQuery(endPointStruct interface{}, database *mgo.Database) *MongoQuery {
	return &MongoQuery{
		dataBase:                     database,
		supportedParameters:          createValidParametersMap(endPointStruct),
		disabledParameters:           []string{},
		additionalSupportedParamters: make(map[string]reflect.Kind),
		endPointStruct:               endPointStruct,
	}
}

// CreateQuery creates a mgo.Query from a HTTP Request for a collection represented by endpointStruct.
//
// Examples:
//     mq := NewMongoQuery(People{}, db)
//     q, _ := mq.CreateQuery(req) // creates a query from the request for the people collection
//
//     mq.DisableParameters("name", "sort")
//     q, _ := mq.CreateQuery(req) // creates a query from the request for the people collection with the parameters "name" and "sort" disabled.
//
func (mq *MongoQuery) CreateQuery(req *http.Request) (*mgo.Query, error) {
	filterMap, err := mq.createQueryFilter(req)
	if err != nil {
		return nil, err
	}
	sortFields, err := mq.createSortFields(req)
	if err != nil {
		return nil, err
	}
	mq.page.Size, err = getUint(req, "limit", DefaultPageSize)
	if err != nil {
		return nil, err
	}
	mq.page.Current, err = getUint(req, "page", 1)
	if err != nil {
		return nil, err
	}
	if mq.page.Current == 0 {
		return nil, errors.New("page cannot be 0")
	}
	q := mq.dataBase.C(structName(mq.endPointStruct)).Find(filterMap)
	q.Sort(sortFields...)
	q = q.Limit(int(mq.page.Size))
	q = q.Skip(int((mq.page.Current - 1) * mq.page.Size))
	return q, nil
}

// Run runs the query on the database and returns a *Response.
func (mq *MongoQuery) Run(req *http.Request) (*Response, error) {
	q, err := mq.CreateQuery(req)
	if err != nil {
		return nil, err
	}

	// copy query and reset limit and skip values to count total items
	// that would be returned for a query
	countQuery := &mgo.Query{}
	*countQuery = *q
	countQuery.Limit(0)
	countQuery.Skip(0)
	items, err := countQuery.Count()
	if err != nil {
		return nil, err
	}

	response := &Response{
		Page: mq.page,
	}
	response.Page.Items = uint(items)
	response.Page.calculateLastPage()

	// create a pointer to an empty slice with same type as enpointStruct to store the
	// result of the query
	slice := reflect.MakeSlice(reflect.SliceOf(reflect.TypeOf(mq.endPointStruct)), 0, 0)
	content := reflect.New(slice.Type()).Interface()
	err = q.All(content)
	if err != nil {
		return nil, err
	}
	response.Content = content
	return response, nil
}

// DisableParameters disables paramters. If a URL query contains any
// of those paramters, an error is returned.
func (mq *MongoQuery) DisableParameters(paramters ...string) {
	for _, p := range paramters {
		if !contains(mq.disabledParameters, p) {
			mq.disabledParameters = append(mq.disabledParameters, p)
		}
	}
	mq.supportedParameters = createValidParametersMap(mq.endPointStruct, mq.disabledParameters...)
	for k, v := range mq.additionalSupportedParamters {
		mq.supportedParameters[k] = v
	}
}

// AddOrOverwriteValidParameter adds or overwrites a valid parmeter with name and reflect.Kind.
func (mq *MongoQuery) AddOrOverwriteValidParameter(name string, value reflect.Kind) {
	mq.additionalSupportedParamters[name] = value
	for k, v := range mq.additionalSupportedParamters {
		mq.supportedParameters[k] = v
	}
}

func (mq *MongoQuery) createQueryFilter(req *http.Request) (map[string]interface{}, error) {
	filter := make(map[string]interface{})

	for parameterName, parameterValue := range req.URL.Query() {
		if kind, ok := mq.supportedParameters[parameterName]; ok {
			// meta parameters are not filters
			if _, ok := validMetaParameters[parameterName]; ok {
				continue
			}
			switch kind {
			case reflect.Bool:
				b, err := strconv.ParseBool(parameterValue[0])
				if err != nil {
					return nil, err
				}
				filter[parameterName] = b
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				i, err := strconv.Atoi(parameterValue[0])
				if err != nil {
					return nil, err
				}
				filter[parameterName] = i
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				i, err := strconv.ParseUint(parameterValue[0], 10, 0)
				if err != nil {
					return nil, err
				}
				filter[parameterName] = i
			case reflect.Float32, reflect.Float64:
				f, err := strconv.ParseFloat(parameterValue[0], 64)
				if err != nil {
					return nil, err
				}
				filter[parameterName] = f
			case reflect.String:
				filter[parameterName] = bson.RegEx{Pattern: parameterValue[0], Options: ""}
			default:
				return nil, fmt.Errorf("reflection kind '%s' is not supported", kind)
			}
		} else {
			return nil, fmt.Errorf("parameter '%s' is not supported", parameterName)
		}
	}
	return filter, nil
}

func (mq *MongoQuery) createFieldsMap(req *http.Request) (map[string]interface{}, error) {
	fields := make(map[string]interface{})
	if _field, ok := req.URL.Query()["field"]; ok {
		for _, v := range _field {
			if _, ok := mq.supportedParameters[v]; !ok {
				return nil, fmt.Errorf("unsupported field value: %s", v)
			}
			fields[v] = 1
		}
	}
	return fields, nil
}

func (mq *MongoQuery) createSortFields(req *http.Request) ([]string, error) {
	sortFields := []string{}
	if _sortField, ok := req.URL.Query()["sort"]; ok {
		for _, v := range _sortField {
			if _, ok := mq.supportedParameters[strings.Trim(v, "-")]; !ok {
				return nil, fmt.Errorf("unsupported field value: %s", v)
			}
			sortFields = append(sortFields, v)
		}
	}
	return sortFields, nil
}

func (p *Page) calculateLastPage() {
	p.Last = uint(math.Ceil(float64(p.Items) / float64(p.Size)))
}
