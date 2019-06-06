package util

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	CleanDataFinalizer = "kubeflow.org/clean-data-in-db"
)

func NeedUpdateFinalizers(o metav1.Object, retainHistoricalData bool) (bool, []string) {
	deleted := !o.GetDeletionTimestamp().IsZero()
	pendingFinalizers := o.GetFinalizers()
	contained := false
	for _, elem := range pendingFinalizers {
		if elem == CleanDataFinalizer {
			contained = true
			break
		}
	}

	if !deleted && !contained {
		if retainHistoricalData {
			return false, []string{}
		}
		finalizers := append(pendingFinalizers, CleanDataFinalizer)
		return true, finalizers
	}
	if deleted && contained {
		finalizers := []string{}
		for _, pendingFinalizer := range pendingFinalizers {
			if pendingFinalizer != CleanDataFinalizer {
				finalizers = append(finalizers, pendingFinalizer)
			}
		}
		return true, finalizers
	}
	return false, []string{}
}
