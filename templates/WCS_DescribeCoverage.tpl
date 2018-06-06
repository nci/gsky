<?xml version="1.0" encoding="UTF-8"?><CoverageDescription xmlns="http://www.opengis.net/wcs" xmlns:gml="http://www.opengis.net/gml" xmlns:xlink="http://www.w3.org/1999/xlink" version="1.0.0">
  <CoverageOffering>
    <description>
      {{ .Abstract }}
    </description>
    <name>{{ .Name }}</name>
    <label>
      {{ .Title }}
    </label>
    <lonLatEnvelope srsName="urn:ogc:def:crs:OGC:1.3:CRS84">
      <gml:pos>-180.0 -90.0</gml:pos>
      <gml:pos>180.0 90.0</gml:pos>
      <gml:timePosition>2015-01-06T01:54:08Z</gml:timePosition>
      <gml:timePosition>2015-12-24T01:54:38Z</gml:timePosition>
    </lonLatEnvelope>
    <domainSet>
      <spatialDomain>
        <EnvelopeWithTimePeriod srsName="urn:ogc:def:crs:OGC:1.3:CRS84">
          <gml:pos dimension="2">-180.0 -90.0</gml:pos>
          <gml:pos dimension="2">180.0 90.0</gml:pos>
          <gml:timePosition>1980-01-00T00:00:00Z</gml:timePosition>
          <gml:timePosition>2018-01-01T00:00:00Z</gml:timePosition>
        </EnvelopeWithTimePeriod>
        <gml:RectifiedGrid srsName="EPSG:4326" dimension="2">
          <gml:limits>
            <gml:GridEnvelope>
              <gml:low>0 0</gml:low>
              <gml:high>3999 3999</gml:high>
            </gml:GridEnvelope>
          </gml:limits>
          <gml:axisName>x</gml:axisName>
          <gml:axisName>y</gml:axisName>
          <gml:origin>
            <gml:pos>-999.9875000000001 -1400.0125</gml:pos>
          </gml:origin>
          <gml:offsetVector>0.025 0.0</gml:offsetVector>
          <gml:offsetVector>0.0 -0.025</gml:offsetVector>
        </gml:RectifiedGrid>
      </spatialDomain>
      <temporalDomain>
		{{ range $index, $value := .Dates }}
        <gml:timePosition>{{ $value }}</gml:timePosition>
        {{ end }}
      </temporalDomain>
    </domainSet>
    <rangeSet>
      <RangeSet>
        <description xmlns="">
          {{ .Abstract }}
        </description>
        <name>{{ .Name }}</name>
        <label>
          {{ .Title }}
        </label>
        <nullValues>
          <singleValue>NaN</singleValue>
        </nullValues>
      </RangeSet>
    </rangeSet>
    <supportedCRSs>
      <requestCRSs>EPSG:4326</requestCRSs>
      <responseCRSs>EPSG:4326</responseCRSs>
    </supportedCRSs>
    <supportedFormats>
      <formats>GeoTIFF</formats>
      <formats>NetCDF</formats>
    </supportedFormats>
    <supportedInterpolations>
      <interpolationMethod>none</interpolationMethod>
    </supportedInterpolations>
  </CoverageOffering>
</CoverageDescription>
