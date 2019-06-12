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

package experiment

import (
	"context"
	"os"

	"github.com/spf13/viper"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission/builder"

	experimentsv1alpha2 "github.com/kubeflow/katib/pkg/api/operators/apis/experiment/v1alpha2"
	trialsv1alpha2 "github.com/kubeflow/katib/pkg/api/operators/apis/trial/v1alpha2"
	"github.com/kubeflow/katib/pkg/controller/v1alpha2/consts"
	"github.com/kubeflow/katib/pkg/controller/v1alpha2/experiment/managerclient"
	"github.com/kubeflow/katib/pkg/controller/v1alpha2/experiment/manifest"
	"github.com/kubeflow/katib/pkg/controller/v1alpha2/experiment/suggestion"
	suggestionfake "github.com/kubeflow/katib/pkg/controller/v1alpha2/experiment/suggestion/fake"
	"github.com/kubeflow/katib/pkg/controller/v1alpha2/experiment/util"
	controllerutil "github.com/kubeflow/katib/pkg/controller/v1alpha2/util"
)

const katibControllerName = "katib-controller"

var log = logf.Log.WithName("experiment-controller")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Experiment Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	r := &ReconcileExperiment{
		Client:        mgr.GetClient(),
		scheme:        mgr.GetScheme(),
		ManagerClient: managerclient.New(),
	}
	imp := viper.GetString(consts.ConfigExperimentSuggestionName)
	r.Suggestion = newSuggestion(imp)

	r.Generator = manifest.New(r.Client)
	r.updateStatusHandler = r.updateStatus
	return r
}

