package experiment

import (
	experimentv1alpha2 "github.com/kubeflow/katib/pkg/api/operators/apis/experiment/v1alpha2"
	trialv1alpha2 "github.com/kubeflow/katib/pkg/api/operators/apis/trial/v1alpha2"
	"k8s.io/apimachinery/pkg/types"
)

// handleMetrics sets the CurrentOptimalTrial.
func (r *ReconcileExperiment) handleMetrics(e *experimentv1alpha2.Experiment,
	trials []trialv1alpha2.Trial) {
	logger := log.WithName(types.NamespacedName{
		Namespace: e.Namespace,
		Name:      e.Name,
	}.String())
	if len(trials) == 0 {
		return
	}
	for _, t := range trials {
		if t.Status.Observation == nil || t.Status.Observation.Objective == nil {
			continue
		}
		// Set e.Status.CurrentOptimalTrial.Observation.Objective.Value first.
		if e.Status.CurrentOptimalTrial.Observation.Objective == nil {
			e.Status.CurrentOptimalTrial.TrialName = t.Name
			e.Status.CurrentOptimalTrial.Observation =
				*t.Status.Observation.DeepCopy()
			e.Status.CurrentOptimalTrial.ParameterAssignments =
				t.Spec.ParameterAssignments
			continue
		}
		if cmp(e.Spec.Objective.Type,
			t.Status.Observation.Objective.Value,
			e.Status.CurrentOptimalTrial.Observation.Objective.Value) {
			logger.V(0).Info("Found a better trial", "metrics",
				t.Status.Observation.Objective.Value)
			e.Status.CurrentOptimalTrial.TrialName = t.Name
			e.Status.CurrentOptimalTrial.Observation =
				*t.Status.Observation.DeepCopy()
			e.Status.CurrentOptimalTrial.ParameterAssignments =
				t.Spec.ParameterAssignments
		}
	}
}

func cmp(typ experimentv1alpha2.ObjectiveType, m1, m2 float64) bool {
	switch typ {
	case experimentv1alpha2.ObjectiveTypeMinimize:
		return m1 < m2
	case experimentv1alpha2.ObjectiveTypeMaximize:
		return m1 > m2
	default:
		return false
	}
}
