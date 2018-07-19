<wps:Output>
<ows:Identifier>precipitation</ows:Identifier>
<ows:Title>Accumulated Precipitation</ows:Title>
<ows:Abstract>Time series data for CHIRPS2.0 accumulated precipitation.</ows:Abstract>
<wps:Data>
<wps:ComplexData mimeType="application/vnd.terriajs.catalog-member+json" schema="https://tools.ietf.org/html/rfc7159">
<![CDATA[{ "data": "date,Prec\n{{ . }}", "isEnabled": true, "type": "csv", "name": "Precipitation%s", "tableStyle": { "columns": { "Prec": { "units": "mm", "chartLineColor": "#72ecfa", "yAxisMin": 0, "active": true } } } }]]>
</wps:ComplexData>
</wps:Data>
</wps:Output>
