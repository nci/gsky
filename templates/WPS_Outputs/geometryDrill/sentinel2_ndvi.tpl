<wps:Output>
<ows:Identifier>sentinel2_ndvi</ows:Identifier>
<ows:Title>Sentinel2 NDVI</ows:Title>
<ows:Abstract>Time series data for Sentinel2 NDVI</ows:Abstract>
<wps:Data>
<wps:ComplexData mimeType="application/vnd.terriajs.catalog-member+json" schema="https://tools.ietf.org/html/rfc7159">
<![CDATA[{ "data": "date,sentinel2_ndvi\n{{ . }}", "isEnabled": true, "type": "csv", "name": "%s", "tableStyle": { "columns": { "sentinel2_ndvi": { "chartLineColor": "#00ff00", "active": true } } } }]]>
</wps:ComplexData>
</wps:Data>
</wps:Output>
