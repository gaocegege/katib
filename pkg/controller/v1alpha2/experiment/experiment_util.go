package experiment

import (
	"bytes"
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	experimentsv1alpha2 "github.com/kubeflow/katib/pkg/api/operators/apis/experiment/v1alpha2"
	suggestionsv1alpha2 "github.com/kubeflow/katib/pkg/api/operators/apis/suggestions/v1alpha2"
	trialsv1alpha2 "github.com/kubeflow/katib/pkg/api/operators/apis/trial/v1alpha2"
	"github.com/kubeflow/katib/pkg/util/v1alpha2/helper"
)

func (r *ReconcileExperiment) createTrialInstance(
	expInstance *experimentsv1alpha2.Experiment,
	suggestion *suggestionsv1alpha2.Suggestion,
	assignment *suggestionsv1alpha2.TrialAssignment) error {
	BUFSIZE := 1024
	logger := log.WithValues("Experiment", types.NamespacedName{Name: expInstance.GetName(), Namespace: expInstance.GetNamespace()})

	trial := &trialsv1alpha2.Trial{}
	trial.Name = *assignment.Name
	trial.Namespace = expInstance.GetNamespace()
	trial.Labels = helper.TrialLabels(expInstance)

	if err := controllerutil.SetControllerReference(expInstance, trial, r.scheme); err != nil {
		logger.Error(err, "Set controller reference error")
		return err
	}

	trial.Spec.Objective = expInstance.Spec.Objective

	hps := assignment.Assignments

	runSpec, err := r.GetRunSpecWithHyperParameters(
		expInstance, expInstance.GetName(), trial.Name, trial.Namespace, hps)
	if err != nil {
		logger.Error(err, "Fail to get RunSpec from experiment", expInstance.Name)
		return err
	}

	trial.Spec.RunSpec = runSpec

	buf := bytes.NewBufferString(runSpec)
	job := &unstructured.Unstructured{}
	if err := k8syaml.NewYAMLOrJSONDecoder(buf, BUFSIZE).Decode(job); err != nil {
		return fmt.Errorf("Invalid spec.trialTemplate: %v.", err)
	}

	var metricNames []string
	metricNames = append(metricNames, expInstance.Spec.Objective.ObjectiveMetricName)
	for _, mn := range expInstance.Spec.Objective.AdditionalMetricNames {
		metricNames = append(metricNames, mn)
	}

	mcSpec, err := r.GetMetricsCollectorManifest(expInstance.GetName(), trial.Name, job.GetKind(), trial.Namespace, metricNames, expInstance.Spec.MetricsCollectorSpec)
	if err != nil {
		logger.Error(err, "Error getting metrics collector manifest")
		return err
	}
	trial.Spec.MetricsCollectorSpec = mcSpec

	if expInstance.Spec.TrialTemplate != nil {
		trial.Spec.RetainRun = expInstance.Spec.TrialTemplate.Retain
	}
	if expInstance.Spec.MetricsCollectorSpec != nil {
		trial.Spec.RetainMetricsCollector = expInstance.Spec.MetricsCollectorSpec.Retain
	}

	if err := r.Create(context.TODO(), trial); err != nil {
		logger.Error(err, "Trial create error", "Trial name", trial.Name)
		return err
	}
	return nil

}
