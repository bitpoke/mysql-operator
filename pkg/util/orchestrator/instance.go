/*
Copyright 2018 Pressinfra SRL
Copyright 2014 Outbrain Inc.

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

package orchestrator

import (
	"time"
)

type InstanceKey struct {
	Hostname string
	Port     int
}

type BinlogType int

const (
	BinaryLog BinlogType = iota
	RelayLog
)

// BinlogCoordinates described binary log coordinates in the form of log file & log position.
type BinlogCoordinates struct {
	LogFile string
	LogPos  int64
	Type    BinlogType
}

type NullInt64 struct {
	Int64 int64
	Valid bool // Valid is true if Int64 is not NULL
}

type Instance struct {
	Key                    InstanceKey
	InstanceAlias          string
	Uptime                 uint
	ServerID               uint
	ServerUUID             string
	Version                string
	VersionComment         string
	FlavorName             string
	ReadOnly               bool
	Binlog_format          string
	BinlogRowImage         string
	LogBinEnabled          bool
	LogSlaveUpdatesEnabled bool
	SelfBinlogCoordinates  BinlogCoordinates
	MasterKey              InstanceKey
	IsDetachedMaster       bool
	Slave_SQL_Running      bool
	Slave_IO_Running       bool
	HasReplicationFilters  bool
	GTIDMode               string
	SupportsOracleGTID     bool
	UsingOracleGTID        bool
	UsingMariaDBGTID       bool
	UsingPseudoGTID        bool
	ReadBinlogCoordinates  BinlogCoordinates
	ExecBinlogCoordinates  BinlogCoordinates
	IsDetached             bool
	RelaylogCoordinates    BinlogCoordinates
	LastSQLError           string
	LastIOError            string
	SecondsBehindMaster    NullInt64
	SQLDelay               uint
	ExecutedGtidSet        string
	GtidPurged             string

	SlaveLagSeconds                 NullInt64
	ClusterName                     string
	SuggestedClusterAlias           string
	DataCenter                      string
	PhysicalEnvironment             string
	ReplicationDepth                uint
	IsCoMaster                      bool
	HasReplicationCredentials       bool
	ReplicationCredentialsAvailable bool
	SemiSyncEnforced                bool
	SemiSyncMasterEnabled           bool
	SemiSyncReplicaEnabled          bool

	LastSeenTimestamp    string
	IsLastCheckValid     bool
	IsUpToDate           bool
	IsRecentlyChecked    bool
	SecondsSinceLastSeen NullInt64
	CountMySQLSnapshots  int

	LastDiscoveryLatency time.Duration
}

type TopologyRecovery struct {
	Id                     int64
	UID                    string
	SuccessorKey           *InstanceKey
	SuccessorAlias         string
	IsActive               bool
	IsSuccessful           bool
	AllErrors              []string
	RecoveryStartTimestamp string
	RecoveryEndTimestamp   string
	ProcessingNodeHostname string
	ProcessingNodeToken    string
	Acknowledged           bool
	AcknowledgedAt         string
	AcknowledgedBy         string
	AcknowledgedComment    string
	LastDetectionId        int64
	RelatedRecoveryId      int64
}
