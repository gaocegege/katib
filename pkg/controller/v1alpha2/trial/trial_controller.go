/*
Copyright 2019 The Kubernetes Authors.

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

package trial

import (
	"bytes"
	"context"

	batchv1beta "k8s.io/api/batch/v1beta1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	trialsv1alpha2 "github.com/kubeflow/katib/pkg/api/operators/apis/trial/v1alpha2"
	commonv1alpha2 "github.com/kubeflow/katib/pkg/common/v1alpha2"
	"github.com/kubeflow/katib/pkg/controller/v1alpha2/trial/managerclient"
	controllerutil "github.com/kubeflow/katib/pkg/controller/v1alpha2/util"
)

var (
	log = logf.Log.WithName("trial-controller")
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Trial Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	r := &ReconcileTrial{
		Client:        mgr.GetClient(),
		scheme:        mgr.GetScheme(),
		ManagerClient: managerclient.New(),
	}
	r.updateStatusHandler = r.updateStatus
	return r
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("trial-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		log.Error(err, "Create trial controller error")
		return err
	}

	// Watch for changes to Trial
	err = c.Watch(&source.Kind{Type: &trialsv1alpha2.Trial{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		log.Error(err, "Trial watch error")
		return err
	}

	// Watch for changes to Cronjob
	err = c.Watch(
		&source.Kind{Type: &batchv1beta.CronJob{}},
		&handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &trialsv1alpha2.Trial{},
		})

	if err != nil {
		log.Error(err, "CronJob watch error")
		return err
	}

	for _, gvk := range commonv1alpha2.GetSupportedJobList() {
		unstructuredJob := &unstructured.Unstructured{}
		unstructuredJob.SetGroupVersionKind(gvk)
		err = c.Watch(
			&source.Kind{Type: unstructuredJob},
			&handler.EnqueueRequestForOwner{
				IsController: true,
				OwnerType:    &trialsv1alpha2.Trial{},
			})
		if err != nil {
			if meta.IsNoMatchError(err) {
				log.Info("Job watch error. CRD might be missing. Please install CRD and restart katib-controller", "CRD Kind", gvk.Kind)
				continue
			}
			return err
		} else {
			log.Info("Job watch added successfully", "CRD Kind", gvk.Kind)
		}
	}
	log.Info("Trial  controller created")
	return nil
}

var _ reconcile.Reconciler = &ReconcileTrial{}

// ReconcileTrial reconciles a Trial object
type ReconcileTrial struct {
	client.Client
	scheme *runtime.Scheme

	managerclient.ManagerClient
	// updateStatusHandler is defined for test purpose.
	updateStatusHandler updateStatusFunc
}

// Reconcile reads that state of the cluster for a Trial object and makes changes based on the state read
// and what is in the Trial.Spec
// +kubebuilder:rbac:groups=trials.kubeflow.org,resources=trials,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=trials.kubeflow.org,resources=trials/status,verbs=get;update;patch
func (r *ReconcileTrial) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the Trial instance
	logger := log.WithValues("Trial", request.NamespacedName)
	original := &trialsv1alpha2.Trial{}
	err := r.Get(context.TODO(), request.NamespacedName, original)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		logger.Error(err, "Trial Get error")
		return reconcile.Result{}, err
	}

	instance := original.DeepCopy()

	if needUpdate, finalizers := controllerutil.NeedUpdateFinalizers(instance, instance.Spec.RetainRun); needUpdate {
		return r.updateFinalizers(instance, finalizers)
	}

	if !instance.IsCreated() {
		if instance.Status.StartTime == nil {
			now := metav1.Now()
			instance.Status.StartTime = &now
		}
		if instance.Status.CompletionTime == nil {
			instance.Status.CompletionTime = &metav1.Time{}
		}
		err = r.CreateTrialInDB(instance)
		if err != nil {
			logger.Error(err, "Create trial in DB error")
			return reconcile.Result{
				Requeue: true,
			}, err
		}
		msg := "Trial is created"
		instance.MarkTrialStatusCreated(TrialCreatedReason, msg)
	} else {
		// Trial already created in DB
		err := r.reconcileTrial(instance)
		if err != nil {
			logger.Error(err, "Reconcile trial error")
			return reconcile.Result{}, err
		}
	}

	if !equality.Semantic.DeepEqual(original.Status, instance.Status) {
		//assuming that only status change
		err = r.UpdateTrialStatusInDB(instance)
		if err != nil {
			logger.Error(err, "Update trial status in DB error")
			return reconcile.Result{}, err
		}
		err = r.updateStatusHandler(instance)
		if err != nil {
			logger.Error(err, "Update trial instance status error")
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileTrial) reconcileTrial(instance *trialsv1alpha2.Trial) error {

	var err error
	logger := log.WithValues("Trial", types.NamespacedName{Name: instance.GetName(), Namespace: instance.GetNamespace()})
	desiredJob, err := r.getDesiredJobSpec(instance)
	if err != nil {
		logger.Error(err, "Job Spec Get error")
		return err
	}
	desiredMetricsCollector, err := r.getDesiredMetricsCollectorSpec(instance)
	if err != nil {
		logger.Error(err, "Metrics Collector Get error")
		return err
	}

	deployedJob, err := r.reconcileJob(instance, desiredJob)
	if err != nil {
		logger.Error(err, "Reconcile job error")
		return err
	}
	_, err = r.reconcileMetricsCollector(instance, desiredMetricsCollector)
	if err != nil {
		logger.Error(err, "Reconcile Metrics Collector error")
		return err
	}

	//Job already exists
	//TODO Can desired Spec differ from deployedSpec?
	if deployedJob != nil {
		if err = r.UpdateTrialStatusObservation(instance, deployedJob); err != nil {
			logger.Error(err, "Update trial status observation error")
			return err
		}
		// Update Trial job status only if observation field is available.
		// This will ensure that trial is set to be complete only if metric is collected at least once
		if isTrialObservationAvailable(instance) {
			if err = r.UpdateTrialStatusCondition(instance, deployedJob); err != nil {
				logger.Error(err, "Update trial status condition error")
				return err
			}
		}
	}
	return nil
}

func (r *ReconcileTrial) reconcileJob(instance *trialsv1alpha2.Trial, desiredJob *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	var err error
	logger := log.WithValues("Trial", types.NamespacedName{Name: instance.GetName(), Namespace: instance.GetNamespace()})
	apiVersion := desiredJob.GetAPIVersion()
	kind := desiredJob.GetKind()
	gvk := schema.FromAPIVersionAndKind(apiVersion, kind)

	deployedJob := &unstructured.Unstructured{}
	deployedJob.SetGroupVersionKind(gvk)
	err = r.Get(context.TODO(), types.NamespacedName{Name: desiredJob.GetName(), Namespace: desiredJob.GetNamespace()}, deployedJob)
	if err != nil {
		if errors.IsNotFound(err) {
			if instance.IsCompleted() {
				return nil, nil
			}
			logger.Info("Creating Job", "kind", kind,
				"name", desiredJob.GetName())
			err = r.Create(context.TODO(), desiredJob)
			if err != nil {
				logger.Error(err, "Create job error")
				return nil, err
			}
		} else {
			logger.Error(err, "Trial Get error")
			return nil, err
		}
	} else {
		if instance.IsCompleted() && !instance.Spec.RetainRun {
			if err = r.Delete(context.TODO(), desiredJob, client.PropagationPolicy(metav1.DeletePropagationForeground)); err != nil {
				logger.Error(err, "Delete job error")
				return nil, err
			} else {
				return nil, nil
			}
		}
	}

	msg := "Trial is running"
	instance.MarkTrialStatusRunning(TrialRunningReason, msg)
	return deployedJob, nil
}

func (r *ReconcileTrial) getDesiredJobSpec(instance *trialsv1alpha2.Trial) (*unstructured.Unstructured, error) {

	bufSize := 1024
	logger := log.WithValues("Trial", types.NamespacedName{Name: instance.GetName(), Namespace: instance.GetNamespace()})
	buf := bytes.NewBufferString(instance.Spec.RunSpec)

	desiredJobSpec := &unstructured.Unstructured{}
	if err := k8syaml.NewYAMLOrJSONDecoder(buf, bufSize).Decode(desiredJobSpec); err != nil {
		logger.Error(err, "Yaml decode error")
		return nil, err
	}
	if err := controllerutil.SetControllerReference(instance, desiredJobSpec, r.scheme); err != nil {
		logger.Error(err, "Set controller reference error")
		return nil, err
	}

	return desiredJobSpec, nil
}

func (r *ReconcileTrial) getDesiredMetricsCollectorSpec(instance *trialsv1alpha2.Trial) (*batchv1beta.CronJob, error) {
	mcjob := &batchv1beta.CronJob{}
	bufSize := 1024
	logger := log.WithValues("Metrics collector for Trial", types.NamespacedName{Name: instance.GetName(), Namespace: instance.GetNamespace()})
	buf := bytes.NewBufferString(instance.Spec.MetricsCollectorSpec)

	if err := k8syaml.NewYAMLOrJSONDecoder(buf, bufSize).Decode(mcjob); err != nil {
		logger.Error(err, "Yaml decode error")
		return nil, err
	}
	if err := controllerutil.SetControllerReference(instance, mcjob, r.scheme); err != nil {
		logger.Error(err, "Set controller reference error")
		return nil, err
	}
	return mcjob, nil
}

func (r *ReconcileTrial) reconcileMetricsCollector(instance *trialsv1alpha2.Trial,
	desiredMetricsCollector *batchv1beta.CronJob) (*batchv1beta.CronJob, error) {
	var err error
	logger := log.WithValues("Trial", types.NamespacedName{Name: instance.GetName(), Namespace: instance.GetNamespace()})

	deployedMetricsCollector := &batchv1beta.CronJob{}
	err = r.Get(context.TODO(), types.NamespacedName{
		Name:      desiredMetricsCollector.GetName(),
		Namespace: desiredMetricsCollector.GetNamespace(),
	}, deployedMetricsCollector)
	if err != nil {
		if errors.IsNotFound(err) {
			if instance.IsCompleted() {
				return nil, nil
			}
			logger.Info("Creating Metrics Collector",
				"name", desiredMetricsCollector.GetName(),
				"namespace", desiredMetricsCollector.GetNamespace())
			err = r.Create(context.TODO(), desiredMetricsCollector)
			if err != nil {
				logger.Error(err, "Create Metrics Collector error")
				return nil, err
			}
		} else {
			logger.Error(err, "Metrics Collector Get error")
			return nil, err
		}
	} else {
		if instance.IsCompleted() && !instance.Spec.RetainMetricsCollector {
			if err = r.Delete(context.TODO(), desiredMetricsCollector, client.PropagationPolicy(metav1.DeletePropagationForeground)); err != nil {
				logger.Error(err, "Delete Metrics Collector error")
				return nil, err
			} else {
				return nil, nil
			}
		}
	}

	return deployedMetricsCollector, nil
}
