<wps:ProcessDescriptions xmlns:ows="http://www.opengis.net/ows/1.1" xmlns:wps="http://www.opengis.net/wps/1.0.0" xmlns:xlink="http://www.w3.org/1999/xlink" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://www.opengis.net/wps/1.0.0 http://schemas.opengis.net/wps/1.0.0/wpsDescribeProcess_response.xsd" service="WPS" version="1.0.0" xml:lang="en-US">
<ProcessDescription wps:processVersion="2" storeSupported="true" statusSupported="true">
	<ows:Identifier>{{ .Identifier }}</ows:Identifier>
	<ows:Title>{{ .Title}}</ows:Title>
	<ows:Abstract>{{ .Abstract}}</ows:Abstract>
	<ows:Metadata xlink:title="TimeSeries Extractor"/>
	<DataInputs>
		{{ range $index, $value := .LiteralData }}
		<Input minOccurs="{{ .MinOccurs }}" maxOccurs="1">
			<ows:Identifier>{{ .Identifier }}</ows:Identifier>
			<ows:Title>{{ .Title }}</ows:Title>
			<ows:Abstract>{{ .Abstract }}</ows:Abstract>
			<LiteralData>
				<ows:DataType ows:reference="{{ .DataTypeRef }}">{{ .DataType }}</ows:DataType>
				{{ if .AllowedValues }}
				<ows:AllowedValues>
					{{ range $index, $value := .AllowedValues }}
					<ows:Value>{{ . }}</ows:Value>
					{{ end }}
				</ows:AllowedValues>
				{{ else }}
				<ows:AnyValue />
				{{ end }}
			</LiteralData>
		</Input>
		{{ end }}
		{{ range $index, $value := .ComplexData }}
		<Input minOccurs="{{ .MinOccurs }}" maxOccurs="1">
			<ows:Identifier>{{ .Identifier }}</ows:Identifier>
			<ows:Title>{{ .Title }}</ows:Title>
			<ows:Abstract>{{ .Abstract }}</ows:Abstract>
			<ComplexData>
				<Default>
					<Format>
						<MimeType>{{ .MimeType }}</MimeType>
						<Schema>{{ .Schema }}</Schema>
					</Format>
				</Default>
				<Supported>
					<Format>
						<MimeType>{{ .MimeType }}</MimeType>
						<Schema>{{ .Schema }}</Schema>
					</Format>
				</Supported>
			</ComplexData>
		</Input>
		{{ end }}
	</DataInputs>
	<ProcessOutputs>
		<Output>
			<ows:Identifier>Result</ows:Identifier>
			<ows:Title>Time Series Output</ows:Title>
			<ows:Abstract>Time series data for location.</ows:Abstract>
			<ComplexOutput>
				<Default>
					<Format>
						<MimeType>application/vnd.terriajs.catalog-member+json</MimeType>
						<Schema>https://tools.ietf.org/html/rfc7159</Schema>
					</Format>
				</Default>
				<Supported>
					<Format>
						<MimeType>application/vnd.terriajs.catalog-member+json</MimeType>
						<Schema>https://tools.ietf.org/html/rfc7159</Schema>
					</Format>
				</Supported>
			</ComplexOutput>
		</Output>
	</ProcessOutputs>
</ProcessDescription>
</wps:ProcessDescriptions>
