package composer

import (
	"reflect"
	"testing"

	tfcommon "github.com/kubeflow/tf-operator/pkg/apis/common/v1beta1"
	tfv1beta1 "github.com/kubeflow/tf-operator/pkg/apis/tensorflow/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	trialv1alpha2 "github.com/kubeflow/katib/pkg/api/operators/apis/trial/v1alpha2"
)

func TestGetDesiredTFJob(t *testing.T) {
	c := New()
	testCases := []struct {
		CaseName      string
		Trial         *trialv1alpha2.Trial
		Expected      *tfv1beta1.TFJob
		ExpectedError bool
	}{
		{
			CaseName: "Normal case",
			Trial: &trialv1alpha2.Trial{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testName",
					Namespace: "testNameSpace",
				},
				Spec: trialv1alpha2.TrialSpec{
					RunSpec: `apiVersion: "kubeflow.org/v1beta1"
kind: "TFJob"
metadata:
    name: "dist-mnist-for-e2e-test"
spec:
    tfReplicaSpecs:
        Worker:
            template:
                spec:
                    containers:
                      - name: tensorflow
                        image: gaocegege/mnist:1`,
				},
			},
			Expected: &tfv1beta1.TFJob{
				Spec: tfv1beta1.TFJobSpec{
					TFReplicaSpecs: map[tfv1beta1.TFReplicaType]*tfcommon.ReplicaSpec{
						tfv1beta1.TFReplicaTypeWorker: &tfcommon.ReplicaSpec{
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{
										{
											Name:  "tensorflow",
											Image: "gaocegege/mnist:1",
										},
									},
								},
							},
						},
					},
				},
			},
			ExpectedError: false,
		},
		{
			CaseName: "Invalid image name case",
			Trial: &trialv1alpha2.Trial{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testName",
					Namespace: "testNameSpace",
				},
				Spec: trialv1alpha2.TrialSpec{
					RunSpec: `apiVersion: "kubeflow.org/v1beta1"
kind: "TFJob"
metadata:
    name: "dist-mnist-for-e2e-test"
spec:
    tfReplicaSpecs:
    Worker:
        template:
            spec:
                containers:
                - name: tensorflow
                                image: 1`,
				},
			},
			ExpectedError: true,
		},
	}

	for _, tc := range testCases {
		println(tc.Trial.Spec.RunSpec)
		actual, actualErr := c.GetDesiredTFJob(tc.Trial)
		if tc.ExpectedError {
			if actualErr == nil {
				t.Errorf("%s: Expected error, got nil", tc.CaseName)
			}
			continue
		}
		if actualErr != nil {
			t.Errorf("%s: Expected nil error, got %v", tc.CaseName, actualErr)
		}
		if !reflect.DeepEqual(actual.Spec.TFReplicaSpecs[tfv1beta1.TFReplicaTypeWorker].Template.Spec.Containers,
			tc.Expected.Spec.TFReplicaSpecs[tfv1beta1.TFReplicaTypeWorker].Template.Spec.Containers) {
			t.Errorf("%s: Expected TFJob %v, got %v", tc.CaseName,
				tc.Expected.Spec.TFReplicaSpecs[tfv1beta1.TFReplicaTypeWorker],
				actual.Spec.TFReplicaSpecs[tfv1beta1.TFReplicaTypeWorker])
		}
	}
}
