package suggestion

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	experimentv1alpha2 "github.com/kubeflow/katib/pkg/api/operators/apis/experiment/v1alpha2"
	trialv1alpha2 "github.com/kubeflow/katib/pkg/api/operators/apis/trial/v1alpha2"
)

var log = logf.Log.WithName("suggestion-service")

type SuggestionService interface {
	GetSuggestion(e *experimentv1alpha2.Experiment,
		trials []trialv1alpha2.Trial, requestNum int) ([]trialv1alpha2.Trial, error)
}

type GeneralSuggestionService struct {
	client.Client
}

func (g *GeneralSuggestionService) GetSuggestion(e *experimentv1alpha2.Experiment, trial []trialv1alpha2.Trial) error {
	return fmt.Errorf("Not implemented")
}
