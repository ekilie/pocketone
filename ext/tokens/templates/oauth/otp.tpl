{{template "head" .}}
<h1>Check your email</h1><p class="sub">We sent a code to {{.Email}}</p>
{{if .Error}}<p class="err">{{.Error}}</p>{{end}}
<form method="post" action="{{.Action}}">{{template "hidden" .}}
<input type="hidden" name="collection" value="{{.Collection}}"><input type="hidden" name="email" value="{{.Email}}"><input type="hidden" name="otp_id" value="{{.OTPId}}">{{if .MFAId}}<input type="hidden" name="mfa_id" value="{{.MFAId}}">{{end}}
<label for="code">Code</label><input id="code" type="text" name="code" inputmode="numeric" autocomplete="one-time-code" required autofocus>
<button class="btn" name="step" value="otp">Continue</button></form>{{template "foot" .}}
