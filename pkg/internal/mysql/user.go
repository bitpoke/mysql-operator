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
	"fmt"

	mysqlv1alpha1 "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
)

// CreateUserIfNotExists creates a user if it doesn't already exist and it gives it the specified permissions
func CreateUserIfNotExists(
	cfg *Config, user, pass, allowedHost string, permissions []mysqlv1alpha1.MySQLPermission,
) error {
	queries := []Query{
		NewQuery("CREATE USER IF NOT EXISTS ?@? IDENTIFIED BY ?", user, allowedHost, pass),
		NewQuery("ALTER USER ?@? IDENTIFIED BY ?", user, allowedHost, pass),
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

// DropUser removes a MySQL user if it exists, along with its privileges
func DropUser(cfg *Config, user string) error {
	query := NewQuery("DROP USER IF EXISTS ?;", user)

	if err := cfg.RunQuery(query.escapedQuery, query.args...); err != nil {
		return fmt.Errorf("failed to delete user, err: %s", err)
	}

	return nil
}
