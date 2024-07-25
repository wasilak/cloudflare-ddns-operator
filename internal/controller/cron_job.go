package controller

import (
	"context"
	"encoding/json"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/cloudflare/cloudflare-go"
	dnsentriesv1 "github.com/wasilak/cloudflare-ddns-operator/api/v1"
)

// https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#job-v1-batch

func (r *DnsEntryReconciler) ensureCronJob(ctx context.Context, instance *dnsentriesv1.DnsEntry, newUpdatedItem *batchv1.CronJob) (*reconcile.Result, error) {

	log := ctrllog.FromContext(ctx)

	// See if it already exists and create if it doesn't
	item := &batchv1.CronJob{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      newUpdatedItem.Name,
		Namespace: instance.Namespace,
	}, item)
	if err != nil && errors.IsNotFound(err) {

		log.V(1).Info("Creating new Cron Job", "Job.Namespace", newUpdatedItem.Namespace, "Job.Name", newUpdatedItem.Name)

		err = r.Create(ctx, newUpdatedItem)

		if err != nil {
			return &reconcile.Result{}, err
		} else {
			return nil, nil
		}

	} else if err != nil {
		return &reconcile.Result{}, err
	}

	// update item
	r.Update(ctx, newUpdatedItem)

	return nil, nil
}

func (r *DnsEntryReconciler) actualCronJob(item *dnsentriesv1.DnsEntry, records []cloudflare.DNSRecord, jobName string, dockerImage string, dnsEntrySecretName string) *batchv1.CronJob {

	var jsonEncodedDnsEntries []byte

	jsonEncodedDnsEntries, err := json.Marshal(records)
	if err != nil {
		jsonEncodedDnsEntries = []byte("{}")
	}

	failedJobsHistoryLimit := int32(1)
	successfulJobsHistoryLimit := int32(1)

	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", jobName, item.Name),
			Namespace: item.Namespace,
		},
		Spec: batchv1.CronJobSpec{
			Schedule:                   item.Spec.Cron,
			FailedJobsHistoryLimit:     &failedJobsHistoryLimit,
			SuccessfulJobsHistoryLimit: &successfulJobsHistoryLimit,
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							RestartPolicy: corev1.RestartPolicyOnFailure,
							Containers: []corev1.Container{{
								Image:           dockerImage,
								ImagePullPolicy: corev1.PullIfNotPresent,
								Name:            fmt.Sprintf("%s-%s", jobName, item.Name),
								Command: []string{
									"/bin/cloudflare-ddns",
								},
								Args: []string{
									"oneoff",
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
									{
										Name: "CFDDNS_MAIL_ENABLED",
										ValueFrom: &corev1.EnvVarSource{
											SecretKeyRef: &corev1.SecretKeySelector{
												LocalObjectReference: corev1.LocalObjectReference{
													Name: dnsEntrySecretName,
												},
												Key: "CFDDNS_MAIL_ENABLED",
											},
										},
									},
									{
										Name: "CFDDNS_MAIL_TO",
										ValueFrom: &corev1.EnvVarSource{
											SecretKeyRef: &corev1.SecretKeySelector{
												LocalObjectReference: corev1.LocalObjectReference{
													Name: dnsEntrySecretName,
												},
												Key: "CFDDNS_MAIL_TO",
											},
										},
									},
									{
										Name: "CFDDNS_MAIL_FROM",
										ValueFrom: &corev1.EnvVarSource{
											SecretKeyRef: &corev1.SecretKeySelector{
												LocalObjectReference: corev1.LocalObjectReference{
													Name: dnsEntrySecretName,
												},
												Key: "CFDDNS_MAIL_FROM",
											},
										},
									},
									{
										Name: "CFDDNS_MAIL_AUTH_USERNAME",
										ValueFrom: &corev1.EnvVarSource{
											SecretKeyRef: &corev1.SecretKeySelector{
												LocalObjectReference: corev1.LocalObjectReference{
													Name: dnsEntrySecretName,
												},
												Key: "CFDDNS_MAIL_AUTH_USERNAME",
											},
										},
									},
									{
										Name: "CFDDNS_MAIL_AUTH_PASSWORD",
										ValueFrom: &corev1.EnvVarSource{
											SecretKeyRef: &corev1.SecretKeySelector{
												LocalObjectReference: corev1.LocalObjectReference{
													Name: dnsEntrySecretName,
												},
												Key: "CFDDNS_MAIL_AUTH_PASSWORD",
											},
										},
									},
								},
							}},
						},
					},
				},
			},
		},
	}

	controllerutil.SetControllerReference(item, cronJob, r.Scheme)

	return cronJob
}