// newSuggestion returns the new Suggestion for the given config.
func newSuggestion(config string) suggestion.Suggestion {
	// Use different implementation according to the configuration.
	switch config {
	case "fake":
		log.Info("Using the fake suggestion implementation")
		return suggestionfake.New()
	case "default":
		log.Info("Using the default suggestion implementation")
		return suggestion.New()
	default:
		log.Info("No valid name specified, using the default suggestion implementation",
			"implementation", config)
		return suggestion.New()
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("experiment-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		log.Error(err, "Failed to create experiment controller")
		return err
	}

	// Watch for changes to Experiment
	err = c.Watch(&source.Kind{Type: &experimentsv1alpha2.Experiment{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		log.Error(err, "Experiment watch failed")
		return err
	}

	// Watch for trials for the experiments
	err = c.Watch(
		&source.Kind{Type: &trialsv1alpha2.Trial{}},
		&handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &experimentsv1alpha2.Experiment{},
		})

	if err != nil {
		log.Error(err, "Trial watch failed")
		return err
	}
	if err = addWebhook(mgr); err != nil {
		log.Error(err, "Failed to create webhook")
		return err
	}

	log.Info("Experiment controller created")
	return nil
}

func addWebhook(mgr manager.Manager) error {
	mutatingWebhook, err := builder.NewWebhookBuilder().
		Name("mutating.experiment.kubeflow.org").
		Mutating().
		Operations(admissionregistrationv1beta1.Create, admissionregistrationv1beta1.Update).
		WithManager(mgr).
		ForType(&experimentsv1alpha2.Experiment{}).
		Handlers(&experimentDefaulter{}).
		Build()
	if err != nil {
		return err
	}
	validatingWebhook, err := builder.NewWebhookBuilder().
		Name("validating.experiment.kubeflow.org").
		Validating().
		Operations(admissionregistrationv1beta1.Create, admissionregistrationv1beta1.Update).
		WithManager(mgr).
		ForType(&experimentsv1alpha2.Experiment{}).
		Handlers(newExperimentValidator(mgr.GetClient())).
		Build()
	if err != nil {
		return err
	}
	as, err := webhook.NewServer("experiment-admission-server", mgr, webhook.ServerOptions{
		CertDir: "/tmp/cert",
		BootstrapOptions: &webhook.BootstrapOptions{
			Secret: &types.NamespacedName{
				Namespace: os.Getenv(experimentsv1alpha2.DefaultKatibNamespaceEnvName),
				Name:      katibControllerName,
			},
			Service: &webhook.Service{
				Namespace: os.Getenv(experimentsv1alpha2.DefaultKatibNamespaceEnvName),
				Name:      katibControllerName,
				Selectors: map[string]string{
					"app": katibControllerName,
				},
			},
			ValidatingWebhookConfigName: "experiment-validating-webhook-config",
			MutatingWebhookConfigName:   "experiment-mutating-webhook-config",
		},
	})
	if err != nil {
		return err
	}
	err = as.Register(mutatingWebhook, validatingWebhook)
	if err != nil {
		return err
	}
	return nil
}

var _ reconcile.Reconciler = &ReconcileExperiment{}

// ReconcileExperiment reconciles a Experiment object
type ReconcileExperiment struct {
	client.Client
	scheme *runtime.Scheme

	suggestion.Suggestion
	manifest.Generator
	managerclient.ManagerClient
	// updateStatusHandler is defined for test purpose.
	updateStatusHandler updateStatusFunc
}

// Reconcile reads that state of the cluster for a Experiment object and makes changes based on the state read
// and what is in the Experiment.Spec
// +kubebuilder:rbac:groups=experiments.kubeflow.org,resources=experiments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=experiments.kubeflow.org,resources=experiments/status,verbs=get;update;patch
func (r *ReconcileExperiment) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the Experiment instance
	logger := log.WithValues("Experiment", request.NamespacedName)
	original := &experimentsv1alpha2.Experiment{}
	err := r.Get(context.TODO(), request.NamespacedName, original)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		logger.Error(err, "Experiment Get error")
		return reconcile.Result{}, err
	}
	instance := original.DeepCopy()

	if needUpdate, finalizers := controllerutil.NeedUpdateFinalizers(instance, instance.Spec.RetainHistoricalData); needUpdate {
		return r.updateFinalizers(instance, finalizers)
	}

	if instance.IsCompleted() {
		return reconcile.Result{}, nil
	}
	if !instance.IsCreated() {
		//Experiment not created in DB
		if instance.Status.StartTime == nil {
			now := metav1.Now()
			instance.Status.StartTime = &now
		}
		if instance.Status.CompletionTime == nil {
			instance.Status.CompletionTime = &metav1.Time{}
		}
		msg := "Experiment is created"
		instance.MarkExperimentStatusCreated(util.ExperimentCreatedReason, msg)

		err = r.CreateExperimentInDB(instance)
		if err != nil {
			logger.Error(err, "Create experiment in DB error")
			return reconcile.Result{}, err
		}
	} else {
		// Experiment already created in DB
		err := r.ReconcileExperiment(instance)
		if err != nil {
			logger.Error(err, "Reconcile experiment error")
			return reconcile.Result{}, err
		}
	}

	if !equality.Semantic.DeepEqual(original.Status, instance.Status) {
		//assuming that only status change
		err = r.UpdateExperimentStatusInDB(instance)
		if err != nil {
			logger.Error(err, "Update experiment status in DB error")
			return reconcile.Result{}, err
		}
		err = r.updateStatusHandler(instance)
		if err != nil {
			logger.Error(err, "Update experiment instance status error")
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileExperiment) ReconcileExperiment(instance *experimentsv1alpha2.Experiment) error {

	logger := log.WithValues("Experiment", types.NamespacedName{Name: instance.GetName(), Namespace: instance.GetNamespace()})
	trials := &trialsv1alpha2.TrialList{}
	labels := map[string]string{"experiment": instance.Name}
	lo := &client.ListOptions{}
	lo.MatchingLabels(labels).InNamespace(instance.Namespace)

	if err := r.List(context.TODO(), lo, trials); err != nil {
		logger.Error(err, "Trial List error")
		return err
	}
	if len(trials.Items) > 0 {
		if err := util.UpdateExperimentStatus(instance, trials); err != nil {
			logger.Error(err, "Update experiment status error")
			return err
		}
	}
	reconcileRequired := !instance.IsCompleted()
	if reconcileRequired {
		r.ReconcileTrials(instance)
	}
	return nil
}

func (r *ReconcileExperiment) ReconcileTrials(instance *experimentsv1alpha2.Experiment) error {

	logger := log.WithValues("Experiment", types.NamespacedName{Name: instance.GetName(), Namespace: instance.GetNamespace()})

	parallelCount := *instance.Spec.ParallelTrialCount
	activeCount := instance.Status.TrialsPending + instance.Status.TrialsRunning
	completedCount := instance.Status.TrialsSucceeded + instance.Status.TrialsFailed + instance.Status.TrialsKilled

	if activeCount > parallelCount {
		deleteCount := activeCount - parallelCount
		if deleteCount > 0 {
			//delete 'deleteCount' number of trails. Sort them?
			logger.Info("DeleteTrials", "deleteCount", deleteCount)
			if err := r.deleteTrials(instance, deleteCount); err != nil {
				logger.Error(err, "Delete trials error")
				return err
			}
		}

	} else if activeCount < parallelCount {
		var requiredActiveCount int32
		if instance.Spec.MaxTrialCount == nil {
			requiredActiveCount = parallelCount
		} else {
			requiredActiveCount = *instance.Spec.MaxTrialCount - completedCount
			if requiredActiveCount > parallelCount {
				requiredActiveCount = parallelCount
			}
		}

		addCount := requiredActiveCount - activeCount
		if addCount < 0 {
			logger.Info("Invalid setting", "requiredActiveCount", requiredActiveCount, "MaxTrialCount",
				*instance.Spec.MaxTrialCount, "CompletedCount", completedCount)
			addCount = 0
		}

		//skip if no trials need to be created
		if addCount > 0 {
			//create "addCount" number of trials
			logger.Info("CreateTrials", "addCount", addCount)
			if err := r.createTrials(instance, addCount); err != nil {
				logger.Error(err, "Create trials error")
				return err
			}
		}
	}

	return nil

}

func (r *ReconcileExperiment) createTrials(instance *experimentsv1alpha2.Experiment, addCount int32) error {

	logger := log.WithValues("Experiment", types.NamespacedName{Name: instance.GetName(), Namespace: instance.GetNamespace()})
	trials, err := r.GetSuggestions(instance, addCount)
	if err != nil {
		logger.Error(err, "Get suggestions error")
		return err
	}
	if len(trials) == 0 {
		// for some suggestion services, such as hyperband, it will stop generating new trial once some condition satisfied
		util.UpdateExperimentStatusCondition(instance, false, true)
	}
	for _, trial := range trials {
		if err = r.createTrialInstance(instance, trial); err != nil {
			logger.Error(err, "Create trial instance error", "trial", trial)
			continue
		}
	}
	return nil
}

func (r *ReconcileExperiment) deleteTrials(instance *experimentsv1alpha2.Experiment, deleteCount int32) error {

	return nil
}
