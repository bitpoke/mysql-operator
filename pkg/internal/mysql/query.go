/*
Copyright 2020 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package mysql

import (
	"errors"
	"fmt"
	"strings"

	mysqlv1alpha1 "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
)

// Query contains a escaped query string with variables marked with a question mark (?) and a slice
// of positional arguments
type Query struct {
	escapedQuery string
	args         []interface{}
}

// NewQuery returns a new Query object
func NewQuery(q string, args ...interface{}) Query {
	if q == "" {
		panic("unexpected empty query")
	}

	if !strings.HasSuffix(q, ";") {
		q += ";"
	}

	return Query{
		escapedQuery: q,
		args:         args,
	}
}

// ConcatenateQueries concatenates the provided queries into a single query
func ConcatenateQueries(queries ...Query) Query {
	args := []interface{}{}
	query := ""

	for _, pq := range queries {
		if query != "" {
			if !strings.HasSuffix(query, "\n") {
				query += "\n"
			}
		}

		query += pq.escapedQuery
		args = append(args, pq.args...)
	}

	return NewQuery(query, args...)
}

// BuildAtomicQuery concatenates the provided queries into a single query wrapped in a BEGIN COMMIT block
func BuildAtomicQuery(queries ...Query) Query {
	queries = append([]Query{NewQuery("BEGIN")}, queries...)
	queries = append(queries, NewQuery("COMMIT"))

	return ConcatenateQueries(queries...)
}

func permissionsToQuery(permissions []mysqlv1alpha1.MySQLPermission, user, allowedHost string) (Query, error) {
	permQueries := []Query{}

	for _, perm := range permissions {
		for _, table := range perm.Tables {
			args := []interface{}{}

			// There are no tables so therefore no permissions are granted
			// If you wish to grant permissions on all tables, you should explicitly use "*"
			if len(perm.Tables) == 0 {
				continue
			}

			// We don't allow backticks (`) in schema and tables
			if strings.Contains(perm.Schema, "`") {
				return Query{}, errors.New("schema is not allowed to contain backticks")
			}

			// Build tables query chunk
			if strings.Contains(table, "`") {
				return Query{}, errors.New("table is not allowed to contain backticks")
			}

			// Wrap the table in backticks if it's not wildcard
			if table != "*" {
				table = fmt.Sprintf("`%s`", table)
			}

			schemaTable := fmt.Sprintf("`%s`.%s", perm.Schema, table)

			// Add the permissions to query
			query := "GRANT " + strings.Join(perm.Permissions, ", ") + " ON " + schemaTable + " TO " + "?@?"

			args = append(args, user, allowedHost)

			permQueries = append(permQueries, NewQuery(query, args...))
		}
	}

	return ConcatenateQueries(permQueries...), nil
}
