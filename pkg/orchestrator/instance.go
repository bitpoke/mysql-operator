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

// Package orchestrator note
// NOTE that this code is copied from:
// https://github.com/github/orchestrator/blob/master/go/inst/instance.go We
// need just the types that's why is copied instead of imported.
package orchestrator

import (
	"database/sql"
	"time"
)

// InstanceKey is the node key
type InstanceKey struct {
	Hostname string
	Port     int
}

// BinlogType can be BinaryLog or RelayLog
type BinlogType int

const (
	// BinaryLog represents mysql binary logs
	BinaryLog BinlogType = iota
	// RelayLog represents mysql relay logs
	RelayLog
)

// BinlogCoordinates described binary log coordinates in the form of log file & log position.
type BinlogCoordinates struct {
	LogFile string
	LogPos  int64
	Type    BinlogType
}

// InstanceKeyMap is a convenience struct for listing InstanceKey-s
type InstanceKeyMap map[InstanceKey]bool

// Instance represents the main structure that an node is represented in
// orchestrator
// nolint: golint,maligned
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
	SecondsBehindMaster    sql.NullInt64
	SQLDelay               uint
	ExecutedGtidSet        string
	GtidPurged             string

	SlaveLagSeconds sql.NullInt64
	//SlaveHosts                      InstanceKeyMap
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
	SecondsSinceLastSeen sql.NullInt64
	CountMySQLSnapshots  int

	// Careful. IsCandidate and PromotionRule are used together
	// and probably need to be merged. IsCandidate's value may
	// be picked up from daabase_candidate_instance's value when
	// reading an instance from the db.
	IsCandidate          bool
	PromotionRule        CandidatePromotionRule
	IsDowntimed          bool
	DowntimeReason       string
	DowntimeOwner        string
	DowntimeEndTimestamp string
	ElapsedDowntime      time.Duration
	UnresolvedHostname   string
	AllowTLS             bool

	LastDiscoveryLatency time.Duration
}

// TopologyRecovery is the structure from orchestrator that represents a
// recovery
// nolint: golint,maligned
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

// CandidatePromotionRule describe the promotion preference/rule for an instance.
// It maps to promotion_rule column in candidate_database_instance
type CandidatePromotionRule string

// nolint: golint
const (
	MustPromoteRule      CandidatePromotionRule = "must"
	PreferPromoteRule    CandidatePromotionRule = "prefer"
	NeutralPromoteRule   CandidatePromotionRule = "neutral"
	PreferNotPromoteRule CandidatePromotionRule = "prefer_not"
	MustNotPromoteRule   CandidatePromotionRule = "must_not"
)

// Maintenance indicates a maintenance entry (also in the database)
// nolint: golint
type Maintenance struct {
	MaintenanceId  uint
	Key            InstanceKey
	BeginTimestamp string
	SecondsElapsed uint
	IsActive       bool
	Owner          string
	Reason         string
}
