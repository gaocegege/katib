package clientset

import (
	"context"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	experimentv1alpha2 "github.com/kubeflow/katib/pkg/api/operators/apis/experiment/v1alpha2"
	trialv1alpha2 "github.com/kubeflow/katib/pkg/api/operators/apis/trial/v1alpha2"
	"github.com/kubeflow/katib/pkg/controller/v1alpha3/recorder"
	"github.com/kubeflow/katib/pkg/controller/v1alpha3/util"
)

var log = logf.Log.WithName("experiment-trial-clientset")

type Trial interface {
	GetParallelTrials(trials []trialv1alpha2.Trial) int
	FilterRunningTrials(
		trials []trialv1alpha2.Trial) []trialv1alpha2.Trial
	FilterKilledTrials(
		trials []trialv1alpha2.Trial) []trialv1alpha2.Trial
	FilterFailedTrials(
		trials []trialv1alpha2.Trial) []trialv1alpha2.Trial
	FilterSucceededTrials(
		trials []trialv1alpha2.Trial) []trialv1alpha2.Trial
	GetTrialsOwnedBy(e *experimentv1alpha2.Experiment) ([]trialv1alpha2.Trial, error)
	CreateOrUpdateTrial(e *experimentv1alpha2.Experiment, trial *trialv1alpha2.Trial) error
}

type GeneralTrialClient struct {
	client.Client
	recorder.Recorder
}

// New creates a new GeneralTrialClient.
func New(c client.Client, r recorder.Recorder) Trial {
	return &GeneralTrialClient{
		Client:   c,
		Recorder: r,
	}
}

func (g *GeneralTrialClient) CreateOrUpdateTrial(e *experimentv1alpha2.Experiment, trial *trialv1alpha2.Trial) error {
	found := &trialv1alpha2.Trial{}
	err := g.Get(context.TODO(), types.NamespacedName{
		Name:      trial.Name,
		Namespace: trial.Namespace,
	}, found)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating TFJob", "namespace", trial.Namespace, "name", trial.Name)
		err = g.Create(context.TODO(), trial)
		if err != nil {
			return err
		}
		g.ReportChange(e, util.FlagCreate, util.TypeTrial)
		return nil
	}

	// Update the found object and write the result back if there are any changes.
	if !reflect.DeepEqual(trial.Spec, found.Spec) {
		found.Spec = trial.Spec
		log.Info("Updating Trial", "namespace", trial.Namespace, "name", trial.Name)
		err = g.Update(context.TODO(), trial)
		if err != nil {
			return err
		}
		g.ReportChange(e, util.FlagUpdate, util.TypeTrial)
	}
	trial.Status = found.Status
	return nil
}

// GetParallelTrials get the number of parallel trials now.
func (g *GeneralTrialClient) GetParallelTrials(trials []trialv1alpha2.Trial) int {
	return len(trials) -
		len(g.FilterFailedTrials(trials)) -
		len(g.FilterKilledTrials(trials)) -
		len(g.FilterSucceededTrials(trials))
}

// GetTrialsOwnedBy returns the trials owned by the experiment.
func (g *GeneralTrialClient) GetTrialsOwnedBy(
	e *experimentv1alpha2.Experiment) ([]trialv1alpha2.Trial, error) {
	actual := &trialv1alpha2.TrialList{}

	requirement, err := labels.NewRequirement(util.LabelExperiment,
		selection.Equals, []string{
			e.Name,
		})
	if err != nil {
		return nil, err
	}
	if err = g.List(context.TODO(), &client.ListOptions{
		LabelSelector: labels.NewSelector().Add(*requirement)}, actual); err != nil {
		return nil, err
	}

	return actual.Items, nil
}

// FilterKilledTrials returns the killed trials owned by the experiment.
func (g *GeneralTrialClient) FilterKilledTrials(
	trials []trialv1alpha2.Trial) []trialv1alpha2.Trial {
	return filterTrials(trials, isTrialKilled)
}

// FilterSucceededTrials returns the succeeded trials owned by the experiment.
func (g *GeneralTrialClient) FilterSucceededTrials(
	trials []trialv1alpha2.Trial) []trialv1alpha2.Trial {
	return filterTrials(trials, isTrialSucceeded)
}

// FilterRunningTrials returns the running trials owned by the experiment.
func (g *GeneralTrialClient) FilterRunningTrials(
	trials []trialv1alpha2.Trial) []trialv1alpha2.Trial {
	return filterTrials(trials, isTrialRunning)
}

// FilterFailedTrials returns the failed trials owned by the experiment.
func (g *GeneralTrialClient) FilterFailedTrials(
	trials []trialv1alpha2.Trial) []trialv1alpha2.Trial {
	return filterTrials(trials, isTrialFailed)
}

type filterFunc func(t *trialv1alpha2.Trial) bool

func filterTrials(trials []trialv1alpha2.Trial,
	f filterFunc) []trialv1alpha2.Trial {
	actual := make([]trialv1alpha2.Trial, 0)
	for _, t := range trials {
		if f(&t) {
			actual = append(actual, t)
		}
	}
	return actual
}

func isTrialKilled(t *trialv1alpha2.Trial) bool {
	return ifTrialHaveCondition(t, trialv1alpha2.TrialKilled, corev1.ConditionTrue)
}

func isTrialSucceeded(t *trialv1alpha2.Trial) bool {
	return ifTrialHaveCondition(t, trialv1alpha2.TrialSucceeded, corev1.ConditionTrue)
}

func isTrialRunning(t *trialv1alpha2.Trial) bool {
	return ifTrialHaveCondition(t, trialv1alpha2.TrialRunning, corev1.ConditionTrue)
}

func isTrialFailed(t *trialv1alpha2.Trial) bool {
	return ifTrialHaveCondition(t, trialv1alpha2.TrialFailed, corev1.ConditionTrue)
}

func ifTrialHaveCondition(t *trialv1alpha2.Trial,
	condType trialv1alpha2.TrialConditionType, boolVal corev1.ConditionStatus) bool {
	for _, c := range t.Status.Conditions {
		if c.Type == condType && c.Status == boolVal {
			return true
		}
	}
	return false
}
