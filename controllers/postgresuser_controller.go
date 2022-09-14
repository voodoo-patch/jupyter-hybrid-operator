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
	"database/sql"
	"github.com/Azure/go-autorest/autorest/date"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	streamflowv1 "github.com/voodoo-patch/jupyter-hybrid-operator/api/v1"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/extra/bundebug"
)

var finalizer = streamflowv1.GroupVersion.Group + "/finalizer"

// PostgresUserReconciler reconciles a PostgresUser object
type PostgresUserReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=streamflow.edu.unito.it,resources=postgresusers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=streamflow.edu.unito.it,resources=postgresusers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=streamflow.edu.unito.it,resources=postgresusers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the PostgresUser object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.2/pkg/reconcile
func (r *PostgresUserReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var result ctrl.Result
	_ = log.FromContext(ctx)

	log := r.Log.WithValues("postgresUser", req.NamespacedName)

	// Fetch the PostgresUser instance
	dbUser := &streamflowv1.PostgresUser{}
	err := r.Get(ctx, req.NamespacedName, dbUser)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not existingDeployment, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Info("PostgresUser resource not existing Deployment. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get PostgresUser")
		return ctrl.Result{}, err
	}

	isUserMarkedToBeDeleted := dbUser.GetDeletionTimestamp() != nil
	if isUserMarkedToBeDeleted {
		return r.handleDeletion(ctx, dbUser, log)
	}

	// Add finalizer for this CR
	result, err = r.addFinalizer(ctx, dbUser)
	if err != nil {
		return result, err
	}

	db := connectToDb()
	exists, err := findUser(ctx, db, dbUser.Spec.Username)
	if err != nil {
		log.Error(err, "Error while retrieving user from db")
	}
	if !exists {
		addUser()
	}

	return ctrl.Result{}, nil
}

func connectToDb() *bun.DB {
	dsn := "postgres://postgres:@localhost:5432/test?sslmode=disable"
	// dsn := "unix://user:pass@dbname/var/run/postgresql/.s.PGSQL.5432"
	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))

	db := bun.NewDB(sqldb, pgdialect.New())

	db.AddQueryHook(bundebug.NewQueryHook(
		bundebug.WithVerbose(true),
		bundebug.FromEnv("BUNDEBUG"),
	))

	return db
}

func findUser(ctx context.Context, db *bun.DB, dbUser *streamflowv1.PostgresUser) (bool, error) {
	return db.
		NewSelect().
		TableExpr("users").
		Where("name = ?", dbUser.Spec.Username).
		Exists(ctx)
}

func addUser(ctx context.Context, db *bun.DB, dbUser *streamflowv1.PostgresUser) {
	db.NewInsert().ColumnExpr()
}

func deleteUser(ctx context.Context, db *bun.DB, dbUser *streamflowv1.PostgresUser) {
	db.NewDelete().ColumnExpr()
}

type User struct {
	bun.BaseModel `bun:"table:users,alias:u"`

	ID           int64     `bun:"id,pk"`
	Name         string    `bun:"name,notnull"`
	Admin        bool      `bun:"admin,notnull"`
	Created      date.Date `bun:"created,notnull"`
	LastActivity date.Date `bun:"last_activity,notnull"`
}

func (r *PostgresUserReconciler) addFinalizer(ctx context.Context, dbUser *streamflowv1.PostgresUser) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(dbUser, finalizer) {
		controllerutil.AddFinalizer(dbUser, finalizer)
		err := r.Update(ctx, dbUser)
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *PostgresUserReconciler) handleDeletion(ctx context.Context, dbUser *streamflowv1.PostgresUser, log logr.Logger) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(dbUser, finalizer) {
		// Run finalization logic for finalizer. If the
		// finalization logic fails, don't remove the finalizer so
		// that we can retry during the next reconciliation.
		if err := r.finalizePostrgresUser(ctx, log, dbUser); err != nil {
			return ctrl.Result{}, err
		}

		// Remove finalizer. Once all finalizers have been
		// removed, the object will be deleted.
		controllerutil.RemoveFinalizer(dbUser, finalizer)
		err := r.Update(ctx, dbUser)
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *PostgresUserReconciler) finalizePostrgresUser(ctx context.Context, log logr.Logger, dbUser *streamflowv1.PostgresUser) error {

	log.Info("Successfully finalized dbUser")

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PostgresUserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&streamflowv1.PostgresUser{}).
		Complete(r)
}
