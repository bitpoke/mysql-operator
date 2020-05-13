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
	"context"
	"errors"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"strings"

	mysqlv1alpha1 "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
)

// CreateUserIfNotExists creates a user if it doesn't already exist and it gives it the specified permissions
func CreateUserIfNotExists(ctx context.Context, sql SQLRunner,
	user, pass string, allowedHosts []string, permissions []mysqlv1alpha1.MysqlPermission,
	resourceOptions corev1.ResourceList,
) error {

	// throw error if there are no allowed hosts
	if len(allowedHosts) == 0 {
		return errors.New("no allowedHosts specified")
	}

	queries := []Query{
		getCreateUserQuery(user, pass, allowedHosts),
		getAlterUserQuery(user, pass, allowedHosts, resourceOptions),
	}

	if len(permissions) > 0 {
		queries = append(queries, permissionsToQuery(permissions, user, allowedHosts))
	}

	query := BuildAtomicQuery(queries...)

	if err := sql.QueryExec(ctx, query); err != nil {
		return fmt.Errorf("failed to configure user (user/pass/access), err: %s", err)
	}

	return nil
}

func getAlterUserQuery(user, pwd string, allowedHosts []string, resourceOptions corev1.ResourceList) Query {
	args := []interface{}{}
	q := "ALTER USER"

	// add user identifications (user@allowedHost) pairs
	ids, idsArgs := getUsersIdentification(user, &pwd, allowedHosts)
	q += ids
	args = append(args, idsArgs...)

	// add WITH statement for resource options
	if len(resourceOptions) > 0 {
		q += " WITH"
		for key, valQ := range resourceOptions {
			q += fmt.Sprintf(" %s ?", Escape(string(key)))
			value, _ := valQ.AsInt64()
			args = append(args, int(value))
		}
	}

	return NewQuery(q, args...)
}

func getCreateUserQuery(user, pwd string, allowedHosts []string) Query {
	idsTmpl, idsArgs := getUsersIdentification(user, &pwd, allowedHosts)

	return NewQuery(fmt.Sprintf("CREATE USER IF NOT EXISTS%s", idsTmpl), idsArgs...)
}

func getUsersIdentification(user string, pwd *string, allowedHosts []string) (ids string, args []interface{}) {
	for i, host := range allowedHosts {
		// add comma if more than one allowed hosts are used
		if i > 0 {
			ids += ","
		}

		if pwd != nil {
			ids += " ?@? IDENTIFIED BY ?"
			args = append(args, user, host, *pwd)
		} else {
			ids += " ?@?"
			args = append(args, user, host)
		}
	}

	return ids, args
}

// DropUser removes a MySQL user if it exists, along with its privileges
func DropUser(ctx context.Context, sql SQLRunner, user string, host *string) error {
	usrTmpl := "?"
	args := []interface{}{user}

	if host != nil {
		usrTmpl = "?@?"
		args = append(args, *host)
	}

	query := NewQuery(fmt.Sprintf("DROP USER IF EXISTS %s;", usrTmpl), args...)

	if err := sql.QueryExec(ctx, query); err != nil {
		return fmt.Errorf("failed to delete user, err: %s", err)
	}

	return nil
}

func permissionsToQuery(permissions []mysqlv1alpha1.MysqlPermission, user string, allowedHosts []string) Query {
	permQueries := []Query{}

	for _, perm := range permissions {
		// If you wish to grant permissions on all tables, you should explicitly use "*"
		for _, table := range perm.Tables {
			args := []interface{}{}

			escPerms := []string{}
			for _, perm := range perm.Permissions {
				escPerms = append(escPerms, Escape(perm))
			}

			schemaTable := fmt.Sprintf("%s.%s", escapeID(perm.Schema), escapeID(table))

			// Build GRANT query
			idsTmpl, idsArgs := getUsersIdentification(user, nil, allowedHosts)

			query := "GRANT " + strings.Join(escPerms, ", ") + " ON " + schemaTable + " TO" + idsTmpl
			args = append(args, idsArgs...)

			permQueries = append(permQueries, NewQuery(query, args...))
		}
	}

	return ConcatenateQueries(permQueries...)
}

func escapeID(id string) string {
	if id == "*" {
		return id
	}

	// don't allow using ` in id name
	id = strings.ReplaceAll(id, "`", "")

	return fmt.Sprintf("`%s`", id)
}
