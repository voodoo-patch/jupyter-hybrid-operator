/*
Copyright 2022.

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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DossierSpec defines the desired state of Dossier
type DossierSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of Dossier. Edit dossier_types.go to remove/update
	Foo string `json:"foo,omitempty"`

	// Jhub is a generic object that should reflect jupyterhub values schema
	//+kubebuilder:pruning:PreserveUnknownFields
	Jhub unstructured.Unstructured `json:"jhub,omitempty" yaml:"jhub,omitempty"`

	// Postgres is a generic object that should reflect postgres values schema
	//+kubebuilder:pruning:PreserveUnknownFields
	Postgres unstructured.Unstructured `json:"postgres,omitempty" yaml:"postgres,omitempty"`
}

// DossierStatus defines the observed state of Dossier
type DossierStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// JhubNodes are the names of the jhub pods
	JhubNodes []string `json:"jhub-nodes"`

	// JhubNodes are the names of the postgres pods
	PostgresNodes []string `json:"postgres-nodes"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Dossier is the Schema for the dossiers API
type Dossier struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DossierSpec   `json:"spec,omitempty"`
	Status DossierStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DossierList contains a list of Dossier
type DossierList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Dossier `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Dossier{}, &DossierList{})
}
