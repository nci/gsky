#!/usr/local/bin/perl
# Created on 31 Mar, 2019
# Last edit: 3 Apr, 2019
# By Dr. Arapaut V. Sivaprasad
=pod
This CGI is for creating the KMLs for displaying the GSKY layers via Google Earth Web.
See http://www.webgenie.com/WebGoogleEarth/
=cut
# -----------------------------------
sub reformat
{
  local($tmp) = $_[0] ;
  $tmp =~ s/\+/ /g ;
  while ($tmp =~ /%([0-9A-Fa-f][0-9A-Fa-f])/)
  {
   $num = $1;
   $dec = hex($num);
   $chr = pack("c",$dec);
   $chr =~ s/&/and/g;  # Replace if it is the & char.
   $tmp =~ s/%$num/$chr/g;
  }
  return($tmp);
}
sub debug
{
  $line = $_[0];
  $exit = $_[1];
  if (!$headerAdded) { print "Content-type: text/html\n\n"; $headerAdded = 1; }
  print "$line<br>\n";
  if ($exit) { exit; }
}
sub Get_fields
{
   my @pquery = split(/\&/, $pquery);
   for my $item (@pquery)
   {
           if ($item =~ /(.*)=(.*)/i)
           {
                $$1 = $2; 
           }
   }
}
sub GroundOverlay
{
	$groundOverlay .= "
<!-- $date -->
<GroundOverlay>
    <name>$title</name>
    <visibility>$visibility</visibility>
    <Icon>
        <href>
            http://130.56.242.15/ows/ge?SERVICE=WMS&amp;VERSION=1.1.1&amp;REQUEST=GetMap&amp;SRS=EPSG:4326&amp;WIDTH=512&amp;HEIGHT=512&amp;LAYERS=$layer&amp;STYLES=default&amp;TRANSPARENT=TRUE&amp;FORMAT=image/png&amp;BBOX=$west,$south,$east,$north&amp;$time
        </href>
        <viewRefreshMode>onStop</viewRefreshMode>
        <viewBoundScale>0.75</viewBoundScale>
    </Icon>
    <LatLonBox>
        <north>$north</north>
        <south>$south</south>
        <east>$east</east>
        <west>$west</west>
    </LatLonBox>
</GroundOverlay>
	";
}
sub CreateSingleKML
{
	$kml = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>
<kml xmlns=\"http://www.opengis.net/kml/2.2\" xmlns:gx=\"http://www.google.com/kml/ext/2.2\" xmlns:kml=\"http://www.opengis.net/kml/2.2\" xmlns:atom=\"http://www.w3.org/2005/Atom\">
<GroundOverlay>
    <name>$title</name>
    <visibility>1</visibility>
    <Icon>
        <href>
            http://130.56.242.15/ows/ge?SERVICE=WMS&amp;VERSION=1.1.1&amp;REQUEST=GetMap&amp;SRS=EPSG:4326&amp;WIDTH=512&amp;HEIGHT=512&amp;LAYERS=$layer&amp;STYLES=default&amp;TRANSPARENT=TRUE&amp;FORMAT=image/png&amp;BBOX=$west,$south,$east,$north&amp;$time
        </href>
        <viewRefreshMode>onStop</viewRefreshMode>
        <viewBoundScale>0.75</viewBoundScale>
    </Icon>
    <LatLonBox>
        <north>$north</north>
        <south>$south</south>
        <east>$east</east>
        <west>$west</west>
    </LatLonBox>
</GroundOverlay>
</kml>
	";
}

sub CreateMultipleKML
{
	$visibility = 1; # Set this to 0 after the first layer. 
	my @times = split(/,/, $time);
	my $len = $#times;
	for (my $j=0; $j <= $len; $j++)
	{
		$date = $times[$j];
		$date =~ s/T.*Z//gi;
		$title = $region . "_" . $basetitle . " " . $date;
		$time = "";
		if($times[$j]) { $time="TIME=$times[$j]"; }
		GroundOverlay;
		$visibility = 0; # Subsequent layers are set as visibility=0
	}
	$kml = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>
<kml xmlns=\"http://www.opengis.net/kml/2.2\" xmlns:gx=\"http://www.google.com/kml/ext/2.2\" xmlns:kml=\"http://www.opengis.net/kml/2.2\" xmlns:atom=\"http://www.w3.org/2005/Atom\">
<Document>
$groundOverlay	
</Document>
</kml>
	";
}
sub do_main
{
        # Kill a runaway CGI, if any.
        my $psline = `ps -ef | grep google_earth.cgi | grep -v grep`;
        my @fields = split (/\s/, $psline);
        $pid = $fields[1];
        my $thispid = $$;
        if ($pid && $pid ne $thispid) { `kill $pid`; }

        my $cl = $ENV{'CONTENT_LENGTH'}; # Method=POST will have a non-zero value.
        $cl //= 0; # Set the value as 0 if $cl is undefined. It won't happen on a well built Apache server.
        if ($cl > 0)
        {
			read(STDIN, $_, $cl);
			$_ .= "&"; # Append an & char so that the last item is not ignored
			$pquery = &reformat($_);
        	print "Content-type: text/html\n\n"; $headerAdded = 1;
        }
        else
        {
        	print "Content-type: text/html\n\n"; $headerAdded = 1;
=pod
			The form input values are sent as a URI by 'ajax.js'. 
				url = cgi + "?createKML+" + ran_number + "+" + pquery;
			The first item is generally the action required. However, in this CGI it is not used.
			Item #2 is a random number to ensure that the URI is not cached.
			Item #3 is the GET string to be parsed.
=cut
			$sc_action = $ARGV[0];
			$pquery = reformat($ARGV[2]);
			$pquery =~ s/\\//gi;
			Get_fields;	# Parse the $pquery to get all form input values
			@fields = split (/\|/, $layer);
			$layer = $fields[0];
			$title = $fields[1];
			$basetitle = $title;
			if ($time =~ /,/)
			{
				# This is a multiple time selection
				CreateMultipleKML;
				$outfile = $region . "_" . $basetitle . ".kml";
				$outfile =~ s/ /_/gi;
				open (OUT, ">KML/$outfile");
				print OUT $kml;
				close(OUT);
				print "<small>Click to download: <a href=\"$url/$outfile\">$outfile</a></small>";
				exit;
			}
			else
			{
				if ($time)
				{
					$time="TIME=$time";
				}
				$title = $region . "_" . $basetitle . " " . $date;
				CreateSingleKML;
				$outfile = $region . "_" . $basetitle . "_" . $date . ".kml";
				$outfile =~ s/ /_/gi;
				open (OUT, ">KML/$outfile");
				print OUT $kml;
				close(OUT);
				print "<small>Click to download: <a href=\"$url/$outfile\">$outfile</a></small>";
			}
        }
}
$|=1;
$url = "https://www.webgenie.com/WebGoogleEarth/KML";
&do_main;
