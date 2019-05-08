package composer

import (
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	experimentv1alpha2 "github.com/kubeflow/katib/pkg/api/operators/apis/experiment/v1alpha2"
	trialv1alpha2 "github.com/kubeflow/katib/pkg/api/operators/apis/trial/v1alpha2"
	grpcsuggestionv1alpha3 "github.com/kubeflow/katib/pkg/api/suggestion/v1alpha3"
)

var log = logf.Log.WithName("experiment-suggestion-composer")

// Composer is the interface to convert structures between
// suggestion GRPC server and the controller.
type Composer interface {
	ConvertExperiment(e *experimentv1alpha2.Experiment) *grpcsuggestionv1alpha3.Experiment
	ConvertTrials(ts []trialv1alpha2.Trial) []*grpcsuggestionv1alpha3.Trial
	ComposeTrialsTemplate(t []*grpcsuggestionv1alpha3.Trial) []trialv1alpha2.Trial
}

// GeneralComposer is the default composer which implements Composer.
type GeneralComposer struct {
}

// New creates a new Composer.
func New() Composer {
	return &GeneralComposer{}
}

// ConvertExperiment converts CRD to the GRPC definition.
func (g *GeneralComposer) ConvertExperiment(e *experimentv1alpha2.Experiment) *grpcsuggestionv1alpha3.Experiment {
	res := &grpcsuggestionv1alpha3.Experiment{}
	res.Name = e.Name
	res.ExperimentSpec = &grpcsuggestionv1alpha3.ExperimentSpec{
		Algorithm: &grpcsuggestionv1alpha3.AlgorithmSpec{
			AlgorithmName:    e.Spec.Algorithm.AlgorithmName,
			AlgorithmSetting: convertAlgorithmSettings(e.Spec.Algorithm.AlgorithmSettings),
		},
		Objective: &grpcsuggestionv1alpha3.ObjectiveSpec{
			Type:                convertObjectiveType(e.Spec.Objective.Type),
			Goal:                *e.Spec.Objective.Goal,
			ObjectiveMetricName: e.Spec.Objective.ObjectiveMetricName,
		},
		ParameterSpecs: &grpcsuggestionv1alpha3.ParameterSpecs{
			Parameters: convertParameters(e.Spec.Parameters),
		},
	}
	return res
}

// ConvertTrials converts CRD to the GRPC definition.
func (g *GeneralComposer) ConvertTrials(
	t []trialv1alpha2.Trial) []*grpcsuggestionv1alpha3.Trial {
	res := make([]*grpcsuggestionv1alpha3.Trial, 0)
	return res
}

// ComposeTrialsTemplate composes trials with raw template from the GRPC response.
func (g *GeneralComposer) ComposeTrialsTemplate(ts []*grpcsuggestionv1alpha3.Trial) []trialv1alpha2.Trial {
	res := make([]trialv1alpha2.Trial, 0)
	for _, t := range ts {
		res = append(res, trialv1alpha2.Trial{
			Spec: trialv1alpha2.TrialSpec{
				ParameterAssignments: composeParameterAssignments(
					t.Spec.ParameterAssignments.Assignments),
			},
		})
	}
	return res
}

func composeParameterAssignments(pas []*grpcsuggestionv1alpha3.ParameterAssignment) []trialv1alpha2.ParameterAssignment {
	res := make([]trialv1alpha2.ParameterAssignment, 0)
	for _, pa := range pas {
		res = append(res, trialv1alpha2.ParameterAssignment{
			Name:  pa.Name,
			Value: pa.Value,
		})
	}
	return res
}

func convertObjectiveType(typ experimentv1alpha2.ObjectiveType) grpcsuggestionv1alpha3.ObjectiveType {
	switch typ {
	case experimentv1alpha2.ObjectiveTypeMaximize:
		return grpcsuggestionv1alpha3.ObjectiveType_MAXIMIZE
	case experimentv1alpha2.ObjectiveTypeMinimize:
		return grpcsuggestionv1alpha3.ObjectiveType_MINIMIZE
	default:
		return grpcsuggestionv1alpha3.ObjectiveType_UNKNOWN
	}
}

func convertAlgorithmSettings(as []experimentv1alpha2.AlgorithmSetting) []*grpcsuggestionv1alpha3.AlgorithmSetting {
	res := make([]*grpcsuggestionv1alpha3.AlgorithmSetting, 0)
	for _, s := range as {
		res = append(res, &grpcsuggestionv1alpha3.AlgorithmSetting{
			Name:  s.Name,
			Value: s.Value,
		})
	}
	return res
}

func convertParameters(ps []experimentv1alpha2.ParameterSpec) []*grpcsuggestionv1alpha3.ParameterSpec {
	res := make([]*grpcsuggestionv1alpha3.ParameterSpec, 0)
	for _, p := range ps {
		res = append(res, &grpcsuggestionv1alpha3.ParameterSpec{
			Name:          p.Name,
			ParameterType: convertParameterType(p.ParameterType),
			FeasibleSpace: convertFeasibleSpace(p.FeasibleSpace),
		})
	}
	return res
}

func convertParameterType(typ experimentv1alpha2.ParameterType) grpcsuggestionv1alpha3.ParameterType {
	switch typ {
	case experimentv1alpha2.ParameterTypeDiscrete:
		return grpcsuggestionv1alpha3.ParameterType_DISCRETE
	case experimentv1alpha2.ParameterTypeCategorical:
		return grpcsuggestionv1alpha3.ParameterType_CATEGORICAL
	case experimentv1alpha2.ParameterTypeDouble:
		return grpcsuggestionv1alpha3.ParameterType_DOUBLE
	case experimentv1alpha2.ParameterTypeInt:
		return grpcsuggestionv1alpha3.ParameterType_INT
	default:
		return grpcsuggestionv1alpha3.ParameterType_UNKNOWN_TYPE
	}
}

func convertFeasibleSpace(fs experimentv1alpha2.FeasibleSpace) *grpcsuggestionv1alpha3.FeasibleSpace {
	res := &grpcsuggestionv1alpha3.FeasibleSpace{
		Max:  fs.Max,
		Min:  fs.Min,
		List: fs.List,
		Step: fs.Step,
	}
	return res
}
