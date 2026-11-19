{{template "head" .}}
<h1>Connect a device</h1><p class="sub">Enter the code shown on your other device.</p>
{{if .Error}}<p class="err">{{.Error}}</p>{{end}}
<form method="get" action="{{.Action}}">
<label for="user_code">Code</label><input id="user_code" class="code" type="text" name="user_code" value="{{.UserCode}}" placeholder="XXXX-XXXX" autocomplete="off" required autofocus>
<button class="btn">Continue</button></form>{{template "foot" .}}
