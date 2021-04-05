{
    "layers": [
        {{ range $index, $layer := .Layers -}}
        {{ if $index -}},{{- end }}
        {
            "title": "{{ $layer.Title }}",
            "name": "{{ $layer.Name }}",
            "data_source": "{{ $layer.DataSource }}",
            "rgb_products": [
              {{- range $ip, $var := $layer.RGBProducts -}}
              {{ if $ip -}},{{- end }}
              "{{ $var }}"
              {{- end }}
            ],
            "time_generator": "mas"
            {{- if $layer.AxesInfo }},{{ end -}}
            {{ if $layer.AxesInfo }}
            "axes": [
              {{- range $ia, $axis := $layer.AxesInfo -}}
              {{ if $ia -}},{{- end }}
              { "name": "{{ $axis.Name }}", "default": "{{ $axis.Default }}",
                "values": [ {{- range $iv, $val := $axis.Values }}{{ if $iv -}},{{- end }}"{{ $val }}"{{ end }} ]
              }
              {{- end }}
            ]
            {{- end }}
        }
        {{- end }}
    ]
}
