<wps:Output>
<ows:Identifier>veg_cover</ows:Identifier>
<ows:Title>Vegetation Cover</ows:Title>
<ows:Abstract>Time series data for Geoglam Fractional Cover.</ows:Abstract>
<wps:Data>
<wps:ComplexData mimeType="application/vnd.terriajs.catalog-member+json" schema="https://tools.ietf.org/html/rfc7159">
<![CDATA[{ "data": "date,PV,PV_min,PV_max,NPV,NPV_min,NPV_max,BS,BS_min,BS_max,Total,Total_min,Total_max\n{{ . }}", "isEnabled": true, "type": "csv", "name": "%s", "tableStyle": { "columns": { "PV": { "units": "%%", "chartLineColor": "#0070c0", "yAxisMin": 0, "yAxisMax": 100, "active": true },"PV_min": { "units": "%%", "chartLineColor": "#005490", "yAxisMin": 0, "yAxisMax": 100, "active": false },"PV_max": { "units": "%%", "chartLineColor": "#003860", "yAxisMin": 0, "yAxisMax": 100, "active": false },"NPV": { "units": "%%", "chartLineColor": "#00b050", "yAxisMin": 0, "yAxisMax": 100, "active": true },"NPV_min": { "units": "%%", "chartLineColor": "#00843c", "yAxisMin": 0, "yAxisMax": 100, "active": false },"NPV_max": { "units": "%%", "chartLineColor": "#005828", "yAxisMin": 0, "yAxisMax": 100, "active": false },"BS": { "units": "%%", "chartLineColor": "#FF0000", "yAxisMin": 0, "yAxisMax": 100, "active": true },"BS_min": { "units": "%%", "chartLineColor": "#c00000", "yAxisMin": 0, "yAxisMax": 100, "active": false },"BS_max": { "units": "%%", "chartLineColor": "#810000", "yAxisMin": 0, "yAxisMax": 100, "active": false },"Total": { "units": "%%", "chartLineColor": "#FFFFFF", "yAxisMin": 0, "yAxisMax": 100, "active": true },"Total_min": { "units": "%%", "chartLineColor": "#c0c0c0", "yAxisMin": 0, "yAxisMax": 100, "active": false },"Total_max": { "units": "%%", "chartLineColor": "#818181", "yAxisMin": 0, "yAxisMax": 100, "active": false } } } }]]>
</wps:ComplexData>
</wps:Data>
</wps:Output>
