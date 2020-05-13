<wps:Output>
<ows:Identifier>sentinel2_ndwi</ows:Identifier>
<ows:Title>Sentinel2 NDWI</ows:Title>
<ows:Abstract>Time series data for Sentinel2 NDWI</ows:Abstract>
<wps:Data>
<wps:ComplexData mimeType="application/vnd.terriajs.catalog-member+json" schema="https://tools.ietf.org/html/rfc7159">
<![CDATA[{ "data": "date,sentinel2_ndwi\n{{ . }}", "isEnabled": true, "type": "csv", "name": "%s", "tableStyle": { "columns": { "sentinel2_ndwi": { "chartLineColor": "#72ecff", "active": true } } } }]]>
</wps:ComplexData>
</wps:Data>
</wps:Output>
