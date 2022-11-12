Build reports for the latest run ({{(index .Run 0).Time}}) available here:
{{range (index .Run 0).Links}}* [{{.Description}}]({{.Link}})
{{end}}{{$length := len .Run}}{{if ge $length 1 }}
<details>
  <summary>Past runs</summary>
  <p>
    <ul>
{{range $i, $e := (slice .Run 1)}}<li>{{.Time}} -{{range .Links}} <a href="{{.Link}}">{{.Description}}</a>{{end}}</li>
{{end}}</ul>
  </p>
</details>{{end}}
