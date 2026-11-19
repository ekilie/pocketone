{{range $k, $v := .Hidden}}<input type="hidden" name="{{$k}}" value="{{$v}}">{{end}}<input type="hidden" name="csrf" value="{{.CSRF}}">
