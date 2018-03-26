<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<wps:Capabilities xmlns:ows="http://www.opengis.net/ows/1.1" xmlns:xlink="http://www.w3.org/1999/xlink" xmlns:wps="http://www.opengis.net/wps/1.0.0" xml:lang="en-US" service="WPS" updateSequence="1" version="1.0.0">
	<ows:ServiceIdentification>
		<ows:Title>GSKY WPS</ows:Title>
		<ows:Abstract>GSKY - A Scalable, Distributed Geospatial Data Service. https://geonetwork.nci.org.au/geonetwork/srv/eng/catalog.search#/metadata/dc9fb2db-8d6f-4b76-a734-93ac7fbc9201</ows:Abstract>
		<ows:Keywords>
			<ows:Keyword>WPS</ows:Keyword>
			<ows:Keyword>GIS</ows:Keyword>
			<ows:Keyword>Geoprocessing</ows:Keyword>
			<ows:Keyword>Geospatial Data</ows:Keyword>
		</ows:Keywords>
		<ows:ServiceType>WPS</ows:ServiceType>
		<ows:ServiceTypeVersion>1.0.0</ows:ServiceTypeVersion>
	        <ows:Fees>None</ows:Fees>
		<ows:AccessConstraints>None</ows:AccessConstraints>
	</ows:ServiceIdentification>
	<ows:ServiceProvider>
		<ows:ProviderName>Australian National Computational Infrastructure.</ows:ProviderName>
		<ows:ProviderSite xlink:href="http://www.nci.org.au"/>
		<ows:ServiceContact>
			<ows:IndividualName>Pablo Rozas Larraondo</ows:IndividualName>
			<ows:PositionName>High Performance Data Analyst</ows:PositionName>
			<ows:ContactInfo>
				<ows:Phone>
					<ows:Voice>+61 (02) 61253211</ows:Voice>
				</ows:Phone>
				<ows:Address>
					<ows:DeliveryPoint>Building 143, Corner of Ward Road and Garran Road, Ward Rd, Acton ACT 2601</ows:DeliveryPoint>
					<ows:City>ACTON</ows:City>
					<ows:AdministrativeArea>ACT</ows:AdministrativeArea>
					<ows:PostalCode>2601</ows:PostalCode>
					<ows:Country>Australia</ows:Country>
					<ows:ElectronicMailAddress>pablo.larraondo@anu.edu.au</ows:ElectronicMailAddress>
				</ows:Address>
			</ows:ContactInfo>
		</ows:ServiceContact>
	</ows:ServiceProvider>
	<ows:OperationsMetadata>
		<ows:Operation name="GetCapabilities">
			<ows:DCP>
				<ows:HTTP>
					<ows:Get xlink:href="http://130.56.242.20/ows?"/>
				</ows:HTTP>
			</ows:DCP>
		</ows:Operation>
		<ows:Operation name="DescribeProcess">
			<ows:DCP>
				<ows:HTTP>
					<ows:Get xlink:href="http://130.56.242.20/ows?"/>
				</ows:HTTP>
			</ows:DCP>
		</ows:Operation>
		<ows:Operation name="Execute">
			<ows:DCP>
				<ows:HTTP>
					<ows:Get xlink:href="http://130.56.242.20/ows?"/>
				</ows:HTTP>
			</ows:DCP>
		</ows:Operation>
	</ows:OperationsMetadata>
	<wps:ProcessOfferings>
		{{ range $index, $value := . }}
		<wps:Process wps:processVersion="1.0.0">
			<ows:Identifier>{{ .Identifier }}</ows:Identifier>
			<ows:Title>{{ .Title }}</ows:Title>
			<ows:Abstract>{{ .Abstract }}</ows:Abstract>
			<ows:Metadata xlink:title="Time Series Extractor"/>
		</wps:Process>
		{{ end }}
	</wps:ProcessOfferings>
	<wps:Languages>
		<wps:Default>
			<ows:Language>en-US</ows:Language>
		</wps:Default>
		<wps:Supported>
			<ows:Language>en-US</ows:Language>
		</wps:Supported>
	</wps:Languages>
</wps:Capabilities>
