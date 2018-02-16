package mapper

import (
	"errors"
	"reflect"
	"strings"
)

const (
	tagMapper = "mapper"
	keyName   = "name"
	keyTarget = "target"
)

type mapper struct {
	mapping map[string]string
	to      reflect.Value
	from    reflect.Value
}

// ByField assigns the value of fields with "mapper" tag from one structs to
// another struct. It matches the "name" key on both structs and assign the
// values if assignable to another.
func ByNameKey(from interface{}, to interface{}) error {
	var t, f reflect.Value
	var ok bool
	if ok, f = isStruct(from); !ok {
		return errors.New(`mapper.ByNameKey: "from" is not a struct`)
	}
	if ok, t = isStruct(to); !ok {
		return errors.New(`mapper.ByNameKey: "to" is not a struct`)
	}
	n := &mapper{
		mapping: make(map[string]string),
		to:      t,
		from:    f,
	}
	n.convert()
	return nil
}

// ByField assigns the value of fields with "mapper" tag from one structs to
// another struct. It looks for a field with name matching target key in the
// source struct and assign the values if assignable to another.
func ByField(from interface{}, to interface{}) error {
	var t, f reflect.Value
	var ok bool
	if ok, f = isStruct(from); !ok {
		return errors.New(`mapper.ByField: "from" is not a struct`)
	}
	if ok, t = isStruct(to); !ok {
		return errors.New(`mapper.ByField: "to" is not a struct`)
	}
	n := &mapper{
		mapping: make(map[string]string),
		to:      t,
		from:    f,
	}
	n.assign(true)
	return nil
}

func (n *mapper) convert() {
	n.mapTags(n.to)
	n.assign(false)
}

func (n *mapper) mapTags(v reflect.Value) {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		name := field.Name
		tag := strings.TrimSpace(field.Tag.Get(tagMapper))
		if tag == "-" {
			continue
		}
		if tag != "" {
			tagMap := parseTags(tag)
			if v, ok := tagMap[keyName]; ok {
				n.mapping[v] = name
			}
		}
	}
}

func (n *mapper) assign(useFieldName bool) {
	t := n.from.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fName := field.Name
		tag := strings.TrimSpace(field.Tag.Get(tagMapper))
		if tag == "-" {
			continue
		}
		if tag != "" {
			tagMap := parseTags(tag)
			selector := keyName
			if useFieldName {
				selector = keyTarget
			}
			if v, ok := tagMap[selector]; ok {
				if n.from.FieldByName(fName).CanInterface() {
					iValue := n.from.FieldByName(fName)
					setterName := ""
					if useFieldName {
						setterName = v
					} else {
						setterName = n.mapping[v]
					}
					if n.to.FieldByName(setterName).CanSet() {
						n.to.FieldByName(setterName).Set(iValue)
					}
				}

			}
		}
	}
}

func isStruct(ss interface{}) (bool, reflect.Value) {
	v := reflect.ValueOf(ss)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	// uninitialized zero value of a struct
	if v.Kind() == reflect.Invalid {
		return false, v
	}

	if v.Kind() == reflect.Struct {
		return true, v
	}
	return false, v
}

func parseTags(tag string) map[string]string {
	tagList := strings.Split(tag, ",")
	tagMap := make(map[string]string)
	for _, t := range tagList {
		parse := strings.Split(t, "=")
		if len(parse) == 2 {
			tagMap[parse[0]] = parse[1]
		} else {
			tagMap[parse[0]] = ""
		}
	}
	return tagMap
}
