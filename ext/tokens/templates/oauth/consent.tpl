{{template "head" .}}
{{if .Superuser}}<span class="badge">Superuser</span>{{end}}
<h1>{{.ClientName}}</h1><p class="sub">wants to access {{.AppName}} as <b>{{.Email}}</b></p>
{{if .Superuser}}<p class="warn"><b>Superuser account.</b> {{.ClientName}} will act as a superuser, with full control of {{.AppName}}: every record, collection and setting.</p>
{{else}}<p class="msg">User account ({{.Collection}}). {{.ClientName}} can do what this user can.</p>{{end}}
{{if .Error}}<p class="err">{{.Error}}</p>{{end}}
<form method="post" action="{{.Action}}">{{template "hidden" .}}
{{if .ScopeChoices}}<ul class="scopes choose">
<li><label><input type="radio" name="access" value="full"{{if .FullAccess}} checked{{end}}><span><b>Full access</b><span>Everything this account can do.</span></span></label></li>
<li><label><input type="radio" name="access" value="limited"{{if not .FullAccess}} checked{{end}}><span><b>Only these</b><span>Tick what {{.ClientName}} may do:</span></span></label>
<ul>{{range .ScopeChoices}}<li><label><input type="checkbox" name="grant" value="{{.Name}}"{{if .Checked}} checked{{end}}><span><b>{{.Name}}</b><span>{{.Description}}</span></span></label></li>{{end}}</ul></li>
</ul>
{{else if .FullAccess}}<ul class="scopes"><li><b>Full access</b><span>Everything this account can do.</span></li></ul>
{{else}}<ul class="scopes">{{range .Scopes}}<li><b>{{.Name}}</b><span>{{.Description}}</span></li>{{end}}</ul>{{end}}
{{if .ClientHost}}<p class="msg">You'll be sent back to {{.ClientHost}}.</p>{{end}}
<button class="btn" name="step" value="allow">Allow</button>
<button class="btn secondary" name="step" value="deny">Deny</button>
<button class="link" name="step" value="switch">Not {{.Email}}? Use another account</button></form>{{template "foot" .}}
