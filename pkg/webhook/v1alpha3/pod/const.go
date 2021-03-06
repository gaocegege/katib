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

package pod

const (
	// JobNameLabel represents the label key for the job name, the value is job name
	JobNameLabel = "job-name"
	// JobRoleLabel represents the label key for the job role, e.g. the value is master
	JobRoleLabel                 = "job-role"
	TFJobRoleLabel               = "tf-job-role"
	PyTorchJobRoleLabel          = "pytorch-job-role"
	MasterRole                   = "master"
	MetricsCollectorSidecar      = "metrics-collector-sidecar"
	MetricsCollectorSidecarImage = "image"

	PyTorchJob                    = "PyTorchJob"
	PyTorchJobWorkerContainerName = "pytorch"

	TFJob                    = "TFJob"
	TFJobWorkerContainerName = "tensorflow"

	BatchJob = "Job"
)

var JobRoleMap = map[string][]string{
	"TFJob":      {JobRoleLabel, TFJobRoleLabel},
	"PyTorchJob": {JobRoleLabel, PyTorchJobRoleLabel},
	"Job":        {},
}
