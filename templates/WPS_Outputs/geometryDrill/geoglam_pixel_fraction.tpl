<wps:Output>
<ows:Identifier>veg_cover</ows:Identifier>
<ows:Title>Vegetation Cover</ows:Title>
<ows:Abstract>Time series data for Geoglam Fractional Cover.</ows:Abstract>
<wps:Data>
<wps:ComplexData mimeType="application/vnd.terriajs.catalog-member+json" schema="https://tools.ietf.org/html/rfc7159">
<![CDATA[{ "data": "date,pixel_frac\n{{ . }}", "isEnabled": true, "type": "csv", "name": "%s", "tableStyle": { "columns": { "pixel_frac": { "units": "%%", "chartLineColor": "#FFFFFF", "yAxisMin": 0, "yAxisMax": 100,  "active": true } } } }]]>
</wps:ComplexData>
</wps:Data>
</wps:Output>
