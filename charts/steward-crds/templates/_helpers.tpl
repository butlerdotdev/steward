{{/*
Expand the name of the chart.
*/}}
{{- define "steward-crds.name" -}}
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
{{- define "steward-crds.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create the cert-manager annotation to inject Certificate CA.
*/}}
{{- define "steward-crds.certManagerAnnotation" -}}
{{- printf "%s/%s" (required "A valid .Values.stewardNamespace is required" .Values.stewardNamespace) (required "A valid .Values.stewardCertificateName is required" .Values.stewardCertificateName) }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "steward-crds.labels" -}}
helm.sh/chart: {{ include "steward-crds.chart" . }}
app.kubernetes.io/name: {{ include "steward-crds.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: "crds"
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}
