{
    "catalog": [
      {
         "type": "group",
         "name": "{{ .Namespace }}",
         "preserveOrder": true,
         "isOpen": true,
         "items": [
          {{ range $index, $layer := .Layers }}
          {
              "name": "{{ $layer.Title }}",
              "type": "wms",
              "url": "{{ $layer.DataURL }}",
              "layers": "{{ $layer.Name }}",
              "linkedWcsUrl": "{{ $layer.DataURL }}",
              "linkedWcsCoverage": "{{ $layer.Name }}",
              "ignoreUnknownTileErrors": true,
              "supportsColorScaleRange": true,
              "opacity": 1.0,
              "colorScaleMinimum": 0.0,
              "colorScaleMaximum": 1.0,
          },{{ end }}
        ]
      }
  ]
}
