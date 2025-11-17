{{/*
Expand the name of the chart.
*/}}
{{- define "vllm-deployment.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "vllm-deployment.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{/*
Common labels
*/}}
{{- define "vllm-deployment.labels" -}}
helm.sh/chart: {{ include "vllm-deployment.chart" . }}
{{ include "vllm-deployment.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/environment: {{ .Values.environment | quote }}
{{- range $key, $value := .Values.labels }}
{{ $key }}: {{ $value | quote }}
{{- end }}
{{- end -}}

{{/*
Selector labels
*/}}
{{- define "vllm-deployment.selectorLabels" -}}
app.kubernetes.io/name: {{ include "vllm-deployment.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{/*
Chart label
*/}}
{{- define "vllm-deployment.chart" -}}
{{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" }}
{{- end -}}

{{/*
Service Account name
*/}}
{{- define "vllm-deployment.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "vllm-deployment.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{/*
Service endpoint format: {release-name}.{namespace}.svc.cluster.local:{port}
*/}}
{{- define "vllm-deployment.serviceEndpoint" -}}
{{- printf "%s.%s.svc.cluster.local:%d" (include "vllm-deployment.fullname" .) .Release.Namespace .Values.service.port -}}
{{- end -}}

