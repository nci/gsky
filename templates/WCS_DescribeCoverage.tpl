<?xml version="1.0" encoding="UTF-8"?>
<CoverageDescription xmlns="http://www.opengis.net/wcs" xmlns:gml="http://www.opengis.net/gml" xmlns:xlink="http://www.w3.org/1999/xlink" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://www.opengis.net/wcs http://schemas.opengis.net/wcs/1.0.0/describeCoverage.xsd" version="1.0.0">
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
      <gml:timePosition>{{ .EffectiveStartDate }}</gml:timePosition>
      <gml:timePosition>{{ .EffectiveEndDate }}</gml:timePosition>
    </lonLatEnvelope>
    <domainSet>
      <spatialDomain>
        <gml:EnvelopeWithTimePeriod srsName="urn:ogc:def:crs:OGC:1.3:CRS84">
          <gml:pos dimension="2">-180.0 -90.0</gml:pos>
          <gml:pos dimension="2">180.0 90.0</gml:pos>
          <gml:timePosition>{{ .EffectiveStartDate }}</gml:timePosition>
          <gml:timePosition>{{ .EffectiveEndDate }}</gml:timePosition>
        </gml:EnvelopeWithTimePeriod>
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
        <description>
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
