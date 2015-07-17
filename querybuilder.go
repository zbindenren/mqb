package mqb

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/ansel1/merry"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

var (
	DefaultPageSize uint = 20 // DefaultPageSize defines how many elements a page contains per default.
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
	Content interface{} `json:"content,omitempty"`
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
	q := mq.dataBase.C(structName(mq.endPointStruct)).Find(filterMap)

	selectFields, err := mq.createFieldsMap(req)
	q.Select(selectFields)

	sortFields, err := mq.createSortFields(req)
	if err != nil {
		return nil, err
	}
	q.Sort(sortFields...)

	mq.page.Size, err = getUint(req, "limit", DefaultPageSize)
	if err != nil {
		return nil, merry.Wrap(err).WithHTTPCode(http.StatusBadRequest)
	}
	mq.page.Current, err = getUint(req, "page", 1)
	if err != nil {
		return nil, merry.Wrap(err).WithHTTPCode(http.StatusBadRequest)
	}
	if mq.page.Current == 0 {
		return nil, merry.Wrap(errors.New("page cannot be 0")).WithHTTPCode(http.StatusBadRequest)
	}
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
		return nil, merry.Wrap(err).WithHTTPCode(http.StatusBadRequest)
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
		return nil, merry.Wrap(err).WithHTTPCode(http.StatusInternalServerError)
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

	for parameterName, parameterValues := range req.URL.Query() {
		s := []interface{}{}
		if kind, ok := mq.supportedParameters[parameterName]; ok {
			// meta parameters are not filters
			if _, ok := validMetaParameters[parameterName]; ok {
				continue
			}
			switch kind {
			case reflect.Bool:
				for _, v := range parameterValues {
					b, err := strconv.ParseBool(v)
					if err != nil {
						return nil, merry.Wrap(err).WithHTTPCode(http.StatusBadRequest)
					}
					s = append(s, b)
				}
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				for _, v := range parameterValues {
					i, err := strconv.Atoi(v)
					if err != nil {
						return nil, merry.Wrap(err).WithHTTPCode(http.StatusBadRequest)
					}
					s = append(s, i)
				}
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				for _, v := range parameterValues {
					i, err := strconv.ParseUint(v, 10, 0)
					if err != nil {
						return nil, merry.Wrap(err).WithHTTPCode(http.StatusBadRequest)
					}
					s = append(s, uint(i))
				}
			case reflect.Float32, reflect.Float64:
				for _, v := range parameterValues {
					f, err := strconv.ParseFloat(v, 64)
					if err != nil {
						return nil, merry.Wrap(err).WithHTTPCode(http.StatusBadRequest)
					}
					s = append(s, f)
				}
			case reflect.String:
				if len(parameterValues) == 1 {
					if bson.IsObjectIdHex(parameterValues[0]) {
						s = []interface{}{bson.ObjectIdHex(parameterValues[0])}
					} else {
						s = []interface{}{bson.RegEx{Pattern: parameterValues[0], Options: ""}}
					}
				} else {
					for _, v := range parameterValues {
						if bson.IsObjectIdHex(v) {
							s = append(s, bson.ObjectIdHex(v))
						} else {
							s = append(s, v)
						}
					}
				}
			default:
				return nil, merry.Wrap(fmt.Errorf("reflection kind '%s' is not supported", kind)).WithHTTPCode(http.StatusBadRequest)
			}
		} else {
			return nil, merry.Wrap(fmt.Errorf("parameter '%s' is not supported", parameterName)).WithHTTPCode(http.StatusBadRequest)
		}
		if len(s) == 1 {
			filter[parameterName] = s[0]
		} else {
			filter[parameterName] = map[string]interface{}{
				"$in": s,
			}
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
				return nil, merry.Wrap(fmt.Errorf("unsupported field value: %s", v)).WithHTTPCode(http.StatusBadRequest)
			}
			sortFields = append(sortFields, v)
		}
	}
	return sortFields, nil
}

func (p *Page) calculateLastPage() {
	p.Last = uint(math.Ceil(float64(p.Items) / float64(p.Size)))
}
