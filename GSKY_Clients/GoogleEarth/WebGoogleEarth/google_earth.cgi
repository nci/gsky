#!/usr/bin/env perl
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
sub debugEnv
{
   print "Content-type:text/html\n\n";
   print "<Pre>\n";
   foreach $item (%ENV)
   {
      print "$item\n";
   }
   exit;
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
sub GroundOverlayTiles
{
	my $skip_curl = $_[0];
	$groundOverlay .= "
<!-- $date -->
<GroundOverlay>
    <name>$title</name>
    <visibility>$visibility</visibility>
    <Icon>
        <href>
            $tileUrl
        </href>
        <viewRefreshMode>onStop</viewRefreshMode>
        <viewBoundScale>0.75</viewBoundScale>
    </Icon>
    <LatLonBox>
        <west>$west</west>
        <south>$south</south>
        <east>$east</east>
        <north>$north</north>
    </LatLonBox>
</GroundOverlay>
	";
	if (!$skip_curl)
	{
		print OUT "echo $nt\n"; $nt++;
		print OUT "curl 'http://$domain/ows/ge?SERVICE=WMS&VERSION=1.1.1&REQUEST=GetMap&SRS=EPSG:4326&WIDTH=512&HEIGHT=512&LAYERS=$layer&STYLES=default&TRANSPARENT=TRUE&FORMAT=image/png&BBOX=$west,$south,$east,$north&$time'\n";
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
            http://130.56.242.15/ows/ge?SERVICE=WMS&amp;BBOX=$west,$south,$east,$north&amp;$time&amp;VERSION=1.1.1&amp;REQUEST=GetMap&amp;SRS=EPSG:4326&amp;WIDTH=512&amp;HEIGHT=512&amp;LAYERS=$layer&amp;STYLES=default&amp;TRANSPARENT=TRUE&amp;FORMAT=image/png
        </href>
        <viewRefreshMode>onStop</viewRefreshMode>
        <viewBoundScale>0.75</viewBoundScale>
    </Icon>
    <LatLonBox>
        <west>$west</west>
        <south>$south</south>
        <east>$east</east>
        <north>$north</north>
    </LatLonBox>
</GroundOverlay>
	";
	# Create a Shell script to run the WMS for each tile and save as PNG
#	print OUT "echo $nt\n"; $nt--;
#	print OUT "curl 'http://130.56.242.15/ows/ge?SERVICE=WMS&amp;VERSION=1.1.1&amp;REQUEST=GetMap&amp;SRS=EPSG:4326&amp;WIDTH=512&amp;HEIGHT=512&amp;LAYERS=$layer&amp;STYLES=default&amp;TRANSPARENT=TRUE&amp;FORMAT=image/png&amp;BBOX=$west,$south,$east,$north&amp;$time'\n";
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
sub CreateMultipleTilesKML
{
	$visibility = 1; # Set this to 0 after the first layer. 
	
	open (INP, "<$aus_bboxes");
	my @filecontent = <INP>;
	close(INP);
	my $len = $#filecontent;
	open (OUT, ">$curl_sh");
	$nt = $len+1;
	for (my $j=0; $j <= $len; $j++)
	{
		my $line = $filecontent[$j];
		$line =~ s/\n//;
		my @fields = split(/,/, $line);
#		if ($fields[4] == 0) { next; } # Skip if the value in last column is 0
		$west = $fields[0];
		$south = $fields[1];
		$east = $fields[2];
		$north = $fields[3];
		pop(@fields);
		$bbox = join(",", @fields);
		GroundOverlay;
#		$visibility = 0; # Subsequent layers are set as visibility=0
	}
	close(OUT);
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
#&debugEnv;	
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
=pod
		The form input values are sent as a URI by 'ajax.js'. 
			url = cgi + "?createKML+" + ran_number + "+" + pquery;
		The first item is generally the action required. However, in this CGI it is not used.
		Item #2 is a random number to ensure that the URI is not cached.
		Item #3 is the GET string to be parsed.
=cut
		$sc_action = $ARGV[0];
		if (!$sc_action)
		{
			$dumb = 1; # This is a dumb URL with &BBOX=0,0,0,0 added at the end.
			$request_string = $ENV{QUERY_STRING};
#&debug("request_string = $request_string");	
			@fields = split (/\&/, $request_string);
			$sc_action = $fields[0];
			@fields = split (/\+/, $sc_action);
			$sc_action = $fields[0];
			if ($sc_action eq "PNG")
			{
				$west = $fields[1];
				$south = $fields[2];
				$east = $fields[3];
				$north = $fields[4];
				$time = $fields[5];
			}
		}
#&debug("sc_action = $sc_action");			
		if ($sc_action eq "createKML")
		{
			print "Content-type: text/html\n\n"; $headerAdded = 1;
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
				$outfile = $region . "_" . $basetitle . "_" . $$ . "_" . $date . ".kml";
				$outfile =~ s/ /_/gi;
				open (OUT, ">KML/$outfile");
				print OUT $kml;
				close(OUT);
				print "<small>Click to download: <a href=\"$url/$outfile\">$outfile</a></small>";
			}
		}
=pod
For DEA: 

To create small tiles across Australia and display them as individual Icons on GEWeb.
It will solve the issue of zoom level over large area. GSKY cannot display
the layer if the zoom level is more than 2 degrees. To give a better resolution,
here we use 1x1 degree tiles and then display them as multiple Icons in GEWeb.
It will let one see the whole Australia together and then zom into see smaller
areas. 

The user experience will improve drammatically. By using cached tiles, the display
will also be faster than directly calling GSKY.

The following are to manually create the GSKY layers.

How to run:
Items 1 and 4 are on webgenie.com, 2 and 3 on VM and 5,6 on Browser.

1. Run at shell command: /home/900/avs900/WebGoogleEarth/google_earth.cgi BBox
- It will create '/home/900/avs900/WebGoogleEarth/aus_bboxes.csv'
- e.g.
	110,-45,115,-40
	110,-40,115,-35
	
2. Run at shell: source /home/900/avs900/WebGoogleEarth/dea_tiles_1.sh&	
- Create tiles for the specific date in /home/900/avs900/WebGoogleEarth/Tiles/Date
- Took 27 min to create the tiles for 2013-03-17

3. Tar the contents of /home/900/avs900/WebGoogleEarth/Tiles/Date 
- SCP to webgenie.com:/var/www/vhosts/webgenie.com/httpdocs/WebGoogleEarth/Tiles/Date
- Untar into /var/www/vhosts/webgenie.com/httpdocs/WebGoogleEarth/Tiles/Date

4. Run at shell command: /home/900/avs900/WebGoogleEarth/google_earth.cgi DeleteEmptyTiles Date
- To delete the empty tiles
- e.g. ./google_earth.cgi DeleteEmptyTiles 2013-03-17

5. Run within browser:
http://www.webgenie.com/WebGoogleEarth/Dev/google_earth.cgi?TilesKML+Date
- To create the KML file for display with GEWeb
- e.g. http://www.webgenie.com/WebGoogleEarth/Dev/google_earth.cgi?TilesKML+2013-03-17

6. Save the KML file and open in GEWeb	

=cut
		if ($sc_action eq "BBox") # For DEA. Create the tile coordinates
		{
			print "Content-type: text/html\n\n"; $headerAdded = 1;
			$w = 110;
			$s = -45;
			$e = 155;
			$n = -10;
			$i = 0;
			open (OUT, ">$aus_bboxes");
			for (my $j = $w; $j <= $e; $j++)
			{
				for (my $k = $s; $k <= $n; $k++)
				{
					$w1 = $j;
					$s1 = $k;
					$n1 = $k+1;
					$e1 = $j+1;
					print OUT "$w1,$s1,$e1,$n1\n";
					$i++;
					if($n1 == $n) { last; }
				}
			}
			close (OUT);
			print "$i records written\n";
		}
=pod			
sub serveImage
{
use GD;

my ( $localPath ) = @_;

if( $localPath =~ /\.png/i )
{
	print "Content-type: image/png\n\n";
	binmode STDOUT;
	my $image = GD::Image->newFromPng( $localPath );
	print $image->png;
}
else
{
	print "Content-type: image/jpeg\n\n";
	binmode STDOUT;
	my $image = GD::Image->newFromJpeg( $localPath );
	print $image->jpeg(100);
}


}
=cut
		if ($sc_action eq "createMultiTilesKML") # For DEA
		{
			print "Content-type: text/html\n\n"; $headerAdded = 1;
			$layer = "LS8:NBAR:TRUE";
			$title = "DEA Landsat 8 surface reflectance true colour";
			CreateMultipleTilesKML;
			$outfile = "DEA_" . $layer . "_" . $$ . ".kml";
			$outfile =~ s/ /_/gi;
			open (OUT, ">KML/$outfile");
			print OUT $kml;
#				print $kml;
			close(OUT);
			print "<small>Click to download: <a href=\"$url/$outfile?$$\">$outfile</a></small>";
		}
		if ($sc_action eq "Test") # For DEA
		{
			# Open the file 117.0+-32.0+118.0+-31.0+2013-03-17
			my $filename = "/var/www/vhosts/webgenie.com/httpdocs/WebGoogleEarth/Tiles/tile_117.0_-32.0_118.0_-31.0_2013-03-17_.png";
			select(STDOUT); $| = 1;   #unbuffer STDOUT
#				print "Content-type: image/png\n\n";
			print "Content-type: text/plain\n\n";
			
			open (IMAGE, '<', $filename);
			print <IMAGE>;
			close IMAGE;			
		}
		if ($sc_action eq "DeleteEmptyTiles") # For DEA. Delete the PNG files that are empty
		{
			# Usage: ./google_earth.cgi DeleteEmptyTiles 2013-03-17
			$date = $ARGV[1];
			$td = "/var/www/vhosts/webgenie.com/httpdocs/WebGoogleEarth/Tiles/$date";
			$ls = `ls -lS $td/*.png`;
			my @ls = split(/\n/, $ls);
			my $len = $#ls;
			for (my $j=0; $j <= $len; $j++)
			{
				my $line = $ls[$j];
				$line =~ tr/  / /s;
				my @fields = split(/ /, $line);
				if ($fields[4] == 2132) # Empty tiles
				{
					unlink ($fields[8]);
				}
			}
		}
		if ($sc_action eq "TilesKML") # for DEA. Create the *.kml file 
		{
			$date = $ARGV[1];
#&debug("date = $date",1);				
			print "Content-type: text/html\n\n"; $headerAdded = 1;
			$visibility = 1; # Set this to 0 after the first layer. 
			$layer = "LS8:NBAR:TRUE";
			$title = "DEA Landsat 8 surface reflectance true colour";
			$td = "/var/www/vhosts/webgenie.com/httpdocs/WebGoogleEarth/Tiles/$date";
			$ls = `ls -lS $td/*.png`;
			my @ls = split(/\n/, $ls);
			my $len = $#ls;
			for (my $j=0; $j <= $len; $j++)
			{
				my $line = $ls[$j];
				$line =~ tr/  / /s;
				my @fields = split(/ /, $line);
				my @fields = split(/_/, $fields[8]);
				$west = $fields[1];
				$south = $fields[2];
				$east = $fields[3];
				$north = $fields[4];
				$time = $fields[5];
				$tileUrl = "$cgi?PNG+$west+$south+$east+$north+$time";					
				GroundOverlayTiles;
			}
			$kml = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>
<kml xmlns=\"http://www.opengis.net/kml/2.2\" xmlns:gx=\"http://www.google.com/kml/ext/2.2\" xmlns:kml=\"http://www.opengis.net/kml/2.2\" xmlns:atom=\"http://www.w3.org/2005/Atom\">
<Document>
$groundOverlay	
</Document>
</kml>
";
			$outfile = "DEA_" . $layer . "_" . $time . "_" . $$ . ".kml";
			$outfile =~ s/ /_/gi;
			open (OUT, ">KML/$outfile");
			print OUT $kml;
#				print $kml;
			close(OUT);
			print "<small>Click to download: <a href=\"$url/$outfile?$$\">$outfile</a></small>";
		}
		if ($sc_action eq "PNG") # For DEA
		{
			if (!$dumb)
			{
				$west = $ARGV[1];
				$south = $ARGV[2];
				$east = $ARGV[3];
				$north = $ARGV[4];
				$time = $ARGV[5];
			}
			# Open the file 
			$tilefile = "$basedir/$time/tile_" . $west . "_" . $south . "_" . $east . "_" . $north . "_" . $time . "_" . ".png";
#&debug("tilefile = $tilefile");
			eval 
			{
				select(STDOUT); $| = 1;   #unbuffer STDOUT
				print "Content-type: image/png\n\n";
				open (IMAGE, '<', $tilefile);
				print <IMAGE>;
				close(IMAGE);
			};
		}
		if ($sc_action eq "CreateDataTilesKML") # For DEA. Create a KML with only data tiles
		{
			# Usage: http://130.56.242.19/cgi-bin/google_earth.cgi?CreateDataTilesKML+2013-03-17
			print "Content-type: text/html\n\n"; $headerAdded = 1;
			$time = $ARGV[1];
			if (!$time) { $time = "2013-03-17"; }
			$tilesdir = "$basedir/$time";
			$ls = `ls -l $tilesdir/*.png`;
			@ls = split(/\n/, $ls);
			my $len = $#ls;
			print "<pre>\n";
			for (my $j=0; $j <= $len; $j++)
			{
				my $line = $ls[$j];
				$line =~ tr/  / /s;
				my @fields = split(/ /, $line);
#				if ($fields[4] > 2132) # skip Empty tiles
				{
# /var/www/html/WebGoogleEarth/Tiles/2013-03-17/tile_145.0_-20.5_145.5_-20.0_2013-03-17_.png
					my @fields = split(/\//, $line);
					@fields = split(/_/, $fields[7]);
					$west = $fields[1];
					$south = $fields[2];
					$east = $fields[3];
					$north = $fields[4];
					$time = $fields[5];
					$title = "$west+$south+$east+$north";
					$tileUrl = "$cgi?PNG+$west+$south+$east+$north+$time";					
					GroundOverlayTiles(1); # This creates both curl.sh and *.kml. Discard the KML
					$n++;
				}
			}
			$kml = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>
<kml xmlns=\"http://www.opengis.net/kml/2.2\" xmlns:gx=\"http://www.google.com/kml/ext/2.2\" xmlns:kml=\"http://www.opengis.net/kml/2.2\" xmlns:atom=\"http://www.w3.org/2005/Atom\">
<Document>
$groundOverlay	
</Document>
</kml>
";
			$outfile = "DEA_" . $layer . "_" . $time . "_" . $$ . ".kml";
			$outfile =~ s/ /_/gi;
			close(OUT); # curl.sh
			open (OUT, ">../html/WebGoogleEarth/KML/$outfile");
			print OUT $kml;
			close(OUT);
			print "$n.<small>Click to download: <a href=\"$url/$outfile?$$\">$outfile</a></small>";
			exit;
		}
		if ($sc_action eq "CurlSH") # For DEA. Create the curl.sh file to make tiles
		{
			# Usage: 
			$time = $ARGV[1];
			open (OUT, ">$basedir/$curl_sh");
			print "Content-type: text/html\n\n"; $headerAdded = 1;
			$bbox = "107.578125,-44.339565,154.687500,-10.141932"; # Australia
			@bbox = split(/,/, $bbox);
			$w = int($bbox[0]);
			$s = int($bbox[1]);
			$e = int($bbox[2]);
			$n = int($bbox[3]);
			for (my $j = $w; $j <= $e; $j++)
			{
				for (my $k = $s; $k <= $n; $k++)
				{
					for (my $j1=1; $j1 <= 2; $j1++)
					{
						$dec = 1/$j1;
						if ($dec == 1) 
						{ 
							$w1 = $j . ".0"; 
							$s1 = $k . ".0";
							$e1 = $j . ".5";
							$n1 = $k+1 . ".5";
						}
						else
						{
							$w1 = $j . ".0"; 
							$s1 = $k+1 . ".5";
							$e1 = $j . ".5";
							$n1 = $k+1 . ".0";
						}
						$tileUrl = "$cgi?PNG+$w1+$s1+$e1+$n1+$time";
						$west = $w1;
						$south = $s1;
						$east = $e1;
						$north = $n1;
#						$title = "$w1+$s1+$e1+$n1";
						$title = "$w1,$s1 $e1,$n1";
#print "$title\n";
						GroundOverlayTiles; # This creates both curl.sh and *.kml. Discard the KML
						$i++;
						if($n1 == $n) { $fin = 1; last; }
					}
				}
			}
			print "To create tiles:\n";
			print "
			<ul>
				<li>Stop the Apache server on http://130.56.242.19: '<b>service httpd stop</b>'</li>
				<li>Start the GSKY server: '<b>source /home/900/avs900/short_build.sh</b></li>
				<li>Execute: '<b>source /var/www/html/WebGoogleEarth/Tiles/curl.sh</b>'</li>
				<li>Stop the GSKY server on http://130.56.242.19:
				<ul>
					<li>
				       <b>pid=`ps -ef | grep gsky | grep -v grep | awk '{split(\$0,a,\" \"); print a[2]}'`</b>
				    </li>
					<li>
						<b>kill \$pid</b>
				    </li>
				</ul>
				</li>
				<li>Start the Apache server: '<b>service httpd start</b>'</li>
			</ul>
			</span>
			";
		}
		if ($sc_action eq "SubBBox_0") # For DEA. Get a subset of the tiles inside a sub BBox
		{
			$time = $ARGV[1];
			open (OUT, ">$basedir/$curl_sh");
			print "Content-type: text/html\n\n"; $headerAdded = 1;
			$bbox = "107.578125,-44.339565,154.687500,-10.141932"; # Australia
			@bbox = split(/,/, $bbox);
			$w = int($bbox[0]) * 2;
			$s = int($bbox[1]);
			$e = int($bbox[2]) * 2;
			$n = int($bbox[3]);
			for (my $j0 = $w; $j0 <= $e; $j0++)
			{
				$j = $j0/2.0;
				for (my $k = $s; $k <= $n; $k++)
				{
					for (my $j1=1; $j1 <= 2; $j1++)
					{
#print "$j. $k. \n";	
=head
        <west>128.0</west>
        <south>-38.0</south>
        <east>128.5</east>
        <north>-37.5</north>

        <west>128.0</west>
        <south>-37.5</south>
        <east>128.5</east>
        <north>-37.0</north>
=cut        
						$dec = 1/$j1;
						if ($dec == 1) 
						{ 
#							$w1 = $j . ".0"; 
#							$s1 = $k . ".0";
#							$e1 = $j . ".5";
#							$n1 = $k+1 . ".5";
							$w1 = sprintf("%.1f", $j); 
							$s1 = sprintf("%.1f", $k);
							$e1 = sprintf("%.1f", $j+0.5);
							$n1 = sprintf("%.1f", $k+0.5);
						}
						else
						{
#							$w1 = $j . ".0"; 
#							$s1 = $k+1 . ".5";
#							$e1 = $j . ".5";
#							$n1 = $k+1 . ".0";
							$w1 = sprintf("%.1f", $j); 
							$s1 = sprintf("%.1f", $k+0.5);
							$e1 = sprintf("%.1f", $j+0.5);
							$n1 = sprintf("%.1f", $k+1);
						}
						$tileUrl = "$cgi?PNG+$w1+$s1+$e1+$n1+$time";
						$west = $w1;
						$south = $s1;
						$east = $e1;
						$north = $n1;
						$title = "$w1,$s1 $e1,$n1";
						GroundOverlayTiles;
						$i++;
#						if($n1 == $n) { $fin = 1; last; }
					}
					if($n1 == $n) { $fin = 1; last; }
#					if($fin) {$fin = 0; last; }
				}
			}
			$kml = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>
<kml xmlns=\"http://www.opengis.net/kml/2.2\" xmlns:gx=\"http://www.google.com/kml/ext/2.2\" xmlns:kml=\"http://www.opengis.net/kml/2.2\" xmlns:atom=\"http://www.w3.org/2005/Atom\">
<Document>
$groundOverlay	
</Document>
</kml>
";
			$outfile = "DEA_" . $layer . "_" . $time . "_" . $$ . ".kml";
			$outfile =~ s/ /_/gi;
			close(OUT); # curl.sh
			open (OUT, ">../html/WebGoogleEarth/KML/$outfile");
			print OUT $kml;
			close(OUT);
			print "<small>$i. Click to download: <a href=\"$url/$outfile?$$\">$outfile</a></small>";
			exit;
		}
		if ($sc_action eq "SubBBox") # For DEA. Get a subset of the tiles inside a sub BBox
		{
			# Usage: http://www.webgenie.com/WebGoogleEarth/Dev/google_earth.cgi?SubBBox+2013-03-17
			# http://130.56.242.19/cgi-bin/google_earth.cgi?SubBBox+2013-03-17
			$time = $ARGV[1];
			if (!$time) { $time = "2013-03-17"; }
			open (OUT, ">$basedir/$curl_sh");
			print "Content-type: text/html\n\n"; $headerAdded = 1;
			$bbox = "107.578125,-44.339565,154.687500,-10.141932"; # Australia
#			$bbox = "128.803711,-38.272689,141.152344,-25.760320"; # SA
#			$bbox = "149.018555,-30.562261,154.204102,-26.941660"; # Part of QLD
#			$bbox = "141.064453,-36.562600,154.423828,-28.071980"; # NSW
#			$bbox = "111.621094,-35.532226,129.462891,-12.811801"; # WA
			@bbox = split(/,/, $bbox);
			$w = int($bbox[0]);
			$s = int($bbox[1]);
			$e = int($bbox[2]);
			$n = int($bbox[3]);
			$i=2; # Number of degrees for tile axis
			for (my $j = $w; $j <= $e; $j+=$i)
			{
				for (my $k = $s; $k <= $n; $k+=$i)
				{
					$w1 = $j . ".0";
					$s1 = $k . ".0";
					$n1 = $k+$i . ".0";
					$e1 = $j+$i . ".0";
					$tileUrl = "$cgi?PNG+$w1+$s1+$e1+$n1+$time";
					$west = $w1;
					$south = $s1;
					$east = $e1;
					$north = $n1;
					$title = "$w1+$s1+$e1+$n1";
					GroundOverlayTiles;
					$ii++;
					if($n1 == $n) { last; }
				}
			}
			$kml = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>
<kml xmlns=\"http://www.opengis.net/kml/2.2\" xmlns:gx=\"http://www.google.com/kml/ext/2.2\" xmlns:kml=\"http://www.opengis.net/kml/2.2\" xmlns:atom=\"http://www.w3.org/2005/Atom\">
<Document>
$groundOverlay	
</Document>
</kml>
";
			$outfile = "DEA_" . $layer . "_" . $time . "_" . $$ . ".kml";
			$outfile =~ s/ /_/gi;
			close(OUT); # curl.sh
			open (OUT, ">../html/WebGoogleEarth/KML/$outfile");
			print OUT $kml;
			close(OUT);
			print "<span style=\"font-family:arial; font-size:12px\">\n";
			print "Click to download: <a href=\"$url/$outfile?$$\">$outfile</a><br><br>";
=pod
			print "To create tiles:\n";
			print "
			<ul>
				<li>Stop the Apache server on http://130.56.242.19: '<b>service httpd stop</b>'</li>
				<li>Start the GSKY server: '<b>source /home/900/avs900/short_build.sh</b></li>
				<li>Execute: '<b>source /var/www/html/WebGoogleEarth/Tiles/curl.sh</b>'</li>
				<li>Stop the GSKY server on http://130.56.242.19:
				<ul>
					<li>
				       <b>pid=`ps -ef | grep gsky | grep -v grep | awk '{split(\$0,a,\" \"); print a[2]}'`</b>
				    </li>
					<li>
						<b>kill \$pid</b>
				    </li>
				</ul>
				</li>
				<li>Start the Apache server: '<b>service httpd start</b>'</li>
			</ul>
			</span>
			";
=cut			
=pod
	- Time to create 407 tiles (1x1 degree) to cover whole of Australia: 27 min.
	- Time to display the 1 degree tiles across Australia on 7Mbits connection: 2:30min
	- Time to display the 1 degree tiles across Australia on 135Mbits connection: 40 sec
	- Time to display the 1 degree tiles across Australia on 500Mbits connection: 34 sec
	- Time to create 3360 tiles (0.5x0.5 degree) to cover whole of Australia: 20 min.
	- Time to create 6460 tiles (0.5x0.5 degree) to cover whole of Australia: 42 min.
=cut
			exit;
		}
		else
		{
			&debug("OK");
		}
	}
}
$|=1;
#$domain = "webgenie.com";
$domain = "130.56.242.19";
#$cgi = "https://$domain/WebGoogleEarth/Dev/google_earth.cgi"; # On WebGenie
$cgi = "http://$domain/cgi-bin/google_earth.cgi"; # On VM19
#$basedir = "/var/www/vhosts/webgenie.com/httpdocs/WebGoogleEarth/Tiles/$time"; # On WebGenie
$basedir = "/var/www/html/WebGoogleEarth/Tiles"; # On VM19
$url = "http://$domain";
#$url = $ENV{HTTP_REFERER};
$url = $url . "/WebGoogleEarth/KML";
$aus_bboxes = "aus_bboxes.csv";
$curl_sh = "curl.sh";
$visibility = 1;  
$layer = "LS8:NBAR:TRUE";
$title = "DEA Landsat 8 surface reflectance true colour";
&do_main;
