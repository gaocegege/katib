package initializer

import (
	"bytes"
	"fmt"
	"html/template"

	"k8s.io/apimachinery/pkg/types"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	experimentv1alpha2 "github.com/kubeflow/katib/pkg/api/operators/apis/experiment/v1alpha2"
	trialv1alpha2 "github.com/kubeflow/katib/pkg/api/operators/apis/trial/v1alpha2"
	"github.com/kubeflow/katib/pkg/controller/v1alpha3/util"
)

var log = logf.Log.WithName("experiment-suggestion-composer")

type Initializer interface {
	Initialize(e *experimentv1alpha2.Experiment, trials []trialv1alpha2.Trial) error
}

type GeneralInitializer struct {
}

func New() Initializer {
	return &GeneralInitializer{}
}

func (g *GeneralInitializer) Initialize(e *experimentv1alpha2.Experiment, trials []trialv1alpha2.Trial) error {
	t, err := g.getTrialTemplate(e)
	if err != nil {
		return err
	}
	for i := range trials {
		if err = g.initializeTrialInstance(e, &trials[i], t); err != nil {
			return err
		}
		trials[i].Spec.Objective = e.Spec.Objective.ObjectiveMetricName
		trials[i].Spec.Metrics = e.Spec.Objective.AdditionalMetricsNames
		trials[i].Spec.MetricsCollector = e.Spec.MetricsCollectorType
	}
	return nil
}

func (g *GeneralInitializer) initializeTrialInstance(expInstance *experimentv1alpha2.Experiment, trialInstance *trialv1alpha2.Trial, trialTemplate *template.Template) error {
	logger := log.WithValues("Experiment", types.NamespacedName{Name: expInstance.GetName(), Namespace: expInstance.GetNamespace()})

	trialInstance.Name = fmt.Sprintf("%s-%s", expInstance.GetName(), utilrand.String(8))
	trialInstance.Namespace = expInstance.GetNamespace()
	trialInstance.Labels = map[string]string{util.LabelExperiment: expInstance.GetName()}

	trialParams := TrialTemplateParams{}

	var buf bytes.Buffer
	if trialInstance.Spec.ParameterAssignments != nil {
		for _, p := range trialInstance.Spec.ParameterAssignments {
			trialParams.HyperParameters = append(trialParams.HyperParameters, p)
		}
	}
	if err := trialTemplate.Execute(&buf, trialParams); err != nil {
		logger.Error(err, "Template execute error")
		return err
	}

	trialInstance.Spec.RunSpec = buf.String()

	return nil

}

func (g *GeneralInitializer) getTrialTemplate(instance *experimentv1alpha2.Experiment) (*template.Template, error) {

	var err error
	var tpl *template.Template = nil
	logger := log.WithValues("Experiment", types.NamespacedName{Name: instance.GetName(), Namespace: instance.GetNamespace()})
	trialTemplate := instance.Spec.TrialTemplate
	if trialTemplate != nil && trialTemplate.GoTemplate.RawTemplate != "" {
		tpl, err = template.New("Trial").Parse(trialTemplate.GoTemplate.RawTemplate)
	}
	// TODO(gaocegege): Deal with config map here.
	if err != nil {
		logger.Error(err, "Template parse error")
		return nil, err
	}

	return tpl, nil
}

type TrialTemplateParams struct {
	HyperParameters []trialv1alpha2.ParameterAssignment
}
