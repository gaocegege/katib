package suggestion

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"sigs.k8s.io/controller-runtime/pkg/client"

	experimentv1alpha2 "github.com/kubeflow/katib/pkg/api/operators/apis/experiment/v1alpha2"
	trialv1alpha2 "github.com/kubeflow/katib/pkg/api/operators/apis/trial/v1alpha2"
	grpcsuggestionv1alpha3 "github.com/kubeflow/katib/pkg/api/suggestion/v1alpha3"
	"github.com/kubeflow/katib/pkg/controller/v1alpha3/experiment/suggestion/composer"
)

const (
	testEndpoint = "0.0.0.0:6789"
)

// SemiSuggestionService is the suggestion service which is only for test purpose.
type SemiSuggestionService struct {
	client.Client
	composer.Composer
}

// NewSemi creates a new SemiSuggestionService.
func NewSemi(client client.Client, composer composer.Composer) SuggestionService {
	return &SemiSuggestionService{
		Client:   client,
		Composer: composer,
	}
}

// GetSuggestion gets suggestions from local suggestion server in `testEndpoint`.
func (s *SemiSuggestionService) GetSuggestion(e *experimentv1alpha2.Experiment,
	trials []trialv1alpha2.Trial, requestNum int) ([]trialv1alpha2.Trial, error) {
	logger := log.WithName(e.Name)
	endpoint := testEndpoint
	conn, err := grpc.Dial(endpoint, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client := grpcsuggestionv1alpha3.NewAdvisorSuggestionClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	request := &grpcsuggestionv1alpha3.GetAdvisorSuggestionsRequest{
		Experiment:    s.ConvertExperiment(e),
		Trials:        s.ConvertTrials(trials),
		RequestNumber: int32(requestNum),
	}
	response, err := client.GetSuggestions(ctx, request)
	logger.V(0).Info("Getting suggestions", "endpoint", endpoint, "response", response, "request", request)
	if err != nil {
		return nil, err
	}
	if len(response.Trials) == 0 {
		return nil, fmt.Errorf("The response contains 0 trials")
	}
	return s.ComposeTrialsTemplate(response.Trials), nil
}
