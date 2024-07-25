/*
Copyright 2024.

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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DnsEntryItemSpec defines the desired state of DnsEntry
type DnsEntryItemSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Name            string `json:"name"`
	ZoneName        string `json:"zone_name"`
	Type            string `json:"type"`
	TTL             int    `json:"ttl"`
	Proxied         bool   `json:"proxied"`
	KeepAfterDelete bool   `json:"keep_after_delete"`
}

type DnsEntrySpec struct {
	Items               []DnsEntryItemSpec `json:"items"`
	Cron                string             `json:"cron"`
	TriggerRecordDelete bool               `json:"trigger_record_delete"`
}

// DnsEntryStatus defines the observed state of DnsEntry
type DnsEntryStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Cron",type=string,JSONPath=`.spec.cron`
// +kubebuilder:printcolumn:name="TriggerRecordDelete",type=boolean,JSONPath=`.spec.trigger_record_delete`
// +kubebuilder:printcolumn:name="Records",type=string,JSONPath=`.spec.items..name`

// DnsEntry is the Schema for the dnsentries API
type DnsEntry struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DnsEntrySpec   `json:"spec,omitempty"`
	Status DnsEntryStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DnsEntryList contains a list of DnsEntry
type DnsEntryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DnsEntry `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DnsEntry{}, &DnsEntryList{})
}
