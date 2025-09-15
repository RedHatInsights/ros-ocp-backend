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
{{- if eq .Values.database.ros.host "internal" }}
{{- printf "%s-db-ros" (include "ros-ocp.fullname" .) }}
{{- else }}
{{- .Values.database.ros.host }}
{{- end }}
{{- end }}

{{/*
Get the database URL - returns complete postgresql connection string
*/}}
{{- define "ros-ocp.databaseUrl" -}}
{{- printf "postgresql://postgres:$(DB_PASSWORD)@%s:%s/%s?sslmode=%s" (include "ros-ocp.databaseHost" .) (.Values.database.ros.port | toString) .Values.database.ros.name .Values.database.ros.sslMode }}
{{- end }}

{{/*
Get the kruize database host - returns internal service name if "internal", otherwise returns the configured host
*/}}
{{- define "ros-ocp.kruizeDatabaseHost" -}}
{{- if eq .Values.database.kruize.host "internal" }}
{{- printf "%s-db-kruize" (include "ros-ocp.fullname" .) }}
{{- else }}
{{- .Values.database.kruize.host }}
{{- end }}
{{- end }}

{{/*
Get the sources database host - returns internal service name if "internal", otherwise returns the configured host
*/}}
{{- define "ros-ocp.sourcesDatabaseHost" -}}
{{- if eq .Values.database.sources.host "internal" }}
{{- printf "%s-db-sources" (include "ros-ocp.fullname" .) }}
{{- else }}
{{- .Values.database.sources.host }}
{{- end }}
{{- end }}

{{/*
Detect if running on OpenShift by checking for OpenShift-specific API resources
Returns true if OpenShift is detected, false otherwise
*/}}
{{- define "ros-ocp.isOpenShift" -}}
{{- if .Capabilities.APIVersions.Has "route.openshift.io/v1" -}}
true
{{- else -}}
false
{{- end -}}
{{- end }}

{{/*
Get OpenShift cluster domain dynamically
Returns the cluster's default route domain (e.g., "apps.mycluster.example.com")
STRICT MODE: Fails deployment if cluster domain cannot be detected
Usage: {{ include "ros-ocp.clusterDomain" . }}
*/}}
{{- define "ros-ocp.clusterDomain" -}}
{{- /* Primary: Try to get domain from cluster ingress configuration */ -}}
{{- $ingressConfig := lookup "config.openshift.io/v1" "Ingress" "" "cluster" -}}
{{- if and $ingressConfig $ingressConfig.spec $ingressConfig.spec.domain -}}
{{- $ingressConfig.spec.domain -}}
{{- else -}}
{{- /* Secondary: Try to get domain from default ingress controller */ -}}
{{- $ingressController := lookup "operator.openshift.io/v1" "IngressController" "openshift-ingress-operator" "default" -}}
{{- if and $ingressController $ingressController.status $ingressController.status.domain -}}
{{- $ingressController.status.domain -}}
{{- else -}}
{{- /* Tertiary: Try to extract domain from any existing route */ -}}
{{- $routes := lookup "route.openshift.io/v1" "Route" "" "" -}}
{{- $clusterDomain := "" -}}
{{- if and $routes $routes.items -}}
{{- range $routes.items -}}
{{- if and .spec.host (contains "." .spec.host) -}}
{{- $hostParts := regexSplit "\\." .spec.host -1 -}}
{{- if gt (len $hostParts) 2 -}}
{{- $clusterDomain = join "." (slice $hostParts 1) -}}
{{- break -}}
{{- end -}}
{{- end -}}
{{- end -}}
{{- end -}}
{{- if $clusterDomain -}}
{{- $clusterDomain -}}
{{- else -}}
{{- /* STRICT MODE: Fail if cluster domain cannot be detected */ -}}
{{- fail "ERROR: Unable to detect OpenShift cluster domain. Ensure you are deploying to a properly configured OpenShift cluster with ingress controllers and routes. Dynamic detection failed for: config.openshift.io/v1/Ingress, operator.openshift.io/v1/IngressController, and existing Routes." -}}
{{- end -}}
{{- end -}}
{{- end -}}
{{- end }}

