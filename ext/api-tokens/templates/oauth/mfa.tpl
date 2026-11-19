{{template "head" .}}
{{if .Superuser}}<span class="badge">Superuser</span>{{end}}
<h1>One more step</h1><p class="sub">{{.AppName}} needs a second sign-in method for <b>{{.Email}}</b></p>
{{if .Error}}<p class="err">{{.Error}}</p>{{end}}
<form method="post" action="{{.Action}}">{{template "hidden" .}}<input type="hidden" name="collection" value="{{.Collection}}"><input type="hidden" name="mfa_id" value="{{.MFAId}}">
{{if .PasswordEnabled}}<label for="password">Password</label><input id="password" type="password" name="password" autocomplete="current-password" required autofocus>
<button class="btn" name="step" value="password">Continue</button>{{end}}
{{if .OTPEnabled}}<button class="{{if .PasswordEnabled}}link{{else}}btn{{end}}" name="step" value="request_otp" formnovalidate>Email me a sign-in code</button>{{end}}
{{range .Providers}}<button class="btn secondary" name="step" value="oauth2:{{.Name}}" formnovalidate>Continue with {{.DisplayName}}</button>{{end}}
<p class="alt"><button class="link" name="step" value="use:{{.Collection}}" formnovalidate>Start over</button></p>
</form>{{template "foot" .}}
