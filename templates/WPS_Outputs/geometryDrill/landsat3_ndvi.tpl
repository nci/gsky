<wps:Output>
<ows:Identifier>landsat3_ndvi</ows:Identifier>
<ows:Title>Landsat3 NDVI</ows:Title>
<ows:Abstract>Time series data for Landsat3 NDVI</ows:Abstract>
<wps:Data>
<wps:ComplexData mimeType="application/vnd.terriajs.catalog-member+json" schema="https://tools.ietf.org/html/rfc7159">
<![CDATA[{ "data": "date,landsat3_ndvi\n{{ . }}", "isEnabled": true, "type": "csv", "name": "%s", "tableStyle": { "columns": { "landsat3_ndvi": { "chartLineColor": "#00ff50", "yAxisMin": -1.2, "yAxisMax": 1.2,  "active": true } } } }]]>
</wps:ComplexData>
</wps:Data>
</wps:Output>