{{/*
Get OpenShift cluster name dynamically
Returns the cluster's infrastructure name (e.g., "mycluster-abcd1")
STRICT MODE: Fails deployment if cluster name cannot be detected
Usage: {{ include "ros-ocp.clusterName" . }}
*/}}
{{- define "ros-ocp.clusterName" -}}
{{- /* Primary: Try to get cluster name from Infrastructure resource */ -}}
{{- $infrastructure := lookup "config.openshift.io/v1" "Infrastructure" "" "cluster" -}}
{{- if and $infrastructure $infrastructure.status $infrastructure.status.infrastructureName -}}
{{- $infrastructure.status.infrastructureName -}}
{{- else -}}
{{- /* Secondary: Try to get cluster name from ClusterVersion */ -}}
{{- $clusterVersion := lookup "config.openshift.io/v1" "ClusterVersion" "" "version" -}}
{{- if and $clusterVersion $clusterVersion.spec $clusterVersion.spec.clusterID -}}
{{- printf "cluster-%s" (substr 0 8 $clusterVersion.spec.clusterID) -}}
{{- else -}}
{{- /* STRICT MODE: Fail if cluster name cannot be detected */ -}}
{{- fail "ERROR: Unable to detect OpenShift cluster name. Ensure you are deploying to a properly configured OpenShift cluster. Dynamic detection failed for: config.openshift.io/v1/Infrastructure and config.openshift.io/v1/ClusterVersion resources." -}}
{{- end -}}
{{- end -}}
{{- end }}

{{/*
Generate external URL for a service based on deployment platform (OpenShift Routes vs Kubernetes Ingress)
Usage: {{ include "ros-ocp.externalUrl" (list . "service-name" "/path") }}
*/}}
{{- define "ros-ocp.externalUrl" -}}
{{- $root := index . 0 -}}
{{- $service := index . 1 -}}
{{- $path := index . 2 -}}
{{- if eq (include "ros-ocp.isOpenShift" $root) "true" -}}
{{- /* OpenShift: Use Route configuration */ -}}
{{- $scheme := "http" -}}
{{- if $root.Values.serviceRoute.tls.termination -}}
{{- $scheme = "https" -}}
{{- end -}}
{{- with (index $root.Values.serviceRoute.hosts 0) -}}
{{- if .host -}}
{{- printf "%s://%s%s" $scheme .host $path -}}
{{- else -}}
{{- printf "%s://%s-%s.%s%s" $scheme $service $root.Release.Namespace (include "ros-ocp.clusterDomain" $root) $path -}}
{{- end -}}
{{- end -}}
{{- else -}}
{{- /* Kubernetes: Use Ingress configuration */ -}}
{{- $scheme := "http" -}}
{{- if $root.Values.serviceIngress.tls -}}
{{- $scheme = "https" -}}
{{- end -}}
{{- with (index $root.Values.serviceIngress.hosts 0) -}}
{{- printf "%s://%s%s" $scheme .host $path -}}
{{- end -}}
{{- end -}}
{{- end }}

{{/*
Detect appropriate volume mode based on actual storage class provisioner
Returns "Block" for block storage, "Filesystem" for filesystem storage
Usage: {{ include "ros-ocp.volumeMode" . }}
*/}}
{{- define "ros-ocp.volumeMode" -}}
{{- $storageClass := include "ros-ocp.databaseStorageClass" . -}}
{{- include "ros-ocp.volumeModeForStorageClass" (list . $storageClass) -}}
{{- end }}

{{/*
Get storage class name - validates user-defined storage class exists, falls back to default
Handles dry-run mode gracefully, fails deployment only if no suitable storage class is found during actual installation
Usage: {{ include "ros-ocp.storageClass" . }}
*/}}
{{- define "ros-ocp.storageClass" -}}
{{- $storageClasses := lookup "storage.k8s.io/v1" "StorageClass" "" "" -}}

{{- $userDefinedClass := "" -}}
{{- if and .Values.global.storageClass (ne .Values.global.storageClass "") -}}
{{- $userDefinedClass = .Values.global.storageClass -}}
{{- end -}}

