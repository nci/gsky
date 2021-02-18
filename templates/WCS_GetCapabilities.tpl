<?xml version="1.0" encoding="UTF-8"?><WCS_Capabilities xmlns="http://www.opengis.net/wcs" xmlns:gml="http://www.opengis.net/gml" xmlns:xlink="http://www.w3.org/1999/xlink" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://www.opengis.net/wcs http://schemas.opengis.net/wcs/1.0.0/wcsCapabilities.xsd" version="1.0.0">
  <Service>
    <name>gsky</name>
    <label>gsky</label>
    <fees>NONE</fees>
    <accessConstraints>NONE</accessConstraints>
  </Service>
  <Capability>
    <Request>
      <GetCapabilities>
        <DCPType>
          <HTTP>
            <Get>
              <OnlineResource xlink:href="{{ .ServiceConfig.OWSProtocol }}://{{ .ServiceConfig.OWSHostname }}/ows/{{ .ServiceConfig.NameSpace }}" />
            </Get>
          </HTTP>
        </DCPType>
      </GetCapabilities>
      <DescribeCoverage>
        <DCPType>
          <HTTP>
            <Get>
              <OnlineResource xlink:href="{{ .ServiceConfig.OWSProtocol }}://{{ .ServiceConfig.OWSHostname }}/ows/{{ .ServiceConfig.NameSpace }}" />
            </Get>
          </HTTP>
        </DCPType>
      </DescribeCoverage>
      <GetCoverage>
        <DCPType>
          <HTTP>
            <Get>
              <OnlineResource xlink:href="{{ .ServiceConfig.OWSProtocol }}://{{ .ServiceConfig.OWSHostname }}/ows/{{ .ServiceConfig.NameSpace }}" />
            </Get>
          </HTTP>
        </DCPType>
      </GetCoverage>
    </Request>
    <Exception>
      <Format>application/vnd.ogc.se_xml</Format>
    </Exception>
  </Capability>
  <ContentMetadata>
	{{ range $index, $value := .Layers }}
    <CoverageOfferingBrief>
      <description>{{ .Abstract }}</description>
      <name>{{ .Name }}</name>
      <label>{{ .Title }}</label>
      <lonLatEnvelope srsName="urn:ogc:def:crs:OGC:1.3:CRS84">
        <gml:pos>-180.0 -90.0</gml:pos>
        <gml:pos>180.0 90.0</gml:pos>
		{{ range $index, $value := .Dates }}
        <gml:timePosition>{{ $value }}</gml:timePosition>
        {{ end }}
      </lonLatEnvelope>
    </CoverageOfferingBrief>
	{{end}}
  </ContentMetadata>
</WCS_Capabilities>
