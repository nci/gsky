<wps:Output>
<ows:Identifier>precipitation</ows:Identifier>
<ows:Title>Accumulated Precipitation</ows:Title>
<ows:Abstract>Time series data for CHIRPS2.0 accumulated precipitation.</ows:Abstract>
<wps:Data>
<wps:ComplexData mimeType="application/vnd.terriajs.catalog-member+json" schema="https://tools.ietf.org/html/rfc7159">
<![CDATA[{ "data": "date,Prec,Prec_min,Prec_max\n{{ . }}", "isEnabled": true, "type": "csv", "name": "%s", "tableStyle": { "columns": { "Prec": { "units": "mm", "chartLineColor": "#72ecfa", "yAxisMin": 0, "active": true },"Prec_min": { "units": "mm", "chartLineColor": "#56b1bc", "yAxisMin": 0, "active": false },"Prec_max": { "units": "mm", "chartLineColor": "#3a767e", "yAxisMin": 0, "active": false } } } }]]>
</wps:ComplexData>
</wps:Data>
</wps:Output>
