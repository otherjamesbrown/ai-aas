/*
Copyright The Kubernetes Authors.

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

package controllers

import (
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	aimodelv1alpha1 "github.com/ai-aas/ai-model-operator/api/v1alpha1"
)

const (
	// DownloaderImage is the image used for the model downloader job
	DownloaderImage = "ai-model-operator/model-downloader:latest" // This will be built in T015
	// S3CredentialsSecretName is the name of the Kubernetes Secret containing S3 credentials
	S3CredentialsSecretName = "ai-aas-s3-credentials"
	// HuggingFaceTokenSecretName is the name of the Kubernetes Secret containing HuggingFace token
	HuggingFaceTokenSecretName = "ai-aas-huggingface-token"
)

// generateDownloaderJob creates a Kubernetes Job object for downloading model artifacts.
func generateDownloaderJob(aiModel *aimodelv1alpha1.AIModel) *batchv1.Job {
	backoffLimit := int32(4)

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      aiModel.Name + "-downloader",
			Namespace: aiModel.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name": "model-downloader",
				"aimodel.ai-aas.io/model-id": aiModel.Spec.ModelID,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{{
						Name:  "downloader",
						Image: DownloaderImage,
						Env: []corev1.EnvVar{
							{
								Name:  "MODEL_ID",
								Value: aiModel.Spec.ModelID,
							},
							{
								Name:  "MODEL_REVISION",
								Value: aiModel.Spec.Revision,
							},
							{
								Name: "S3_ENDPOINT",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{Name: S3CredentialsSecretName},
										Key:                  "endpoint",
									},
								},
							},
							{
								Name: "S3_BUCKET",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{Name: S3CredentialsSecretName},
										Key:                  "bucket",
									},
								},
							},
							{
								Name: "AWS_ACCESS_KEY_ID", // Used by AWS SDK
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{Name: S3CredentialsSecretName},
										Key:                  "accessKeyID",
									},
								},
							},
							{
								Name: "AWS_SECRET_ACCESS_KEY", // Used by AWS SDK
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{Name: S3CredentialsSecretName},
										Key:                  "secretAccessKey",
									},
								},
							},
							{
								Name: "HUGGINGFACE_TOKEN",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{Name: HuggingFaceTokenSecretName},
										Key:                  "token",
										Optional:             ptr.To(true),
									},
								},
							},
						},
						// TODO: Add resource limits and requests
					}},
				},
			},
		},
	}
}
