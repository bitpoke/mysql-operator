/*
Copyright 2019 Pressinfra SRL

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

package mysqldsl

import (
	"fmt"
	"strings"
)

// CreateUserQuery returns a list of MySQL queries that creates a new user,
// sets the password on it and adds all related permissions
func CreateUserQuery(name, pass, host string, rights ...interface{}) []string {
	user := fmt.Sprintf("%s@'%s'", name, host)

	queries := []string{
		fmt.Sprintf("DROP USER IF EXISTS %s", user),
		fmt.Sprintf("CREATE USER %s", user),
		fmt.Sprintf("ALTER USER %s IDENTIFIED BY '%s'", user, pass),
	}

	if len(rights)%2 != 0 {
		panic("not a good number of parameters")
	}
	grants := []string{}
	for i := 0; i < len(rights); i += 2 {
		var (
			right []string
			on    string
			ok    bool
		)
		if right, ok = rights[i].([]string); !ok {
			panic("[right] not a good parameter")
		}
		if on, ok = rights[i+1].(string); !ok {
			panic("[on] not a good parameter")
		}
		grant := fmt.Sprintf("GRANT %s ON %s TO %s", strings.Join(right, ", "), on, user)
		grants = append(grants, grant)
	}

	return append(queries, grants...)
}
