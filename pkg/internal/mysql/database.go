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
	"fmt"
)

// CreateDatabaseIfNotExists creates a database if it doesn't already exist
func CreateDatabaseIfNotExists(ctx context.Context, sql SQLRunner, database string) error {
	query := NewQuery(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", Escape(database)))

	if err := sql.QueryExec(ctx, query); err != nil {
		return fmt.Errorf("failed to create database, err: %s", err)
	}

	return nil
}

// DropDatabase deletes the database
func DropDatabase(ctx context.Context, sql SQLRunner, database string) error {
	query := NewQuery(fmt.Sprintf("DROP DATABASE IF EXISTS %s", escapeID(database)))

	if err := sql.QueryExec(ctx, query); err != nil {
		return fmt.Errorf("failed to remove database, err: %s", err)
	}

	return nil
}
