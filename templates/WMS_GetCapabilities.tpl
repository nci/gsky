<?xml version="1.0" encoding="UTF-8"?><WMS_Capabilities version="1.3.0" updateSequence="312" xmlns="http://www.opengis.net/wms" xmlns:xlink="http://www.w3.org/1999/xlink" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://www.opengis.net/wms http://schemas.opengis.net/wms/1.3.0/capabilities_1_3_0.xsd" xmlns:dea="http://www.dea.org.au">
	<Service>
		<Name>WMS</Name>
		<Title>GSKY Web Map Service</Title>
		<Abstract>This service relies on GSKY - A Scalable, Distributed Geospatial Data Service. https://geonetwork.nci.org.au/geonetwork/srv/eng/catalog.search#/metadata/dc9fb2db-8d6f-4b76-a734-93ac7fbc9201</Abstract>
		<KeywordList>
			<Keyword>WFS</Keyword>
			<Keyword>WMS</Keyword>
			<Keyword>GSKY</Keyword>
		</KeywordList>
		<OnlineResource xlink:type="simple" xlink:href="http://{{ .ServiceConfig.OWSHostname }}/ows/{{ .ServiceConfig.NameSpace }}" />
		<ContactInformation>
		    <ContactPersonPrimary>
		        <ContactOrganization>National Computational Infrastructure</ContactOrganization>
			<ContactPerson>GSKY Developers</ContactPerson>
		    </ContactPersonPrimary>
			<ContactAddress>
				<Address>143 Ward Road</Address>
				<City>Acton</City>
				<StateOrProvince>ACT</StateOrProvince>
				<PostCode>2601</PostCode>
				<Country>Australia</Country>
			</ContactAddress>
			<ContactElectronicMailAddress>help@nci.org.au</ContactElectronicMailAddress>
		</ContactInformation>
		<Fees>NONE</Fees>
		<LayerLimit>1</LayerLimit>
		<MaxWidth>512</MaxWidth>
		<MaxHeight>512</MaxHeight>
	</Service>
	<Capability>
		<Request>
			<GetCapabilities>
				<Format>text/xml</Format>
				<DCPType>
				  <HTTP>
				    <Get>
				      <OnlineResource xlink:type="simple" xlink:href="http://{{ .ServiceConfig.OWSHostname }}/ows/{{ .ServiceConfig.NameSpace }}?SERVICE=WMS&amp;"/>
				    </Get>
				  </HTTP>
				</DCPType>
			</GetCapabilities>
			<GetMap>
				<Format>image/png</Format>
				<DCPType>
				  <HTTP>
				    <Get>
				      <OnlineResource xlink:type="simple" xlink:href="http://{{ .ServiceConfig.OWSHostname }}/ows/{{ .ServiceConfig.NameSpace }}?SERVICE=WMS&amp;"/>
				    </Get>
				  </HTTP>
				</DCPType>
			</GetMap>
			<GetFeatureInfo>
				<Format>application/json</Format>
				<DCPType>
				  <HTTP>
				    <Get>
				      <OnlineResource xlink:type="simple" xlink:href="http://{{ .ServiceConfig.OWSHostname }}/ows/{{ .ServiceConfig.NameSpace }}?SERVICE=WMS&amp;"/>
				    </Get>
				  </HTTP>
				</DCPType>
			</GetFeatureInfo>
		</Request>
		<Exception>
			<Format>XML</Format>
			<Format>INIMAGE</Format>
			<Format>BLANK</Format>
			<Format>JSON</Format>
		</Exception>
		{{ range $index, $extension := .Extensions }}
		<dea:SupportedExtension>
			<dea:Extension version="{{ .Version }}">{{ .Name }}</dea:Extension>
			<OnlineResource xlink:type="simple" xlink:href="{{ .ResourceURL }}"/>
			<dea:Layer>{{ .Layer.Name }}</dea:Layer>
			{{ range $prop_idx, $property := .Properties }}
				<dea:ExtensionProperty name="{{ .Name }}">
					{{ .Value }}
				</dea:ExtensionProperty>
			{{ end }}
		</dea:SupportedExtension>
		{{ end }}
		<Layer>
			<Title>GSKY Web Map Service</Title>
			<Abstract>A compliant implementation of WMS</Abstract>
			<!--All supported EPSG projections:-->
			<CRS>EPSG:3857</CRS>
			<CRS>EPSG:4326</CRS>
			<EX_GeographicBoundingBox>
				<westBoundLongitude>-180.0</westBoundLongitude>
				<eastBoundLongitude>180.0</eastBoundLongitude>
				<southBoundLatitude>-90.0</southBoundLatitude>
				<northBoundLatitude>90.0</northBoundLatitude>
			</EX_GeographicBoundingBox>
			<BoundingBox CRS="EPSG:4326" minx="-90.0" miny="-180.0" maxx="90.0" maxy="180.0"/>
			{{ range $index, $layer := .Layers }}
			<Layer queryable="1" opaque="0">
				<Name>{{ .Name }}</Name>
				<Title>{{ .Title }}</Title>
				<Abstract>{{ .Abstract }}</Abstract>
				<CRS>EPSG:4326</CRS>
				<EX_GeographicBoundingBox>
					<westBoundLongitude>-180.0</westBoundLongitude>
					<eastBoundLongitude>180.0</eastBoundLongitude>
					<southBoundLatitude>-90.0</southBoundLatitude>
					<northBoundLatitude>90.0</northBoundLatitude>
				</EX_GeographicBoundingBox>
				<BoundingBox CRS="EPSG:4326" minx="-90.0" miny="-180.0" maxx="90.0" maxy="180.0"/>
				<Dimension name="time" default="current" current="True" units="ISO8601">{{ range $index, $value := .Dates }}{{if $index}},{{end}}{{ $value }}{{ end }}</Dimension>

				{{ range $ia, $axis := .AxesInfo }}
				<Dimension name="{{ $axis.Name }}" default="{{ $axis.Default }}">{{ range $iv, $value := $axis.Values }}{{if $iv}},{{end}}{{ $value }}{{ end }}</Dimension>
				{{ end }}

				<MetadataURL type="ISO19115:2003">
					<Format>text/plain</Format>
					<OnlineResource xlink:type="simple" xlink:href="{{ .MetadataURL }}"/>
				</MetadataURL>
				<DataURL>
					<Format>text/plain</Format>
					<OnlineResource xlink:type="simple" xlink:href="{{ .DataURL }}"/>
				</DataURL>
				
				{{ range $styleIdx, $style := $layer.Styles }}
					{{if .Visibility }}
					<Style>
						<Name>{{ .Name }}</Name>
						<Title>{{ .Title }}</Title>
						<Abstract>{{ .Abstract }}</Abstract>
						{{if .LegendPath }}
						<LegendURL width="{{ .LegendWidth }}" height="{{ .LegendHeight }}">
							<Format>image/png</Format>
							<OnlineResource xlink:type="simple" xlink:href="http://{{ $layer.OWSHostname }}/ows/{{ .NameSpace }}?service=WMS&amp;request=GetLegendGraphic&amp;version=1.3.0&amp;layers={{ $layer.Name }}&amp;styles={{ .Name }}"/>
						</LegendURL>
						{{end}}
					</Style>
					{{end}}
				{{end}}
			</Layer>
			{{end}}
		</Layer>
	</Capability>
</WMS_Capabilities>
