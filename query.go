package graphql

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/dbmedialab/go-graphql-client/ident"
)

func constructQuery(v interface{}, variables map[string]interface{}) string {
	query := GenerateQueryFields(v)
	if variables != nil {
		return "query(" + queryArguments(variables) + ")" + query
	}
	return query
}

func constructMutation(v interface{}, variables map[string]interface{}) string {
	query := GenerateQueryFields(v)
	if variables != nil {
		return "mutation(" + queryArguments(variables) + ")" + query
	}
	return "mutation" + query
}

// queryArguments constructs a minified arguments string for variables.
//
// E.g., map[string]interface{}{"a": Int(123), "b": NewBoolean(true)} -> "$a:Int!$b:Boolean".
func queryArguments(variables map[string]interface{}) string {
	// Sort keys in order to produce deterministic output for testing purposes.
	// TODO: If tests can be made to work with non-deterministic output, then no need to sort.
	keys := make([]string, 0, len(variables))
	for k := range variables {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf bytes.Buffer
	for _, k := range keys {
		io.WriteString(&buf, "$")
		io.WriteString(&buf, k)
		io.WriteString(&buf, ":")
		writeArgumentType(&buf, reflect.TypeOf(variables[k]), true)
		// Don't insert a comma here.
		// Commas in GraphQL are insignificant, and we want minified output.
		// See https://facebook.github.io/graphql/October2016/#sec-Insignificant-Commas.
	}
	return buf.String()
}

// writeArgumentType writes a minified GraphQL type for t to w.
// value indicates whether t is a value (required) type or pointer (optional) type.
// If value is true, then "!" is written at the end of t.
func writeArgumentType(w io.Writer, t reflect.Type, value bool) {
	if t.Kind() == reflect.Ptr {
		// Pointer is an optional type, so no "!" at the end of the pointer's underlying type.
		writeArgumentType(w, t.Elem(), false)
		return
	}

	switch t.Kind() {
	case reflect.Slice, reflect.Array:
		// List. E.g., "[Int]".
		io.WriteString(w, "[")
		writeArgumentType(w, t.Elem(), true)
		io.WriteString(w, "]")
	default:
		// Named type. E.g., "Int".
		name := t.Name()
		if name == "string" { // HACK: Workaround for https://github.com/shurcooL/githubql/issues/12.
			name = "ID"
		}
		io.WriteString(w, name)
	}

	if value {
		// Value is a required type, so add "!" to the end.
		io.WriteString(w, "!")
	}
}

// GenerateQueryFields construct a minified graphql string type definition query string
// from the provided struct v.
//
// E.g., struct{Foo Int, BarBaz *Boolean} -> "{foo,barBaz}".
//
// You can concatenate snippets to produce custom queries, while still successfully
// avoiding most of the repetition of describing your Go type in graphql terms.
// To make a full query, you also need to prefix a variables section; the QueryCustom
// method will do this for you.
// Arguments, Aliases, and Fragments can also all be prepended to a Fields snippet;
// see http://graphql.org/learn/queries/
// for more description of each of these concepts.
func GenerateQueryFields(v interface{}) string {
	var buf bytes.Buffer
	writeQuery(&buf, reflect.TypeOf(v), map[edge]int{}, []string{}, false)
	return buf.String()
}

// edge is simply a tuple to key the visitation map that we use to keep
// writeQuery from recursing without bound on recursive types.
type edge struct {
	t  reflect.Type
	fn int
}

// writeQuery writes a minified query for t to w.
// If inline is true, the struct fields of t are inlined into parent struct.
func writeQuery(w io.Writer, t reflect.Type, visited map[edge]int, visitPath []string, inline bool) {
	switch t.Kind() {
	case reflect.Ptr, reflect.Slice:
		writeQuery(w, t.Elem(), visited, visitPath, false)
	case reflect.Struct:
		// If the type implements json.Unmarshaler, it's a scalar. Don't expand it.
		if reflect.PtrTo(t).Implements(jsonUnmarshaler) {
			return
		}
		if !inline {
			io.WriteString(w, "{")
		}
		for i := 0; i < t.NumField(); i++ {
			if i != 0 {
				io.WriteString(w, ",")
			}
			f := t.Field(i)

			// Check how many times we've traversed this before (recursion limit).
			edge := edge{t, i}
			visited[edge]++
			switch limit := getRecursionLimit(f); {
			case limit < 2:
				// if not recursion limit configured: cycle is error.
				if visited[edge] > 1 {
					visitPath = append(visitPath, t.Name())
					panic(fmt.Errorf("cycle found: %s", strings.Join(visitPath, "->")))
				}
			default:
				// if recursion limit configured: if we're under, that's fine; if over, skip.
				if visited[edge] > limit {
					continue
				}
			}

			value, ok := f.Tag.Lookup("graphql")
			inlineField := f.Anonymous && !ok
			if !inlineField {
				if ok {
					io.WriteString(w, value)
				} else {
					io.WriteString(w, ident.ParseMixedCaps(f.Name).ToLowerCamelCase())
				}
			}
			visitPath = append(visitPath, t.String()+"."+f.Name)
			writeQuery(w, f.Type, visited, visitPath, inlineField)
			visitPath = visitPath[:len(visitPath)-1]
			visited[edge]--
		}
		if !inline {
			io.WriteString(w, "}")
		}
	}
}

func getRecursionLimit(f reflect.StructField) int {
	value, ok := f.Tag.Lookup("graphql-recurse")
	if !ok {
		return 1
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		panic(fmt.Errorf("graphql-recurse tag should be int: %s", err))
	}
	if n < 2 {
		panic(fmt.Errorf("graphql-recurse tag only makes sense for values greater than 1"))
	}
	return n
}

var jsonUnmarshaler = reflect.TypeOf((*json.Unmarshaler)(nil)).Elem()
