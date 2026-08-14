{{- define "appforge.name" -}}appforge{{- end }}
{{- define "appforge.labels" -}}
app.kubernetes.io/name: {{ include "appforge.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}
