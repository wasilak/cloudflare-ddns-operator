package controller

import (
	"context"
	"encoding/json"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	dnsentriesv1 "github.com/wasilak/cloudflare-ddns-operator/api/v1"
)

// https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#job-v1-batch

func (r *DnsEntryReconciler) ensureBatchJob(ctx context.Context, instance *dnsentriesv1.DnsEntry, newUpdatedItem *batchv1.Job) (*reconcile.Result, error) {

	log := ctrllog.FromContext(ctx)

	// See if it already exists and create if it doesn't
	item := &batchv1.Job{}
	_ = r.Get(ctx, types.NamespacedName{
		Name:      newUpdatedItem.Name,
		Namespace: instance.Namespace,
	}, item)

	log.V(1).Info("Creating new Batch Job", "Job.Namespace", newUpdatedItem.Namespace, "Job.Name", newUpdatedItem.Name)

	err := r.Create(ctx, newUpdatedItem)
	if err != nil {
		return &reconcile.Result{}, err
	}

	return nil, nil
}

func (r *DnsEntryReconciler) actualBatchJob(item *dnsentriesv1.DnsEntry, name string, dockerImage string, dnsEntrySecretName string) *batchv1.Job {

	var jsonEncodedDnsEntries []byte

	jsonEncodedDnsEntries, err := json.Marshal(item.Spec.Items)
	if err != nil {
		jsonEncodedDnsEntries = []byte("{}")
	}

	parralelism := int32(1)
	ttlSecondsAfterFinished := int32(60)

	cronJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s-delete", name, item.Name),
			Namespace: item.Namespace,
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: &ttlSecondsAfterFinished,
			Parallelism:             &parralelism,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{{
						Image:           dockerImage,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Name:            fmt.Sprintf("%s-%s-delete", name, item.Name),
						Command: []string{
							"/bin/cloudflare-ddns",
						},
						Args: []string{
							"delete",
						},

						Env: []corev1.EnvVar{
							{
								Name:  "CFDDNS_RECORDS",
								Value: string(jsonEncodedDnsEntries),
							},
							{
								Name: "CF_API_KEY",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: dnsEntrySecretName,
										},
										Key: "CF_API_KEY",
									},
								},
							},
							{
								Name: "CF_API_EMAIL",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: dnsEntrySecretName,
										},
										Key: "CF_API_EMAIL",
									},
								},
							},
						},
					}},
				},
			},
		},
	}

	return cronJob
}
