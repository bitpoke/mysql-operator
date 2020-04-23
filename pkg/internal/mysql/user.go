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

// CreateUserIfNotExists creates a user if it doesn't already exist and it gives it the specified permissions
func CreateUserIfNotExists(
	cfg *Config, user, pass, allowedHost string, permissions []mysqlv1alpha1.MySQLPermission,
	resourceOptions mysqlv1alpha1.AccountResourceLimits,
) error {
	queries := []Query{
		NewQuery("CREATE USER IF NOT EXISTS ?@? IDENTIFIED BY ?", user, allowedHost, pass),
		getAlterUserQuery(user, pass, allowedHost, resourceOptions),
	}

	if len(permissions) > 0 {
		permissionsQuery, err := permissionsToQuery(permissions, user, allowedHost)
		if err != nil {
			return err
		}

		queries = append(queries, permissionsQuery)
	}

	query := BuildAtomicQuery(queries...)

	if err := cfg.RunQuery(query.escapedQuery, query.args...); err != nil {
		return fmt.Errorf("failed to configure user (user/pass/access), err: %s", err)
	}

	return nil
}

func getAlterUserQuery(user, pwd, allowedHost string, resourceOptions mysqlv1alpha1.AccountResourceLimits) Query {
	q := "ALTER USER ?@? IDENTIFIED BY ?"
	args := []interface{}{user, pwd, allowedHost}

	if len(resourceOptions) > 0 {
		q += " WITH"
		for key, value := range resourceOptions {
			q += " ?=?"
			args = append(args, key, value)
		}
	}

	return NewQuery(q, args...)
}

// DropUser removes a MySQL user if it exists, along with its privileges
func DropUser(cfg *Config, user string) error {
	query := NewQuery("DROP USER IF EXISTS ?;", user)

	if err := cfg.RunQuery(query.escapedQuery, query.args...); err != nil {
		return fmt.Errorf("failed to delete user, err: %s", err)
	}

	return nil
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
