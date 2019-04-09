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

package sidecar

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Test sidecar appconf", func() {
	It("should create the right query for users", func() {
		Expect(createUserQuery("uName", "uPass", "%")).To(Equal(`
DROP USER IF EXISTS uName@'%';
CREATE USER uName@'%';
ALTER USER uName@'%' IDENTIFIED BY 'uPass';
`))
	})

	It("should create the right query for users with grants", func() {
		Expect(createUserQuery("uName", "uPass", "%", []string{"SELECT", "SUPER"}, "*.*")).To(Equal(`
DROP USER IF EXISTS uName@'%';
CREATE USER uName@'%';
ALTER USER uName@'%' IDENTIFIED BY 'uPass';
GRANT SELECT, SUPER ON *.* TO uName@'%';`))
	})

	It("should create the right query for users with grants", func() {
		Expect(createUserQuery("uName", "uPass", "%", []string{"SELECT"}, "*.*", []string{"SUPER"}, "a.b")).To(Equal(`
DROP USER IF EXISTS uName@'%';
CREATE USER uName@'%';
ALTER USER uName@'%' IDENTIFIED BY 'uPass';
GRANT SELECT ON *.* TO uName@'%';
GRANT SUPER ON a.b TO uName@'%';`))
	})

	It("should fail if bad parameters passed", func() {
		Expect(func() { createUserQuery("a", "b", "c", "d") }).To(Panic())
		Expect(func() { createUserQuery("a", "b", "c", "d", "c") }).To(Panic())
		Expect(func() { createUserQuery("a", "b", "c", []string{"d"}, 1) }).To(Panic())
	})
})
