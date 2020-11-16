/*
Copyright 2017 The Kubernetes Authors.

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

package v1alpha1

import (
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ResourceStatus string

const (
	ConnectorStatusApplied = "Applied"
	ConnectorStatusPending = "Pending"
	ConnectorStatusFailed  = "Failed"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Connector is a specification for a Connector resource
type Connector struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Config ConnectorConfig `json:"config"`
	Status ConnectorStatus `json:"status"`
}

// ConnectorConfig is the spec for a Connector resource
type ConnectorConfig map[string]ConfigItem

func (i *ConfigItem) UnmarshalJSON(data []byte) error {
	switch data[0] {
	case '{':
		i.ValueFrom = &ValueFrom{}
		if err := json.Unmarshal(data, i.ValueFrom); err != nil {
			return err
		}
	default:
		if err := json.Unmarshal(data, &i.Value); err != nil {
			return err
		}
	}
	return nil
}

type ConfigItem struct {
	Value     interface{} `json:"-"`
	ValueFrom *ValueFrom  `json:"valueFrom"`
}

type ValueFrom struct {
	Secret    *Ref `json:"secret"`
	ConfigMap *Ref `json:"configMap"`
}

type Ref struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

// ConnectorStatus is the status for a Connector resource
type ConnectorStatus struct {
	Applied ResourceStatus `json:"applied"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ConnectorList is a list of Connector resources
type ConnectorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Connector `json:"items"`
}
