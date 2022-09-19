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

	// Jhub is a generic object that should reflect jupyterhub values schema
	//+kubebuilder:pruning:PreserveUnknownFields
	Jhub unstructured.Unstructured `json:"jhub,omitempty" yaml:"jhub,omitempty"`

	// Postgres is a generic object that should reflect postgres values schema
	//+kubebuilder:pruning:PreserveUnknownFields
	Postgres unstructured.Unstructured `json:"postgres,omitempty" yaml:"postgres,omitempty"`
}

// JupyterhubStatus defines the observed state of Dossier
type DossierStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// JhubCR is the name of the created jupyterhub custom resource
	JhubCR string `json:"jhub-cr,omitempty" yaml:"jhub-cr,omitempty"`

	// PostgresCR is the name of the created postgres custom resource
	PostgresCR string `json:"postgres-cr,omitempty" yaml:"postgres-cr,omitempty"`
}

// Dossier is the Schema for the dossiers API
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
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