{{- /* Handle dry-run mode or cluster connectivity issues */ -}}
{{- if not (and $storageClasses $storageClasses.items) -}}
{{- if $userDefinedClass -}}
{{- /* In dry-run mode, trust the user-defined storage class */ -}}
{{- $userDefinedClass -}}
{{- else -}}
{{- /* In dry-run mode with no explicit storage class, use a reasonable default */ -}}
{{- if eq (include "ros-ocp.isOpenShift" .) "true" -}}
ocs-storagecluster-ceph-rbd
{{- else -}}
standard
{{- end -}}
{{- end -}}
{{- else -}}
{{- /* Normal operation - query cluster for available storage classes */ -}}
{{- $defaultFound := "" -}}
{{- $userClassExists := false -}}
{{- range $storageClasses.items -}}
{{- if and .metadata.annotations (eq (index .metadata.annotations "storageclass.kubernetes.io/is-default-class") "true") -}}
{{- $defaultFound = .metadata.name -}}
{{- end -}}
{{- if and $userDefinedClass (eq .metadata.name $userDefinedClass) -}}
{{- $userClassExists = true -}}
{{- end -}}
{{- end -}}

{{- if $userDefinedClass -}}
{{- if $userClassExists -}}
{{- $userDefinedClass -}}
{{- else -}}
{{- if $defaultFound -}}
{{- printf "# Warning: Storage class '%s' not found, using default '%s' instead" $userDefinedClass $defaultFound | println -}}
{{- $defaultFound -}}
{{- else -}}
{{- fail (printf "Storage class '%s' not found and no default storage class available. Available storage classes: %s" $userDefinedClass (join ", " (pluck "metadata.name" $storageClasses.items))) -}}
{{- end -}}
{{- end -}}
{{- else -}}
{{- if $defaultFound -}}
{{- $defaultFound -}}
{{- else -}}
{{- fail (printf "No default storage class found in cluster. Available storage classes: %s\nPlease either:\n1. Set a default storage class with 'storageclass.kubernetes.io/is-default-class=true' annotation, or\n2. Explicitly specify a storage class with 'global.storageClass'" (join ", " (pluck "metadata.name" $storageClasses.items))) -}}
{{- end -}}
{{- end -}}
{{- end -}}
{{- end }}

{{/*
Get storage class for database workloads - uses same logic as main storage class
Only uses default storage class or user-defined, no fallbacks
Usage: {{ include "ros-ocp.databaseStorageClass" . }}
*/}}
{{- define "ros-ocp.databaseStorageClass" -}}
{{- include "ros-ocp.storageClass" . -}}
{{- end }}

{{/*
Detect volume mode by querying the actual StorageClass provisioner
Falls back to safe defaults if storage class cannot be found (e.g., during dry-run)
Usage: {{ include "ros-ocp.volumeModeForStorageClass" (list . "storage-class-name") }}
*/}}
{{- define "ros-ocp.volumeModeForStorageClass" -}}
{{- $root := index . 0 -}}
{{- $storageClassName := index . 1 -}}
{{- $storageClass := lookup "storage.k8s.io/v1" "StorageClass" "" $storageClassName -}}
{{- if $storageClass -}}
{{- $provisioner := $storageClass.provisioner -}}
{{- if or (contains "no-provisioner" $provisioner) (contains "local" $provisioner) -}}
{{- /* Local storage usually requires checking the actual PV */ -}}
Block
{{- else if or (contains "rbd" $provisioner) (contains "ceph-rbd" $provisioner) -}}
Filesystem
{{- else if or (contains "ebs" $provisioner) (contains "disk" $provisioner) -}}
Filesystem
{{- else if or (contains "nfs" $provisioner) (contains "rgw" $provisioner) (contains "bucket" $provisioner) -}}
Filesystem
{{- else -}}
Filesystem
{{- end -}}
{{- else -}}
{{- /* Storage class not found - use safe defaults based on known patterns */ -}}
{{- if or (contains "local" $storageClassName) (contains "no-provisioner" $storageClassName) -}}
Block
{{- else if or (contains "rbd" $storageClassName) (contains "ceph" $storageClassName) -}}
Filesystem
{{- else -}}
{{- /* Default to Filesystem for most cloud storage and during dry-run */ -}}
Filesystem
{{- end -}}
{{- end -}}
{{- end }}