// Package mqb creates mgo queries from HTTP requests.
//
// Let's say you have a collection represented by the following type:
//     type Person struct {
//        Name string
//        Age  int
//     }
//
// then this package creates from a http request with parameters like:
//
//     /?name=peter&age=10&field=name&limit=10&offset=2&sort=-name&sort=age
//
// a mgo query like:
//
//     s := session.DB("dbname")
//     q := s.C("people").Find(bson.M{"name": bson.RegExp{Pattern: "peter", Options: ""}, "age": 10}).Select(bson.M{"name": 1}).Limit(10).Skip(2).Sort("-name", "age")
//
package mqb
