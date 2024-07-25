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

package controller

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	dnsentryv1 "github.com/wasilak/cloudflare-ddns-operator/api/v1"
)

var (
	OperatorName      string
	DockerImageRunner string
	CronJobName       string
	BatchJobName      string
	FinalizerName     string
)

// DnsEntryReconciler reconciles a DnsEntry object
type DnsEntryReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=cloudflare-ddns.mindthebig.rocks,resources=dnsentries,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cloudflare-ddns.mindthebig.rocks,resources=dnsentries/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cloudflare-ddns.mindthebig.rocks,resources=dnsentries/finalizers,verbs=update
// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the DnsEntry object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.18.4/pkg/reconcile
func (r *DnsEntryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	crdInstance := &dnsentryv1.DnsEntry{}
	err := r.Get(ctx, req.NamespacedName, crdInstance)

	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.V(1).Info("DnsEntry resource not found. Ignoring since object must be deleted.")

			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err

	}

	if err := r.handleFinalizer(ctx, *crdInstance, FinalizerName); err != nil {
		log.Error(err, "failed to update finalizer")
		return ctrl.Result{}, err
	}

	records, err := getRecordsFromCRD(ctx, crdInstance)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Check if this Batch job already exists
	// The `existingItem` variable is being used to check if a specific Batch job (CronJob) already exists in the
	// cluster. It is being initialized as an empty `batchv1.CronJob` object and then an attempt is made
	// to retrieve the CronJob with the same name and namespace as the `DnsEntry` object being reconciled.
	// The purpose is to determine whether the Batch job already exists in the cluster or not.
	existingItem := &batchv1.CronJob{}

	err = r.Get(ctx, types.NamespacedName{Name: crdInstance.Name, Namespace: crdInstance.Namespace}, existingItem)
	if err != nil {
		log.V(1).Info("Cron job not found", "Namespace", existingItem.Namespace, "Name", existingItem.Name, "error", err)
		// return reconcile.Result{}, err
	}

	var result *reconcile.Result
	result, err = r.ensureCronJob(ctx, crdInstance, r.actualCronJob(crdInstance, records, CronJobName, DockerImageRunner, OperatorName))
	if result != nil {
		return *result, err
	}

	// if crdInstance.ObjectMeta.Annotations == nil {
	// 	crdInstance.ObjectMeta.Annotations = make(map[string]string)
	// }

	// crdInstance.ObjectMeta.Annotations["cronJobId"] = cronJob.ID().String()

	// Batch job already exists - don't requeue
	log.V(1).Info("Skip reconcile: cron job already exists", "Namespace", existingItem.Namespace, "Name", existingItem.Name)

	if !crdInstance.ObjectMeta.DeletionTimestamp.IsZero() {

		// cleanup: deleting record from CloudFlare
		if controllerutil.ContainsFinalizer(crdInstance, FinalizerName) {
			// run batch job (not cron) with "delete" command for every record that has KeepAfterDelete == false
			var result *reconcile.Result
			result, err = r.ensureBatchJob(ctx, crdInstance, r.actualBatchJob(crdInstance, BatchJobName, DockerImageRunner, OperatorName))
			if result != nil {
				return *result, err
			}

			log.V(1).Info("DNS records deleted from CloudFlare")
		}

		return reconcile.Result{}, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DnsEntryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dnsentryv1.DnsEntry{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 10,
		}).
		Complete(r)
}

func (r *DnsEntryReconciler) handleFinalizer(ctx context.Context, obj dnsentryv1.DnsEntry, FinalizerName string) error {
	log := ctrllog.FromContext(ctx)

	if obj.ObjectMeta.DeletionTimestamp.IsZero() {
		// add finalizer in case of create/update
		if !controllerutil.ContainsFinalizer(&obj, FinalizerName) {
			ok := controllerutil.AddFinalizer(&obj, FinalizerName)
			log.V(1).Info(fmt.Sprintf("Add Finalizer %s : %t", FinalizerName, ok))
			return r.Update(ctx, &obj)
		}
	} else {
		// remove finalizer in case of deletion
		if controllerutil.ContainsFinalizer(&obj, FinalizerName) {
			ok := controllerutil.RemoveFinalizer(&obj, FinalizerName)
			log.V(1).Info(fmt.Sprintf("Remove Finalizer %s : %t", FinalizerName, ok))
			return r.Update(ctx, &obj)
		}
	}
	return nil
}
