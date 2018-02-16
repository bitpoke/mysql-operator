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

package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/presslabs/titanium/cmd/toolbox/appclone"
	"github.com/presslabs/titanium/cmd/toolbox/appinit"
	"github.com/presslabs/titanium/pkg/util/logs"
)

func main() {
	logs.InitLogs()
	defer logs.FlushLogs()
	stopCh := SetupSignalHandler()

	cmd := &cobra.Command{
		Use:   "titanium-toolbox",
		Short: fmt.Sprintf("Titanium operator toolbox."),
		Long: `
titanium-toolbox: helper for config pods`,
		Run: func(cmd *cobra.Command, args []string) {
			glog.Fatal("Running toolbox, see help.")
		},
	}

	initCmd := &cobra.Command{
		Use:   "init-configs",
		Short: "Init subcommand, for init files.",
		Run: func(cmd *cobra.Command, args []string) {
			err := appinit.RunInitCommand(stopCh)
			if err != nil {
				glog.Fatalf("Init command failed with error: %s .", err)
			}

		},
	}
	cmd.AddCommand(initCmd)

	cloneCmd := &cobra.Command{
		Use:   "clone",
		Short: "Clone data from a bucket or prior node.",
		Run: func(cmd *cobra.Command, args []string) {
			err := appclone.RunCloneCommand(stopCh)
			if err != nil {
				glog.Fatalf("Clone command failed with error: %s .", err)
			}
		},
	}
	cmd.AddCommand(cloneCmd)

	cmd.Flags().AddGoFlagSet(flag.CommandLine)
	flag.CommandLine.Parse([]string{})
	if err := cmd.Execute(); err != nil {
		glog.Fatal(err)
	}
}

var shutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}
var onlyOneSignalHandler = make(chan struct{})

// SetupSignalHandler registered for SIGTERM and SIGINT. A stop channel is returned
// which is closed on one of these signals. If a second signal is caught, the program
// is terminated with exit code 1.
func SetupSignalHandler() (stopCh <-chan struct{}) {
	close(onlyOneSignalHandler) // panics when called twice

	stop := make(chan struct{})
	c := make(chan os.Signal, 2)
	signal.Notify(c, shutdownSignals...)
	go func() {
		<-c
		close(stop)
		fmt.Println("Press C-c again to exit.")
		<-c
		os.Exit(1) // second signal. Exit directly.
	}()

	return stop
}
