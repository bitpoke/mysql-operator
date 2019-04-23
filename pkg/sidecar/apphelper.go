/*
Copyright 2018 Pressinfra SRL

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

// const (
// 	// timeOut represents the number of tries to check mysql to be ready.
// 	timeOut = 60
// 	// connRetry represents the number of tries to connect to master server
// 	connRetry = 10
// )

// RunSidecarCommand is the main command, and represents the runtime helper that
// configures the mysql server
func RunSidecarCommand(cfg *Config, stop <-chan struct{}) error {
	log.Info("start http server for backups")
	srv := newServer(cfg, stop)
	return srv.ListenAndServe()
}
