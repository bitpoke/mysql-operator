/*
Copyright 2019 Harvest

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
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io/ioutil"
	"os"
	"path"
)

var _ = Describe("Test RunCloneCommand cloning logic", func() {

	var (
		cfg               *Config
		fakeBackupFile    string // as named in fakeServer.validXBStream
		fakeMasterServer  *fakeServer
		fakeReplicaServer *fakeServer
		// Normally, these are true k8s services, each listening on
		// SidecarServerPort. Since we can't simulate that in unit tests, we put
		// each "service" on its own port.
		masterServiceAddr          = fmt.Sprintf(":%d", serverPort)
		healthyReplicasServiceAddr = ":8081"
	)

	setupFakeDataDir := func() {
		tempDataDir, err := ioutil.TempDir("", "mysql-operator-tests")
		Expect(err).ToNot(HaveOccurred())
		dataDir = tempDataDir
		fakeBackupFile = path.Join(dataDir, "t")
	}

	teardownFakeDataDir := func() {
		err := os.RemoveAll(dataDir)
		Expect(err).ToNot(HaveOccurred())
	}

	startFakeServer := func(address string) *fakeServer {
		fakeSrv := newFakeServer(address, cfg)
		err := fakeSrv.start()
		Expect(err).NotTo(HaveOccurred())
		return fakeSrv
	}

	startFakeMasterService := func() {
		fakeMasterServer = startFakeServer(masterServiceAddr)
	}

	startFakeReplicaService := func() {
		fakeReplicaServer = startFakeServer(healthyReplicasServiceAddr)
	}

	stopFakeMasterService := func() {
		if fakeMasterServer != nil {
			err := fakeMasterServer.stop()
			Expect(err).ToNot(HaveOccurred())
		}
	}

	stopFakeReplicaService := func() {
		if fakeReplicaServer != nil {
			err := fakeReplicaServer.stop()
			Expect(err).ToNot(HaveOccurred())
		}
	}

	// Don't let xtrabackup try to --prepare our little fake xbstream sample or
	// it will return errors.
	disableXtraBackup := func() {
		xtrabackupCommand = "echo"
	}

	BeforeSuite(func() {
		cfg = &Config{
			masterService:              "localhost" + masterServiceAddr,
			healthyReplicaCloneService: "localhost" + healthyReplicasServiceAddr,
		}

		setupFakeDataDir()
		disableXtraBackup()
	})

	AfterSuite(func() {
		teardownFakeDataDir()
	})

	BeforeEach(func() {
		err := os.RemoveAll(fakeBackupFile)
		Expect(err).ToNot(HaveOccurred())
		cfg.ExistsMySQLData = false
		startFakeReplicaService()
		startFakeMasterService()
	})

	AfterEach(func() {
		stopFakeMasterService()
		stopFakeReplicaService()
	})

	It("should not try to clone when data already exists", func() {
		cfg.ExistsMySQLData = true

		err := RunCloneCommand(cfg)
		Expect(err).ToNot(HaveOccurred())

		Expect(fakeReplicaServer.backupRequestsReceived()).To(Equal(0))
		Expect(fakeMasterServer.backupRequestsReceived()).To(Equal(0))

		Expect(fakeBackupFile).ShouldNot(BeAnExistingFile())
	})

	It("should request a backup and succeed ", func() {
		Expect(fakeBackupFile).ShouldNot(BeAnExistingFile())

		err := RunCloneCommand(cfg)
		Expect(err).To(Succeed())

		Expect(fakeReplicaServer.backupRequestsReceived()).To(Equal(1))
		Expect(fakeMasterServer.backupRequestsReceived()).To(Equal(0))

		Expect(fakeBackupFile).Should(BeAnExistingFile())
	})

	Context("with truncated xbstream data from replicas", func() {

		BeforeEach(func() {
			fakeReplicaServer.simulateTruncate = true
		})

		It("cloneFromSource should clean up the data directory after failure", func() {
			Expect(fakeBackupFile).ShouldNot(BeAnExistingFile())

			err := cloneFromSource(cfg, healthyReplicasServiceAddr)
			Expect(err).To(HaveOccurred())

			Expect(fakeReplicaServer.backupRequestsReceived()).To(Equal(1))
			Expect(fakeMasterServer.backupRequestsReceived()).To(Equal(0))

			Expect(fakeBackupFile).ShouldNot(BeAnExistingFile())
		})

		It("should not fall back to master service", func() {
			Expect(fakeBackupFile).ShouldNot(BeAnExistingFile())

			err := RunCloneCommand(cfg)
			Expect(err).To(HaveOccurred())

			Expect(fakeReplicaServer.backupRequestsReceived()).To(Equal(1))
			Expect(fakeMasterServer.backupRequestsReceived()).To(Equal(0))

			Expect(fakeBackupFile).ShouldNot(BeAnExistingFile())
		})

	})

	Context("with http error during backup", func() {

		BeforeEach(func() {
			fakeReplicaServer.simulateError = true
		})

		It("cloneFromSource should clean up the data directory after failure", func() {
			Expect(fakeBackupFile).ShouldNot(BeAnExistingFile())

			err := cloneFromSource(cfg, healthyReplicasServiceAddr)
			Expect(err).To(HaveOccurred())

			Expect(fakeReplicaServer.backupRequestsReceived()).To(Equal(1))
			Expect(fakeMasterServer.backupRequestsReceived()).To(Equal(0))

			Expect(fakeBackupFile).ShouldNot(BeAnExistingFile())
		})

		It("should not fall back to master service", func() {
			Expect(fakeBackupFile).ShouldNot(BeAnExistingFile())

			err := RunCloneCommand(cfg)
			Expect(err).To(HaveOccurred())

			Expect(fakeReplicaServer.backupRequestsReceived()).To(Equal(1))
			Expect(fakeMasterServer.backupRequestsReceived()).To(Equal(0))

			Expect(fakeBackupFile).ShouldNot(BeAnExistingFile())
		})

	})

	Context("with no healthy replicas service", func() {

		BeforeEach(func() {
			stopFakeReplicaService()
		})

		It("should fall back to master service and succeed", func() {
			Expect(fakeBackupFile).ShouldNot(BeAnExistingFile())

			err := RunCloneCommand(cfg)
			Expect(err).ToNot(HaveOccurred())

			Expect(fakeReplicaServer.backupRequestsReceived()).To(Equal(0))
			Expect(fakeMasterServer.backupRequestsReceived()).To(Equal(1))

			Expect(fakeBackupFile).Should(BeAnExistingFile())
		})

	})

	Context("with no healthy replicas or master service", func() {

		BeforeEach(func() {
			stopFakeReplicaService()
			stopFakeMasterService()
		})

		It("should return nil", func() {
			Expect(fakeBackupFile).ShouldNot(BeAnExistingFile())

			err := RunCloneCommand(cfg)
			Expect(err).To(Succeed())

			Expect(fakeBackupFile).ShouldNot(BeAnExistingFile())
		})

	})

})
