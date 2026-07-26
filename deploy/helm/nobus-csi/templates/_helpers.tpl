{{- define "nobus-csi.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "nobus-csi.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- include "nobus-csi.name" . -}}
{{- end -}}
{{- end -}}

{{- define "nobus-csi.namespace" -}}
{{- default .Release.Namespace .Values.namespaceOverride -}}
{{- end -}}

{{- define "nobus-csi.labels" -}}
app.kubernetes.io/name: {{ include "nobus-csi.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
{{- end -}}

{{- define "nobus-csi.controllerServiceAccountName" -}}
{{- if .Values.serviceAccount.controller.create -}}
{{- default (printf "%s-controller" (include "nobus-csi.fullname" .)) .Values.serviceAccount.controller.name -}}
{{- else -}}
{{- required "serviceAccount.controller.name is required when serviceAccount.controller.create=false" .Values.serviceAccount.controller.name -}}
{{- end -}}
{{- end -}}

{{- define "nobus-csi.nodeServiceAccountName" -}}
{{- if .Values.serviceAccount.node.create -}}
{{- default (printf "%s-node" (include "nobus-csi.fullname" .)) .Values.serviceAccount.node.name -}}
{{- else -}}
{{- required "serviceAccount.node.name is required when serviceAccount.node.create=false" .Values.serviceAccount.node.name -}}
{{- end -}}
{{- end -}}

{{- define "nobus-csi.secretName" -}}
{{- if .Values.nobus.existingSecret -}}
{{- .Values.nobus.existingSecret -}}
{{- else -}}
{{- default (include "nobus-csi.fullname" .) .Values.nobus.secretName -}}
{{- end -}}
{{- end -}}

{{- define "nobus-csi.controllerImage" -}}
{{- printf "%s:%s" .Values.image.repository .Values.image.controllerTag -}}
{{- end -}}

{{- define "nobus-csi.nodeImage" -}}
{{- printf "%s:%s" .Values.image.repository .Values.image.nodeTag -}}
{{- end -}}
