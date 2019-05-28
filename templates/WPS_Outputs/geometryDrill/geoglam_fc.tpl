<wps:Output>
<ows:Identifier>veg_cover</ows:Identifier>
<ows:Title>Vegetation Cover</ows:Title>
<ows:Abstract>Time series data for Geoglam Fractional Cover.</ows:Abstract>
<wps:Data>
<wps:ComplexData mimeType="application/vnd.terriajs.catalog-member+json" schema="https://tools.ietf.org/html/rfc7159">
<![CDATA[{ "data": "date,green,non-green,bare ground,total\n{{ . }}", "isEnabled": true, "type": "csv", "name": "%s", "tableStyle": { "columns": { "green": { "units": "%%", "chartLineColor": "#00b050", "yAxisMin": 0, "yAxisMax": 100, "active": true }, "non-green": { "units": "%%", "chartLineColor": "#0070c0", "yAxisMin": 0, "yAxisMax": 100, "active": true }, "bare ground": { "units": "%%", "chartLineColor": "#FF0000", "yAxisMin": 0, "yAxisMax": 100,  "active": true }, "total": { "units": "%%", "chartLineColor": "#FFFFFF", "yAxisMin": 0, "yAxisMax": 100,  "active": true } } } }]]>
</wps:ComplexData>
</wps:Data>
</wps:Output>
