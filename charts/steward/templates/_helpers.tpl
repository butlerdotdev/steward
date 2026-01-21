{{/*
Expand the name of the chart.
*/}}
{{- define "steward.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "steward.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "steward.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "steward.labels" -}}
helm.sh/chart: {{ include "steward.chart" . }}
{{ include "steward.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "steward.selectorLabels" -}}
app.kubernetes.io/name: {{ default (include "steward.name" .) .name }}
app.kubernetes.io/instance: {{ default .Release.Name .instance }}
app.kubernetes.io/component: {{ default "controller-manager" .component }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "steward.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "steward.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the Service to user for webhooks
*/}}
{{- define "steward.webhookServiceName" -}}
{{- printf "%s-webhook-service" (include "steward.fullname" .) }}
{{- end }}

{{/*
Create the name of the Service to user for metrics
*/}}
{{- define "steward.metricsServiceName" -}}
{{- printf "%s-metrics-service" (include "steward.fullname" .) }}
{{- end }}

{{/*
Create the name of the cert-manager secret
*/}}
{{- define "steward.webhookSecretName" -}}
{{- printf "%s-webhook-server-cert" (include "steward.fullname" .) }}
{{- end }}

{{/*
Create the name of the cert-manager Certificate
*/}}
{{- define "steward.certificateName" -}}
{{- printf "%s-serving-cert" (include "steward.fullname" .) }}
{{- end }}


{{/*
Kubeconfig Generator Deployment name.
*/}}
{{- define "steward.kubeconfigGeneratorName" -}}
{{- if .Values.kubeconfigGenerator.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name "kubeconfig-generator" | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
