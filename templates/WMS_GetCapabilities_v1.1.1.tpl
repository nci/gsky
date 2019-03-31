<?xml version='1.0' encoding="UTF-8" standalone="no" ?>
<!-- The DTD (Document Type Definition) given here must correspond to the version number declared in the WMT_MS_Capabilities element below. -->
<!DOCTYPE WMT_MS_Capabilities SYSTEM
 "http://www.digitalearth.gov/wmt/xml/capabilities_1_1_1.dtd"
 [
 <!-- Vendor-specific elements are defined here if needed. -->
 <!-- If not needed, just leave this EMPTY declaration.  Do not
  delete the declaration entirely. -->
 <!ELEMENT VendorSpecificCapabilities EMPTY>
 ]>  <!-- end of DOCTYPE declaration -->

<!-- Note: this XML is just an EXAMPLE that attempts to show all
required and optional elements for illustration.  Consult the Web Map
Service 1.1.0 specification and the DTD for guidance on what to actually
include and what to leave out. -->

<!-- The version number listed in the WMT_MS_Capabilities element here must
correspond to the DTD declared above.  See the WMT specification document for
how to respond when a client requests a version number not implemented by the
server. -->
<WMT_MS_Capabilities version="1.1.1" updateSequence="0">
<!-- Service Metadata -->
<Service>
  <!-- The WMT-defined name for this type of service -->
  <Name>OGC:WMS</Name>
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
		        <ContactPerson>Dr. Arapaut V. Sivaprasad</ContactPerson>
		       <!-- <ContactOrganization>NCI</ContactOrganization> --> <!-- This element is not accepted by Google Earth, though it is a valid element in WMS v1.1.1 -->
		    </ContactPersonPrimary>
		    <ContactPosition>Senior Software Engineer</ContactPosition>
			<ContactAddress>
				<Address>143 Ward Road</Address>
				<City>Acton</City>
				<StateOrProvince>ACT</StateOrProvince>
				<PostCode>2601</PostCode>
				<Country>Australia</Country>
			</ContactAddress>

			<ContactVoiceTelephone>+61 2 6135 3211</ContactVoiceTelephone>
			<ContactFacsimileTelephone>NA</ContactFacsimileTelephone>

			<ContactElectronicMailAddress>Arapaut.Sivaprasad@anu.edu.au</ContactElectronicMailAddress>
		</ContactInformation>
		<Fees>NONE</Fees>
		<AccessConstraints>NONE</AccessConstraints>
	</Service>
	<Capability>
		<Request>
			<GetCapabilities>
				<Format>application/vnd.ogc.wms_xml</Format>
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
				<Format>text/plain</Format>
				<Format>application/vnd.ogc.gml</Format>
				<Format>text/xml</Format>
				<Format>application/vnd.ogc.gml/3.1.1</Format>
				<Format>text/xml; subtype=gml/3.1.1</Format>
				<Format>text/html</Format>
				<Format>application/json</Format>
				<DCPType>
				  <HTTP>
				    <Get>
				      <OnlineResource xlink:type="simple" xlink:href="http://{{ .ServiceConfig.OWSHostname }}/ows/{{ .ServiceConfig.NameSpace }}?SERVICE=WMS&amp;"/>
				    </Get>
				  </HTTP>
				</DCPType>
			</GetFeatureInfo>
			<DescribeLayer>
				<Format>application/vnd.ogc.wms_xml</Format>
				<DCPType>
				  <HTTP>
					<Get>
					  <OnlineResource xlink:type="simple" xlink:href="http://{{ .ServiceConfig.OWSHostname }}/ows/{{ .ServiceConfig.NameSpace }}?SERVICE=WMS&amp;"/>
					</Get>
				  </HTTP>
				</DCPType>
			</DescribeLayer>
			<GetLegendGraphic>
				<Format>image/png</Format>
				<Format>image/jpeg</Format>
				<Format>application/json</Format>
				<Format>image/gif</Format>
				<DCPType>
				  <HTTP>
					<Get>
					  <OnlineResource xlink:type="simple" xlink:href="http://{{ .ServiceConfig.OWSHostname }}/ows/{{ .ServiceConfig.NameSpace }}?SERVICE=WMS&amp;"/>
					</Get>
				  </HTTP>
				</DCPType>
			</GetLegendGraphic>
			<GetStyles>
				<Format>application/vnd.ogc.sld+xml</Format>
				<DCPType>
				  <HTTP>
					<Get>
					  <OnlineResource xlink:type="simple" xlink:href="http://{{ .ServiceConfig.OWSHostname }}/ows/{{ .ServiceConfig.NameSpace }}?SERVICE=WMS&amp;"/>
					</Get>
				  </HTTP>
				</DCPType>
			</GetStyles>
		</Request>
		<Exception>
			<Format>application/vnd.ogc.se_xml</Format>
			<Format>application/vnd.ogc.se_inimage</Format>
			<Format>application/vnd.ogc.se_blank</Format>
			<Format>application/json</Format>
			<Format>XML</Format>
			<Format>INIMAGE</Format>
			<Format>BLANK</Format>
			<Format>JSON</Format>
		</Exception>
		<Layer>
			<Title>GSKY Map Server</Title>
			<SRS>EPSG:3857</SRS> <!-- all layers are available in at least this SRS -->
			<SRS>EPSG:4326</SRS> <!-- all layers are available in at least this SRS -->
			<Layer queryable="1" opaque="0">
				<Name>global:c6:frac_cover</Name>
				<Title>GEOGLAM Fractional Cover C6</Title>
				<Abstract> <!-- Valid, but not required. Comes from conf.json -->
					Fractional Cover - MODIS, CSIRO Land and Water algorithm, Australia coverage. Vegetation fractional cover represents the exposed proportion of Photosynthetic Vegetation (PV), Non-Photosynthetic Vegetation (NPV) and Bare Soil (BS) within each pixel. In forested canopies the photosynthetic or non-photosynthetic portions of trees may obscure those of the grass layer and/or bare soil. The MODIS Fractional Cover product is derived from the MODIS Nadir BRDF-Adjusted Reflectance product MCD43A4). A suite of derivative are also produced, namely total vegetation cover (PV+NPV), monthly fractional cover and total vegetation cover, monthly anomaly of total cover against the time series, and three-monthly total cover difference. MODIS fractional cover has been validated for Australia. A global product is also produced with the same algorithm using the MCD43C1 (0.05 degrees spatial resolution).
				</Abstract>
				<SRS>EPSG:4326</SRS>
				<SRS>SRS:84</SRS>
				<LatLonBoundingBox minx="-180.0" miny="-90.0" maxx="180.0" maxy="90.0"/>
				<BoundingBox SRS="SRS:84" minx="-180.0" miny="-90.0" maxx="180.0" maxy="90.0"/>
				<BoundingBox SRS="EPSG:4326" minx="-90.0" miny="-180.0" maxx="90.0" maxy="180.0"/>
			</Layer>
			<Layer queryable="1" opaque="0">
				<Name>global:c6:total_cover</Name>
				<Title>GEOGLAM Total Cover C6</Title>
				<Abstract> <!-- Valid, but not required. Comes from conf.json -->
					Total Cover- MODIS, CSIRO Land and Water algorithm, Australia coverage. Vegetation fractional cover represents the exposed proportion of Photosynthetic Vegetation (PV), Non-Photosynthetic Vegetation (NPV) and Bare Soil (BS) within each pixel. In forested canopies the photosynthetic or non-photosynthetic portions of trees may obscure those of the grass layer and/or bare soil. The MODIS Fractional Cover product is derived from the MODIS Nadir BRDF-Adjusted Reflectance product MCD43A4). A suite of derivative are also produced, namely total vegetation cover (PV+NPV), monthly fractional cover and total vegetation cover, monthly anomaly of total cover against the time series, and three-monthly total cover difference. MODIS fractional cover has been validated for Australia. A global product is also produced with the same algorithm using the MCD43C1 (0.05 degrees spatial resolution).
				</Abstract>
				<SRS>EPSG:4326</SRS>
				<SRS>SRS:84</SRS>
				<LatLonBoundingBox minx="-180.0" miny="-90.0" maxx="180.0" maxy="90.0"/>
				<BoundingBox SRS="SRS:84" minx="-180.0" miny="-90.0" maxx="180.0" maxy="90.0"/>
				<BoundingBox SRS="EPSG:4326" minx="-90.0" miny="-180.0" maxx="90.0" maxy="180.0"/>
			</Layer>
			<Layer queryable="1" opaque="0">
				<Name>global:c6:monthly_frac_cover</Name>
				<Title>GEOGLAM Monthly Fractional Cover C6</Title>
				<Abstract> <!-- Valid, but not required. Comes from conf.json -->
					Monthly Cover - MODIS, CSIRO Land and Water algorithm, Australia coverage. Vegetation monthly cover represents the exposed proportion of Photosynthetic Vegetation (PV), Non-Photosynthetic Vegetation (NPV) and Bare Soil (BS) within each pixel. In forested canopies the photosynthetic or non-photosynthetic portions of trees may obscure those of the grass layer and/or bare soil. The MODIS Fractional Cover product is derived from the MODIS Nadir BRDF-Adjusted Reflectance product MCD43A4). A suite of derivative are also produced, namely total vegetation cover (PV+NPV), monthly fractional cover and total vegetation cover, monthly anomaly of total cover against the time series, and three-monthly total cover difference. MODIS fractional cover has been validated for Australia. A global product is also produced with the same algorithm using the MCD43C1 (0.05 degrees spatial resolution).
				</Abstract>
				<SRS>EPSG:4326</SRS>
				<SRS>SRS:84</SRS>
				<LatLonBoundingBox minx="-180.0" miny="-90.0" maxx="180.0" maxy="90.0"/>
				<BoundingBox SRS="SRS:84" minx="-180.0" miny="-90.0" maxx="180.0" maxy="90.0"/>
				<BoundingBox SRS="EPSG:4326" minx="-90.0" miny="-180.0" maxx="90.0" maxy="180.0"/>
			</Layer>
			<Layer queryable="1" opaque="0">
				<Name>global:c6:monthly_total_cover</Name>
				<Title>GEOGLAM Monthly Total Cover C6</Title>
				<Abstract> <!-- Valid, but not required. Comes from conf.json -->
					Total Cover- MODIS, CSIRO Land and Water algorithm, Australia coverage. Vegetation fractional cover represents the exposed proportion of Photosynthetic Vegetation (PV), Non-Photosynthetic Vegetation (NPV) and Bare Soil (BS) within each pixel. In forested canopies the photosynthetic or non-photosynthetic portions of trees may obscure those of the grass layer and/or bare soil. The MODIS Fractional Cover product is derived from the MODIS Nadir BRDF-Adjusted Reflectance product MCD43A4). A suite of derivative are also produced, namely total vegetation cover (PV+NPV), monthly fractional cover and total vegetation cover, monthly anomaly of total cover against the time series, and three-monthly total cover difference. MODIS fractional cover has been validated for Australia. A global product is also produced with the same algorithm using the MCD43C1 (0.05 degrees spatial resolution).
				</Abstract>
				<SRS>EPSG:4326</SRS>
				<SRS>SRS:84</SRS>
				<LatLonBoundingBox minx="-180.0" miny="-90.0" maxx="180.0" maxy="90.0"/>
				<BoundingBox SRS="SRS:84" minx="-180.0" miny="-90.0" maxx="180.0" maxy="90.0"/>
				<BoundingBox SRS="EPSG:4326" minx="-90.0" miny="-180.0" maxx="90.0" maxy="180.0"/>
			</Layer>
			<Layer queryable="1" opaque="0">
				<Name>global:c6:monthly_decile_total_cover</Name>
				<Title>GEOGLAM Monthly Decile Total Cover C6</Title>
				<Abstract> <!-- Valid, but not required. Comes from conf.json -->
					Total Cover- MODIS, CSIRO Land and Water algorithm, Australia coverage. Vegetation fractional cover represents the exposed proportion of Photosynthetic Vegetation (PV), Non-Photosynthetic Vegetation (NPV) and Bare Soil (BS) within each pixel. In forested canopies the photosynthetic or non-photosynthetic portions of trees may obscure those of the grass layer and/or bare soil. The MODIS Fractional Cover product is derived from the MODIS Nadir BRDF-Adjusted Reflectance product MCD43A4). A suite of derivative are also produced, namely total vegetation cover (PV+NPV), monthly fractional cover and total vegetation cover, monthly anomaly of total cover against the time series, and three-monthly total cover difference. MODIS fractional cover has been validated for Australia. A global product is also produced with the same algorithm using the MCD43C1 (0.05 degrees spatial resolution).
				</Abstract>
				<SRS>EPSG:4326</SRS>
				<SRS>SRS:84</SRS>
				<LatLonBoundingBox minx="-180.0" miny="-90.0" maxx="180.0" maxy="90.0"/>
				<BoundingBox SRS="SRS:84" minx="-180.0" miny="-90.0" maxx="180.0" maxy="90.0"/>
				<BoundingBox SRS="EPSG:4326" minx="-90.0" miny="-180.0" maxx="90.0" maxy="180.0"/>
			</Layer>
			<Layer queryable="1" opaque="0">
				<Name>global:c6:monthly_anom_frac_cover</Name>
				<Title>GEOGLAM Anomaly Fractional Cover C6</Title>
				<Abstract> <!-- Valid, but not required. Comes from conf.json -->
					Fractional Cover - MODIS, CSIRO Land and Water algorithm, Australia coverage. Vegetation fractional cover represents the exposed proportion of Photosynthetic Vegetation (PV), Non-Photosynthetic Vegetation (NPV) and Bare Soil (BS) within each pixel. In forested canopies the photosynthetic or non-photosynthetic portions of trees may obscure those of the grass layer and/or bare soil. The MODIS Fractional Cover product is derived from the MODIS Nadir BRDF-Adjusted Reflectance product MCD43A4). A suite of derivative are also produced, namely total vegetation cover (PV+NPV), monthly fractional cover and total vegetation cover, monthly anomaly of total cover against the time series, and three-monthly total cover difference. MODIS fractional cover has been validated for Australia. A global product is also produced with the same algorithm using the MCD43C1 (0.05 degrees spatial resolution).
				</Abstract>
				<SRS>EPSG:4326</SRS>
				<SRS>SRS:84</SRS>
				<LatLonBoundingBox minx="-180.0" miny="-90.0" maxx="180.0" maxy="90.0"/>
				<BoundingBox SRS="SRS:84" minx="-180.0" miny="-90.0" maxx="180.0" maxy="90.0"/>
				<BoundingBox SRS="EPSG:4326" minx="-90.0" miny="-180.0" maxx="90.0" maxy="180.0"/>
			</Layer>
			<Layer queryable="1" opaque="0">
				<Name>LS8:NBAR:TRUE</Name>
				<Title>DEA Landsat 8 surface reflectance true colour</Title>
				<Abstract> <!-- Valid, but not required. Comes from conf.json -->
					This product has been corrected to remove the influences of the atmosphere, the time of year and satellite view angles using the methods described in Li et al. 2010 https://doi.org/10.1109/JSTARS.2010.2042281. Landsat 8 Operational Land Imager (OLI) data is available from March 2013 and onwards. More detailed information about the surface reflectance product suite produced using Digital Earth Australia including CCBY4.0 is available at http://dx.doi.org/10.4225/25/5a7a501e1c5af. This service provides access to Landsat 8 OLI terrain corrected surface reflectance data. This service provides access to Landsat 8 OLI surface reflectance data. The true colour composite is composed of wavelengths of light as seen by the human eye. The image composites are made from images acquired within a 16 day period, and may include clouds.  
				</Abstract>
				<SRS>EPSG:4326</SRS>
				<SRS>SRS:84</SRS>
				<LatLonBoundingBox minx="-180.0" miny="-90.0" maxx="180.0" maxy="90.0"/>
				<BoundingBox SRS="SRS:84" minx="-180.0" miny="-90.0" maxx="180.0" maxy="90.0"/>
				<BoundingBox SRS="EPSG:4326" minx="-90.0" miny="-180.0" maxx="90.0" maxy="180.0"/>
			</Layer>
		</Layer>
	</Capability>
</WMT_MS_Capabilities>
