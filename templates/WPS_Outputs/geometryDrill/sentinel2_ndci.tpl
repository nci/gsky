<wps:Output>
<ows:Identifier>sentinel2_ndci</ows:Identifier>
<ows:Title>Sentinel2 NDCI</ows:Title>
<ows:Abstract>Time series data for Sentinel2 NDCI</ows:Abstract>
<wps:Data>
<wps:ComplexData mimeType="application/vnd.terriajs.catalog-member+json" schema="https://tools.ietf.org/html/rfc7159">
<![CDATA[{ "data": "date,sentinel2_ndci\n{{ . }}", "isEnabled": true, "type": "csv", "name": "%s", "tableStyle": { "columns": { "sentinel2_ndci": { "chartLineColor": "#ee0000", "yAxisMin": 0, "active": true } } } }]]>
</wps:ComplexData>
</wps:Data>
</wps:Output>
