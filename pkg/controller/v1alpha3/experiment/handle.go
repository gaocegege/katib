package experiment

import (
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	experimentv1alpha2 "github.com/kubeflow/katib/pkg/api/operators/apis/experiment/v1alpha2"
	trialv1alpha2 "github.com/kubeflow/katib/pkg/api/operators/apis/trial/v1alpha2"
	"github.com/kubeflow/katib/pkg/controller/v1alpha3/util"
)

func (r *ReconcileExperiment) handle(
	experiment *experimentv1alpha2.Experiment) (result reconcile.Result, err error) {
	oldStatus := experiment.Status.DeepCopy()
	result = reconcile.Result{}
	logger := log.WithName(types.NamespacedName{
		Namespace: experiment.Namespace,
		Name:      experiment.Name,
	}.String())
	result = reconcile.Result{}

	defer func() {
		err = r.updateStatusIfChanged(experiment, oldStatus)
	}()

	trials, err := r.GetTrialsOwnedBy(experiment)
	if err != nil {
		return result, err
	}
	logger.V(0).Info("Getting trials owned by the experiment", "len(trials)", len(trials))

	failedTrials := r.FilterFailedTrials(trials)
	runningTrials := r.FilterRunningTrials(trials)
	succeededTrials := r.FilterSucceededTrials(trials)
	killedTrials := r.FilterKilledTrials(trials)

	lenTrials := len(trials)
	lenFailedTrials := len(failedTrials)
	lenRunningTrials := len(runningTrials)
	lenSucceededTrials := len(succeededTrials)
	lenKilledTrials := len(killedTrials)
	parallelCount := r.GetParallelTrials(trials)

	// Set all statistics.
	r.setStatistics(experiment, lenTrials, lenFailedTrials,
		lenRunningTrials, lenSucceededTrials, lenKilledTrials)

	// Update best trial.
	if lenSucceededTrials >= 0 {
		r.handleMetrics(experiment, succeededTrials)
	}

	// The job is completed, we mark it and return.
	if experiment.Spec.MaxTrialCount != nil {
		if lenTrials >= *experiment.Spec.MaxTrialCount {
			logger.V(0).Info("The experiment is completed")
			r.markCompleted(experiment)
			return result, err
		}
	}

	if experiment.Spec.MaxFailedTrialCount != nil {
		if lenFailedTrials >= *experiment.Spec.MaxFailedTrialCount {
			logger.V(0).Info("The experiment is failed")
			r.markFailed(experiment)
			return result, err
		}
	}

	// Create trials if the parallelCount < ParallelTrialCount
	if experiment.Spec.ParallelTrialCount != nil {
		if parallelCount < *experiment.Spec.ParallelTrialCount {
			requestNum := *experiment.Spec.ParallelTrialCount - parallelCount
			if err = r.handleTrials(experiment, trials, requestNum); err != nil {
				return result, err
			}
		}
	} else {
		logger.V(0).Info("The experiment does not define parallelTrialCount")
	}

	return result, err
}

func (r *ReconcileExperiment) handleTrials(experiment *experimentv1alpha2.Experiment,
	trials []trialv1alpha2.Trial, requestNum int) error {
	logger := log.WithName(types.NamespacedName{
		Namespace: experiment.Namespace,
		Name:      experiment.Name,
	}.String())

	newTrials, err := r.GetSuggestion(experiment, trials, requestNum)
	if err != nil {
		r.reportError(
			experiment, err, "Failed to get suggestions", util.FailReason)
		return err
	}
	if err = r.Initialize(experiment, newTrials); err != nil {
		r.reportError(
			experiment, err, "Failed to initialize trials", util.FailReason)
		return err
	}
	logger.V(0).Info("Some trials need to be created", "trials", newTrials)
	for i := range newTrials {
		if err = controllerutil.SetControllerReference(
			experiment, &newTrials[i], r.scheme); err != nil {
			r.reportError(experiment, err, "Fail to set owner", util.FailReason)
			return err
		}
		if err = r.CreateOrUpdateTrial(experiment, &newTrials[i]); err != nil {
			r.reportError(experiment, err,
				"Fail to create or update trial", util.FailReason)
			return err
		}
	}
	return nil
}
