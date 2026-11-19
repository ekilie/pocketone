{{template "head" .}}
<h1>{{if .Error}}Not connected{{else}}All set{{end}}</h1><p class="msg">{{if .Error}}{{.Error}}{{else}}{{.Message}}{{end}}</p>{{template "foot" .}}
