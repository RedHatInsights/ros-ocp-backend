{{/*
Expand the name of the chart.
*/}}
{{- define "ros-ocp.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "ros-ocp.fullname" -}}
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
{{- define "ros-ocp.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "ros-ocp.labels" -}}
helm.sh/chart: {{ include "ros-ocp.chart" . }}
{{ include "ros-ocp.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "ros-ocp.selectorLabels" -}}
app.kubernetes.io/name: {{ include "ros-ocp.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "ros-ocp.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "ros-ocp.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Get the database host - returns internal service name if "internal", otherwise returns the configured host
*/}}
{{- define "ros-ocp.databaseHost" -}}
{{- if eq .Values.rosocp.database.host "internal" }}
{{- printf "%s-db-ros" (include "ros-ocp.fullname" .) }}
{{- else }}
{{- .Values.rosocp.database.host }}
{{- end }}
{{- end }}

{{/*
Get the database URL - returns complete postgresql connection string
*/}}
{{- define "ros-ocp.databaseUrl" -}}
{{- printf "postgresql://postgres:$(DB_PASSWORD)@%s:%s/%s?sslmode=disable" (include "ros-ocp.databaseHost" .) (.Values.rosocp.database.port | toString) .Values.rosocp.database.name }}
{{- end }}