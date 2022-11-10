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
	"github.com/go-logr/logr"
	"github.com/google/uuid"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/extra/bundebug"
	dossierv1 "github.com/voodoo-patch/jupyter-hybrid-operator/api/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"regexp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strings"
	"time"

	"github.com/uptrace/bun"
)

var finalizer = dossierv1.GroupVersion.Group + "/finalizer"
var logger logr.Logger
var dsnProtocolRegex = regexp.MustCompile(`^(postgresql)([^:]*)`)

// PostgresUserReconciler reconciles a PostgresUser object
type PostgresUserReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

type User struct {
	bun.BaseModel `bun:"table:users,alias:u"`

	ID                 int64          `bun:"id,pk,autoincrement"`
	Name               string         `bun:"name,notnull"`
	Admin              bool           `bun:"admin,notnull"`
	Created            time.Time      `bun:"created,nullzero,notnull,default:current_timestamp"`
	LastActivity       time.Time      `bun:"last_activity,nullzero,notnull,default:current_timestamp"`
	CookieId           string         `bun:"cookie_id,notnull,type:uuid,default:uuid_generate_v4()"`
	State              sql.NullString `bun:"state"`
	EncryptedAuthState []byte         `bun:"encrypted_auth_state,notnull"`
}

//+kubebuilder:rbac:groups=dossier.di.unito.it,resources=postgresusers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=dossier.di.unito.it,resources=postgresusers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=dossier.di.unito.it,resources=postgresusers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *PostgresUserReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var result ctrl.Result
	_ = log.FromContext(ctx)

	logger = r.Log.WithValues("postgresUser", req.NamespacedName)

	// Fetch the PostgresUser instance
	dbUser := &dossierv1.PostgresUser{}
	err := r.Get(ctx, req.NamespacedName, dbUser)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not existing, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			logger.Info("PostgresUser resource not existing. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		logger.Error(err, "Failed to get PostgresUser")
		return ctrl.Result{}, err
	}

	isUserMarkedToBeDeleted := dbUser.GetDeletionTimestamp() != nil
	if isUserMarkedToBeDeleted {
		return r.handleDeletion(ctx, dbUser, logger)
	}

	// Add finalizer for this CR
	result, err = r.addFinalizer(ctx, dbUser)
	if err != nil {
		return result, err
	}

	err = r.addUserIfNotExists(ctx, dbUser)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *PostgresUserReconciler) addUserIfNotExists(ctx context.Context, dbUser *dossierv1.PostgresUser) error {
	dossier, err := r.getDossier(ctx, types.NamespacedName{Namespace: dbUser.Namespace, Name: dbUser.Spec.Dossier})
	if err != nil {
		return err
	}
	db, err := r.connectToDb(dossier)
	if err != nil {
		logger.Error(err, "Unable to establish a connection to the database")
		return err
	}

	if dbUser.Status.Username != "" && dbUser.Status.Username != dbUser.Spec.Username {
		err = deleteUser(ctx, db, dbUser.Status.Username)
		if err != nil {
			return err
		}
	}

	existingUser, err := getUser(ctx, db, dbUser)
	if err != nil && err != sql.ErrNoRows {
		logger.Error(err, "Error while retrieving user from db")
		return err
	}
	if err == sql.ErrNoRows {
		err = r.addUser(ctx, db, dbUser)
	} else if existingUser.Admin != dbUser.Spec.IsAdmin {
		err = deleteUser(ctx, db, dbUser.Spec.Username)
		if err != nil {
			return err
		}
		err = r.addUser(ctx, db, dbUser)
	}
	return err
}

func (r *PostgresUserReconciler) getDossier(ctx context.Context, dossierName types.NamespacedName) (*dossierv1.Dossier, error) {
	dossier := &dossierv1.Dossier{}
	err := r.Get(ctx, dossierName, dossier)
	if err != nil {
		logger.Error(err, "Failed to get Dossier named: "+dossierName.String())
		return nil, err
	}
	return dossier, nil
}

func (r *PostgresUserReconciler) connectToDb(dossier *dossierv1.Dossier) (*bun.DB, error) {
	//connectionString := "postgres://jhub@localhost:5432/jhubdb"
	jhub := dossier.Spec.Jhub.UnstructuredContent()
	connectionString := getValueByKey("hub.db.url", jhub).(string)
	password := getValueByKey("hub.db.password", jhub).(string)

	sqldb := sql.OpenDB(pgdriver.NewConnector(
		pgdriver.WithDSN(sanitizeDsn(connectionString)),
		pgdriver.WithPassword(password),
		pgdriver.WithInsecure(true)))
	db := bun.NewDB(sqldb, pgdialect.New())
	db.AddQueryHook(bundebug.NewQueryHook(
		bundebug.WithVerbose(true),
		bundebug.FromEnv("BUNDEBUG"),
	))

	return db, db.Ping()
}

func getValueByKey(key string, src map[string]interface{}) interface{} {
	splits := strings.Split(key, ".")

	if len(splits) > 1 {
		return getValueByKey(strings.Join(splits[1:], "."), src[splits[0]].(map[string]interface{}))
	} else {
		return src[splits[0]]
	}
}

func sanitizeDsn(dsn string) string {
	return dsnProtocolRegex.ReplaceAllString(dsn, "$1")
}

func getUser(ctx context.Context, db *bun.DB, dbUser *dossierv1.PostgresUser) (*User, error) {
	user := &User{}
	err := db.
		NewSelect().
		Model(user).
		Where("?TableAlias.name = ?", dbUser.Spec.Username).
		Limit(1).
		Scan(ctx)
	return user, err
}

func (r *PostgresUserReconciler) addUser(ctx context.Context, db *bun.DB, dbUser *dossierv1.PostgresUser) error {
	user := &User{
		Name:         dbUser.Spec.Username,
		Admin:        dbUser.Spec.IsAdmin,
		Created:      time.Now(),
		LastActivity: time.Now(),
		CookieId:     uuid.New().String(),
	}
	_, err := db.NewInsert().
		Model(user).
		Exec(ctx)
	if err != nil {
		logger.Error(err, "Error while adding user to db")
	}

	dbUser.Status.Username = dbUser.Spec.Username
	err = r.Status().Update(ctx, dbUser)
	if err != nil {
		logger.Error(err, "Error while updating user status")
	}

	return err
}

func deleteUser(ctx context.Context, db *bun.DB, username string) error {
	_, err := db.NewDelete().
		Model(&User{}).
		Where("?TableAlias.name = ?", username).
		Exec(ctx)
	if err != nil {
		logger.Error(err, "Error while deleting user from db")
	}
	return err
}

func (r *PostgresUserReconciler) addFinalizer(ctx context.Context, dbUser *dossierv1.PostgresUser) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(dbUser, finalizer) {
		controllerutil.AddFinalizer(dbUser, finalizer)
		err := r.Update(ctx, dbUser)
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *PostgresUserReconciler) handleDeletion(ctx context.Context, dbUser *dossierv1.PostgresUser, log logr.Logger) (ctrl.Result, error) {
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

func (r *PostgresUserReconciler) finalizePostrgresUser(ctx context.Context, log logr.Logger, dbUser *dossierv1.PostgresUser) error {
	dossier, err := r.getDossier(ctx, types.NamespacedName{dbUser.Namespace, dbUser.Spec.Dossier})
	if err != nil {
		return err
	}
	db, err := r.connectToDb(dossier)
	err = deleteUser(ctx, db, dbUser.Spec.Username)
	if err != nil {
		return err
	}
	log.Info("Successfully finalized dbUser")

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PostgresUserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dossierv1.PostgresUser{}).
		Complete(r)
}
