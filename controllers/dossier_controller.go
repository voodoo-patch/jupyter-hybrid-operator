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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	// "sigs.k8s.io/controller-runtime/pkg/log"

	dossierv1 "github.com/voodoo-patch/jupyter-hybrid-operator/api/v1"
)

var dossierFinalizer = dossierv1.GroupVersion.Group + "/finalizer"

// DossierReconciler reconciles a Dossier object
type DossierReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=dossier.di.unito.it,resources=dossiers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=dossier.di.unito.it,resources=dossiers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=dossier.di.unito.it,resources=dossiers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *DossierReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var result ctrl.Result
	_ = log.FromContext(ctx)

	log := r.Log.WithValues("dossier", req.NamespacedName)
	// Fetch the Dossier instance
	dossier := &dossierv1.Dossier{}
	err := r.Get(ctx, req.NamespacedName, dossier)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not existingDeployment, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Info("Dossier resource not existing. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get Dossier")
		return ctrl.Result{}, err
	}

	isDossierMarkedToBeDeleted := dossier.GetDeletionTimestamp() != nil
	if isDossierMarkedToBeDeleted {
		return r.handleDeletion(ctx, dossier, log)
	}

	// Add finalizer for this CR
	result, err = r.addFinalizer(ctx, dossier)
	if err != nil {
		return result, err
	}

	// Define a new jhub resource
	result, err = r.reconcileJhub(ctx, dossier, log)
	if err != nil {
		return result, err
	}

	// Define a new postgres resource
	result, err = r.reconcilePostgres(ctx, dossier, log)
	if err != nil {
		return result, err
	}

	// Resources created successfully - return and requeue
	return ctrl.Result{Requeue: true}, nil
}

func (r *DossierReconciler) addFinalizer(ctx context.Context, dossier *dossierv1.Dossier) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(dossier, dossierFinalizer) {
		controllerutil.AddFinalizer(dossier, dossierFinalizer)
		err := r.Update(ctx, dossier)
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *DossierReconciler) handleDeletion(ctx context.Context, dossier *dossierv1.Dossier, log logr.Logger) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(dossier, dossierFinalizer) {
		// Run finalization logic for dossierFinalizer. If the
		// finalization logic fails, don't remove the finalizer so
		// that we can retry during the next reconciliation.
		if err := r.finalizeDossier(ctx, log, dossier); err != nil {
			return ctrl.Result{}, err
		}

		// Remove dossierFinalizer. Once all finalizers have been
		// removed, the object will be deleted.
		controllerutil.RemoveFinalizer(dossier, dossierFinalizer)
		err := r.Update(ctx, dossier)
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *DossierReconciler) finalizeDossier(ctx context.Context, log logr.Logger, dossier *dossierv1.Dossier) error {
	// delete jhub
	jhub := getJhubCustomResource(dossier, true)
	err := r.Client.Delete(ctx, jhub)
	if err != nil {
		return err
	}

	// delete postgres
	postgres := getPostgresCustomResource(dossier, true)
	err = r.Client.Delete(ctx, postgres)
	if err != nil {
		return err
	}

	log.Info("Successfully finalized dossier")

	return nil
}

func (r *DossierReconciler) reconcileJhub(ctx context.Context, dossier *dossierv1.Dossier, log logr.Logger) (ctrl.Result, error) {
	jhub := getJhubCustomResource(dossier, false)
	err := r.Get(ctx, client.ObjectKeyFromObject(jhub), jhub)
	if err != nil && errors.IsNotFound(err) {
		jhub = getJhubCustomResource(dossier, true)
		ctrl.SetControllerReference(dossier, jhub, r.Scheme)
		r.logCreatingCR(log, jhub)
		err = r.Create(ctx, jhub)
		if err != nil {
			r.logFailCR(log, err, jhub)
			return ctrl.Result{}, err
		}
		dossier.Status.JhubCR = jhub.GetName()
		err := r.Status().Update(ctx, dossier)
		if err != nil {
			return ctrl.Result{}, err
		}
	} else {
		jhub = getJhubCustomResource(dossier, true)
		err = r.Update(ctx, jhub)
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *DossierReconciler) reconcilePostgres(ctx context.Context, dossier *dossierv1.Dossier, log logr.Logger) (ctrl.Result, error) {
	postgres := getPostgresCustomResource(dossier, false)
	err := r.Get(ctx, client.ObjectKeyFromObject(postgres), postgres)
	if err != nil && errors.IsNotFound(err) {
		postgres = getPostgresCustomResource(dossier, true)
		ctrl.SetControllerReference(dossier, postgres, r.Scheme)
		r.logCreatingCR(log, postgres)
		err = r.Create(ctx, postgres)
		if err != nil {
			r.logFailCR(log, err, postgres)
			return ctrl.Result{}, err
		}
		dossier.Status.PostgresCR = postgres.GetName()
		err := r.Status().Update(ctx, dossier)
		if err != nil {
			return ctrl.Result{}, err
		}
	} else {
		postgres = getJhubCustomResource(dossier, true)
		err = r.Update(ctx, postgres)
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *DossierReconciler) logFailCR(log logr.Logger, err error, cr *unstructured.Unstructured) {
	kind := cr.GetKind()
	log.Error(err, fmt.Sprintf("Failed to create new %s", kind), fmt.Sprintf("%s.Namespace", kind), cr.GetNamespace(), fmt.Sprintf("%s.Name", kind), cr.GetName())
}

func (r *DossierReconciler) logCreatingCR(log logr.Logger, cr *unstructured.Unstructured) {
	kind := cr.GetKind()
	log.Info(fmt.Sprintf("Creating a new %s", kind), fmt.Sprintf("%s.Namespace", kind), cr.GetNamespace(), fmt.Sprintf("%s.Name", kind), cr.GetName())
}

func getJhubCustomResource(d *dossierv1.Dossier, withSpec bool) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	if withSpec {
		jhub := d.Spec.Jhub
		delete(jhub.Object, "kind")
		u.SetUnstructuredContent(map[string]interface{}{
			"spec": d.Spec.Jhub.Object,
		})
	}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   dossierv1.GroupVersion.Group,
		Kind:    "Jupyterhub",
		Version: "v1alpha1",
	})
	u.SetName(d.Name + "-jhub")
	u.SetNamespace(d.Namespace)

	return u
}

func getPostgresCustomResource(d *dossierv1.Dossier, withSpec bool) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	if withSpec {
		postgres := d.Spec.Postgres
		delete(postgres.Object, "kind")
		u.SetUnstructuredContent(map[string]interface{}{
			"spec": postgres.Object,
		})
	}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   dossierv1.GroupVersion.Group,
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
		For(&dossierv1.Dossier{}).
		Complete(r)
}
