{{- define "backend-assessment.fullname" -}}
{{- printf "%s-%s" .Chart.Name "server" | trunc 63 | trimSuffix "-" -}}
{{- end -}}


