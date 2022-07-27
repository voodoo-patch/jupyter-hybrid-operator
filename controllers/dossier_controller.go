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

package controllers

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	// "sigs.k8s.io/controller-runtime/pkg/log"

	streamflowv1 "github.com/voodoo-patch/jupyter-hybrid-operator/api/v1"
)

// DossierReconciler reconciles a Dossier object
type DossierReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=streamflow.edu.unito.it,resources=dossiers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=streamflow.edu.unito.it,resources=dossiers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=streamflow.edu.unito.it,resources=dossiers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Dossier object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.2/pkg/reconcile
func (r *DossierReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// TODO(user): your logic here
	log := r.Log.WithValues("dossier", req.NamespacedName)

	// Fetch the Dossier instance
	dossier := &streamflowv1.Dossier{}
	err := r.Get(ctx, req.NamespacedName, dossier)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not existingDeployment, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Info("Dossier resource not existingDeployment. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get Dossier")
		return ctrl.Result{}, err
	}

	// Define a new jhub resource
	jhub := getJhubCustomResource(dossier, false)
	err = r.Get(ctx, client.ObjectKeyFromObject(jhub), jhub)
	if err != nil && errors.IsNotFound(err) {
		jhub = getJhubCustomResource(dossier, true)
		r.logCreatingCR(log, jhub)
		err = r.Create(ctx, jhub)
		if err != nil {
			r.logFailCR(log, err, jhub)
			return ctrl.Result{}, err
		}
	}

	// Define a new postgres resource
	postgres := getPostgresCustomResource(dossier, false)
	err = r.Get(ctx, client.ObjectKeyFromObject(postgres), postgres)
	if err != nil && errors.IsNotFound(err) {
		postgres = getPostgresCustomResource(dossier, true)
		r.logCreatingCR(log, postgres)
		err = r.Create(ctx, postgres)
		if err != nil {
			r.logFailCR(log, err, postgres)
			return ctrl.Result{}, err
		}
	}

	// Resources created successfully - return and requeue
	return ctrl.Result{Requeue: true}, nil
}

func (r *DossierReconciler) logFailCR(log logr.Logger, err error, cr *unstructured.Unstructured) {
	kind := cr.GetKind()
	log.Error(err, fmt.Sprintf("Failed to create new %s", kind), fmt.Sprintf("%s.Namespace", kind), cr.GetNamespace(), fmt.Sprintf("%s.Name", kind), cr.GetName())
}

func (r *DossierReconciler) logCreatingCR(log logr.Logger, cr *unstructured.Unstructured) {
	kind := cr.GetKind()
	log.Info(fmt.Sprintf("Creating a new %s", kind), fmt.Sprintf("%s.Namespace", kind), cr.GetNamespace(), fmt.Sprintf("%s.Name", kind), cr.GetName())
}

func getJhubCustomResource(d *streamflowv1.Dossier, withSpec bool) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	if withSpec {
		jhub := d.Spec.Jhub
		delete(jhub.Object, "kind")
		u.SetUnstructuredContent(map[string]interface{}{
			"spec": d.Spec.Jhub.Object,
		})
	}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   streamflowv1.GroupVersion.Group,
		Kind:    "Jupyterhub",
		Version: "v1alpha1",
	})
	u.SetName(d.Name + "-jhub")
	u.SetNamespace(d.Namespace)

	return u
}

func getPostgresCustomResource(d *streamflowv1.Dossier, withSpec bool) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	if withSpec {
		postgres := d.Spec.Postgres
		delete(postgres.Object, "kind")
		u.SetUnstructuredContent(map[string]interface{}{
			"spec": postgres.Object,
		})
	}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   streamflowv1.GroupVersion.Group,
		Kind:    "Postgresql",
		Version: "v1alpha1",
	})
	u.SetName(d.Name + "-postgres")
	u.SetNamespace(d.Namespace)

	return u
}

// SetupWithManager sets up the controller with the Manager.
func (r *DossierReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&streamflowv1.Dossier{}).
		Complete(r)
}
