<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE WMS_DescribeLayerResponse SYSTEM "http://{{ .OWSHostname }}/schemas/wms/1.1.1/WMS_DescribeLayerResponse.dtd">
<WMS_DescribeLayerResponse version="1.1.1">
	<LayerDescription name="{{ .Name }}" owsURL="{{ .OWSProtocol }}://{{ .OWSHostname }}/ows/{{ .NameSpace }}" owsType="WMS">
		<Query typeName="{{ .Name }}"/>
	</LayerDescription>
</WMS_DescribeLayerResponse>
