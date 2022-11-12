{{$el := (index .Order 0)}}{{$fi := (index .Runs $el)}}Latest PRESUBMIT run {{block "started" $fi}}started at <b>{{.Time}}{{with get . "Error"}} (:red_circle: FAILED :red_circle:){{end}}</b>{{end}}:<dl>
{{block "items" dict "Id" $el "Run" (index .Runs $el)}}
{{$cbp := "https://console.cloud.google.com/cloud-build/builds;region=global/"}}{{$cbs := "?project=xxxxxxxx"}}{{$bbp := "http://xxxx.xxxx.net/invocation/"}}

  <dt><b>cloudbuild:</b></dt>
  <dd><ul><li>{{block "link" dict "Url" (print $cbp .Id $cbs) "Text" "console"}}<kbd><a href="{{.Url}}" target="_blank">{{.Text}}</a></kbd>{{end}} {{template "link" dict "Url" (print $cbp .Id ";step=7" $cbs) "Text" "affected targets"}} {{template "link" dict "Url" (print $cbp .Id ";step=8" $cbs) "Text" "build log"}} {{template "link" dict "Url" (print $cbp .Id ";step=9" $cbs) "Text" "test log"}}</li></ul></dd>

  {{if or (hasKey .Run "BuildID") (hasKey .Run "TestID")}}<dt><b>buildbuddy:</b></dt><dd><ul>
    {{with get .Run "BuildID"}}<li>{{template "link" dict "Url" (print $bbp .) "Text" "build log"}} | {{block "mount" .}}mount with: <samp>enkit outputs mount -i {{.}}</samp>{{end}}</li>{{end}}
    {{with get .Run "TestID"}}<li>{{template "link" dict "Url" (print $bbp .) "Text" "test log"}} | {{template "mount" . }}</li>{{end}}
  </ul></dd>{{end}}

  {{if hasKey .Run "Error"}}<dt><b>:fire: Error</b></dt><dd><ul>
    <li>{{.Run.Error}} {{with get .Run "Link"}} {{template "link" dict "Url" . "Text" "suggestions for fixing"}}{{end}}</li>
    {{with get .Run "Hint"}}<li><b>Hint</b>: {{.}}</li>{{end}}
  </ul></dd>{{end}}
   
{{end}}
</dl>{{if ge (len .Order) 2}}<details>
<summary>Past runs</summary><p><ul>
{{range $i, $el := (slice .Order 1)}}{{if (index $.Runs $el)}}<li>
Run {{template "started" (index $.Runs $el)}}:<ul>
{{template "items"  dict "Id" $el "Run" (index $.Runs $el)}}
</ul>
</li>{{end}}{{end}}
</ul></p></details>{{end}}
<sub>Add a comment in this thread with the text <kbd>/<!---->gcbrun</kbd> to re-run the presubmit.
Do NOT use the retry buttons on cloudbuild or github.</sub>
{{if get . "Owners"}}{{if get .Owners "0"}}{{with get . "Owners"}}
<hr />
Possible review strategies (minimal sets of reviewers):<br><br>
<table><tr>
{{$owners := .}}
{{range $i, $e := until 5}}
  {{with get $owners (printf "%d" $e)}}<td>{{$e}}:{{range $k, $v := .}} {{block "username" $k}}{{template "link" dict "Url" (printf "https://www.github.com/%s" (.|replace "@" "")) "Text" (. | replace "@" "@<!---->")}}{{end}}{{end}}</td>{{end}}
{{end}}
</tr></table>{{$dwarning := false}}
<details><summary>Show Details</summary>
<ul>
{{range $i, $e := until 5}}
  {{with get $owners (printf "%d" $e)}}
<li>Option {{$e}}:<ul>
{{range $k, $v := .}}
<li>{{template "username" $k}}{{if not (or (get $v "fullname") (get $v "email"))}} {{$dwarning = true}}(missing - {{template "link" dict "Url" "https://xxxx.xxxx.net" "Text" "please add to directory"}}){{end}}{{with get $v "fullname"}} ({{.}}){{end}}{{with get $v "email"}} - {{.}}{{end}}:{{with get $v "files"}}<ul>
  {{range $_, $file := .}}<li>{{template "link" dict "Url" (printf "https://github.com/xxxx/yyyyy/blob/master/%s" .) "Text" .}}</li>{{end}}
</ul>{{end}}</li>
{{end}}
</ul></li>
  {{end}}
{{end}}
</ul>
</details>
{{if $dwarning}}<b>Warning:</b> the table above may be inaccurate - please {{template "link" dict "Url" "https://xxxx.xxxx.net" "Text" "add missing owners in the directory."}}{{end}}
{{end}}{{end}}{{end}}
<hr />
<b>Review PRs assigned to you</b> on {{template "link" dict "Url" "https://github.com/pulls/assigned" "Text" "your github page"}} | <b>Assign reviewers</b> with the "Assignees" field to the right :arrow_right: :arrow_right:
<br>
{{template "link" dict "Url" "https://xxxx.xxxx.net/" "Text" "buildbuddy home"}} | {{template "link" dict "Url" "https://console.cloud.google.com/cloud-build/builds?project=xxxxxxx" "Text" "CI/CD History"}} | {{template "link" dict "Url" "https://docs.google.com/xxxxxxx" "Text" "Code Review Guidelines"}} | {{template "link" dict "Url" "https://docs.google.com/xxxxx" "Text" "git problems?"}} | {{template "link" dict "Url" "https://docs.google.com/xxxxxx" "Text" "gee problems?"}} | {{template "link" dict "Url" "https://docs.google.com/xxxxxx" "Text" "bazel problems?"}}

