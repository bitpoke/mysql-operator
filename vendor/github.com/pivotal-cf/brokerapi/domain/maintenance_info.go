package domain

import "reflect"

type MaintenanceInfo struct {
	Public  map[string]string `json:"public,omitempty"`
	Private string            `json:"private,omitempty"`
	Version string            `json:"version,omitempty"`
}

func (m *MaintenanceInfo) Equals(input MaintenanceInfo) bool {
	return reflect.DeepEqual(*m, input)
}

func (m *MaintenanceInfo) NilOrEmpty() bool {
	return m == nil || m.Equals(MaintenanceInfo{})
}
