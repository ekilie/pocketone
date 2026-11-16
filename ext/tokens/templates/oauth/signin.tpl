{{template "head" .}}
{{if .Superuser}}<span class="badge">Superuser</span><h1>Superuser sign-in</h1>
<p class="sub">to {{.AppName}}{{if .ClientName}}, to continue to {{.ClientName}}{{end}}</p>
<p class="warn">A superuser has full control of {{.AppName}}. Only sign in here if you mean to give {{if .ClientName}}{{.ClientName}}{{else}}this app{{end}} that power.</p>
{{else}}<h1>Sign in to {{.AppName}}</h1>
<p class="sub">{{if .ClientName}}to continue to {{.ClientName}}{{else}}to continue{{end}}</p>{{end}}
{{if .Error}}<p class="err">{{.Error}}</p>{{end}}
<form method="post" action="{{.Action}}">{{template "hidden" .}}<input type="hidden" name="collection" value="{{.Collection}}">
{{if gt (len .Collections) 1}}<div class="tabs">{{range .Collections}}<button class="tab{{if eq . $.Collection}} on{{end}}" name="step" value="use:{{.}}" formnovalidate>{{.}}</button>{{end}}</div>{{end}}
{{if or .PasswordEnabled .OTPEnabled}}<label for="email">{{.IdentityLabel}}</label><input id="email" type="{{if eq .IdentityLabel "Email"}}email{{else}}text{{end}}" name="email" value="{{.Email}}" autocomplete="username" required autofocus>{{end}}
{{if .PasswordEnabled}}<label for="password">Password</label><input id="password" type="password" name="password" autocomplete="current-password">
<button class="btn" name="step" value="password">Sign in</button>{{end}}
{{if .OTPEnabled}}<button class="{{if .PasswordEnabled}}link{{else}}btn{{end}}" name="step" value="request_otp" formnovalidate>Email me a sign-in code</button>{{end}}
{{if .Providers}}{{if or .PasswordEnabled .OTPEnabled}}<p class="or">or</p>{{end}}
{{range .Providers}}<button class="btn secondary" name="step" value="oauth2:{{.Name}}" formnovalidate>Continue with {{.DisplayName}}</button>{{end}}{{end}}
{{if .SuperuserLink}}<p class="alt"><button class="link" name="step" value="use:_superusers" formnovalidate>Sign in as a superuser</button></p>{{end}}
{{if .UserLink}}<p class="alt"><button class="link" name="step" value="use:" formnovalidate>Sign in as a user instead</button></p>{{end}}
</form>{{template "foot" .}}
