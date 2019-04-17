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
	# To create the multi "GroundOverlay" KML for displaying the DEA tiles
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
	if ($create_tiles && !$skip_curl)
	{
		print OUT "echo $nt\n"; $nt++;
#$gskyUrl = "http://$ows_domain/ows/ge?SERVICE=WMS&VERSION=1.1.1&REQUEST=GetMap&SRS=EPSG:4326&WIDTH=512&HEIGHT=512&LAYERS=$layer&STYLES=default&TRANSPARENT=TRUE&FORMAT=image/png&BBOX=$west,$south,$east,$north&TIME=$time" . "T00:00:00.000Z";
		print OUT "curl '$gskyUrl&OUTPUT=PNG_FILE'\n";
#		print OUT "curl 'http://$domain/ows/ge?SERVICE=WMS&VERSION=1.1.1&REQUEST=GetMap&SRS=EPSG:4326&WIDTH=512&HEIGHT=512&LAYERS=$layer&STYLES=default&TRANSPARENT=TRUE&FORMAT=image/png&BBOX=$west,$south,$east,$north&$time'\n";
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
	# For the GEOGLAM Tiles. Called from geoglam.html as below.
	# <input type="button" value="Create KML" style="color:blue" onclick="ValidateInput(document.forms.google_earth,1);">
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
	open (OUT, ">$create_tiles_sh");
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
#&debug($sc_action);
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
				$res = $fields[6];
			}
		}
#&debug("sc_action = $sc_action");			
		if ($sc_action eq "GEOGLAM")
		{
=pod
	For GEOGLAM the call from geoglam.html will create a KML with GSKY call as..
         http://130.56.242.15/ows/ge?SERVICE=WMS&amp;VERSION=1.1.1&amp;REQUEST=GetMap&amp;SRS=EPSG:4326&amp;WIDTH=512&amp;HEIGHT=512&amp;LAYERS=global:c6:frac_cover&amp;STYLES=default&amp;TRANSPARENT=TRUE&amp;FORMAT=image/png&amp;BBOX=112.324219,-44.087585,153.984375,-10.919618&amp;

    The BBox values specified in the above call will be used to generate the PNG
    file on the flye as in the case of a GetMap request to the GSKY server.
	
=cut
			
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
				open (OUT, ">$docroot/WebGoogleEarth/KML/$outfile");
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
				open (OUT, ">$docroot/WebGoogleEarth/KML/$outfile");
				print OUT $kml;
				close(OUT);
				print "<small>Click to download: <a href=\"$url/$outfile\">$outfile</a></small>";
			}
			exit;
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

How to create the tiles:

1. Open the web page at: http://130.56.242.19/dea.html

	- Use "Australia" and the required resolution.
	- Choose other parameters an click 'Create KML'
	- Run 'curl.sh' at commandline
		- source /home/900/avs900/WebGoogleEarth/Tiles/curl.sh
	- Create a subdir in /home/900/avs900/WebGoogleEarth/Tiles/2017-03-17 as the tile res...
		- 3
		- 2
		- 1
		- 0.5
		- 0.2
		- 0.1
	- Copy the tiles from /home/900/avs900/WebGoogleEarth/Tiles/2017-03-17
		- to the respective sub dir with it.

How to display the layers on GEWeb:

1. Open the web page at: http://130.56.242.19/dea.html

2. Choose "Australia" or any of the smaller regions.

3. Choose other parameters an click 'Create KML'

4. Download and save the KML

5. Open it in GEWeb


=cut
		if ($sc_action eq "DeleteEmptyTiles") # For DEA. Delete the PNG files that are empty
		{
			# Usage: ./google_earth.cgi DeleteEmptyTiles resolution
			# e.g. ./google_earth.cgi DeleteEmptyTiles 1
			$res = $ARGV[1];
			if (!$res)
			{
				print "Must specify the resolution.";
				exit;
			}
			# Usage: /var/www/cgi-bin/google_earth.cgi DeleteEmptyTiles 3
			$td = "$localdir/2013-03-17/$res";
			$ls = `ls -l $td`;
			my @ls = split(/\n/, $ls);
			my $len = $#ls;
			$n = 0;
			for (my $j=0; $j <= $len; $j++)
			{
				my $line = $ls[$j];
				$line =~ tr/  / /s;
				my @fields = split(/ /, $line);
				if ($fields[4] == 2132) # Empty tiles
				{
					$filename = "$td/$fields[8]"; 
					$n++;
					print "$n.	unlink ($filename) - $fields[4]\n";
					unlink ($filename);
				}
			}
			exit;
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
				$res = $ARGV[6];
			}
			# Open the file 
			$tilefile = "$localdir/$time/$res/tile_" . $west . "_" . $south . "_" . $east . "_" . $north . "_" . $time . "_" . ".png";
			eval 
			{
				select(STDOUT); $| = 1;   #unbuffer STDOUT
				print "Content-type: image/png\n\n";
				open (IMAGE, '<', $tilefile);
				print <IMAGE>;
				close(IMAGE);
			};
		}
		if ($sc_action eq "Help") # Help to create the tiles
		{
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
		sub DEA_High
		{
			$pquery = reformat($ARGV[2]);
			$pquery =~ s/\\//gi;
			Get_fields;	# Parse the $pquery to get all form input values
			@fields = split (/\|/, $layer);
			$layer = $fields[0];
			$title = $fields[1];
			$basetitle = $title;
			if (!$time) { $time = "2013-03-17"; }
			@bbox = split(/,/, $bbox);
			$r = $resolution;
			$m = int(1/$r);
			$w = int($bbox[0]) * $m;
			$s = int($bbox[1]) * $m;
			$e = int($bbox[2]) * $m;
			$n = int($bbox[3]) * $m;
			open (INP, "<$localdir/$time/$resolution/tiles.txt");
			my @filecontent = <INP>;
			close (INP);
			my $len = $#filecontent;
			$filecontent = join("|", @filecontent);
			$filecontent =~ s/\n//gi;
#&debug("Include: $len; $localdir/$time/$resolution/tiles.txt");	
#&debug("Include: $filecontent");	
			for (my $j0 = $w; $j0 <= $e; $j0++)
			{
				$j = $j0/$m;
				for (my $k0 = $s; $k0 <= $n; $k0++)
				{
					$fin = 0;
					$k = $k0/$m;
					$w1 = sprintf("%.1f", $j); 
					$s1 = sprintf("%.1f", $k);
					$e1 = sprintf("%.1f", $j+$r);
					$n1 = sprintf("%.1f", $k+$r);
					$tile_filename = "tile_" . $w1 . "_" . $s1 . "_" . $e1 . "_" . $n1 . "_" . $time . "_.png";
#&debug("Check: $tile_filename");	
					if ($tile_filename !~ /$filecontent/)
					{
						if (!$create_tiles)
						{
#&debug("Skip: $tile_filename");	
							next;
						}
					}
#					$tile_file = "$localdir/$time/$resolution/$tile_filename";
#					if (!$create_tiles && !-f $tile_file)
#					{
#&debug("Skip: $tile_file");	
#						next;
#					}
					$tileUrl = "$cgi?PNG+$w1+$s1+$e1+$n1+$time+$r";
#&debug("tileUrl: $tileUrl");	
					$west = $w1;
					$south = $s1;
					$east = $e1;
					$north = $n1;
					$gskyUrl = "http://$ows_domain/ows/ge?SERVICE=WMS&VERSION=1.1.1&REQUEST=GetMap&SRS=EPSG:4326&WIDTH=512&HEIGHT=512&LAYERS=$layer&STYLES=default&TRANSPARENT=TRUE&FORMAT=image/png&BBOX=$west,$south,$east,$north&TIME=$time" . "T00:00:00.000Z";
					if ($r <= 0.5)
					{
						$tileUrl = $gskyUrl;
						$tileUrl =~ s/&/&amp;/gi;
					}
					$title = "$w1,$s1 $e1,$n1 R $r";
					GroundOverlayTiles;
					$ii++;
					if($n1 >= $n/$m) { last; }
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
			open (OUT, ">$docroot/WebGoogleEarth/KML/$outfile");
			print OUT $kml;
			close(OUT);
			print "<small>Click to download: <a href=\"$url/$outfile?$$\">$outfile</a></small>";
			exit;
		}
		if ($sc_action eq "DEA") # For DEA from dea.html
		{
			# To create the DEA tiles. Called from dea.html as below.
			# <input type="button" value="Create KML" style="color:blue" onclick="ValidateInput(document.forms.google_earth,2);">
			print "Content-type: text/html\n\n"; $headerAdded = 1;
			$pquery = reformat($ARGV[2]);
			$pquery =~ s/\\//gi;
#&debug("pquery = $pquery");
			Get_fields;	# Parse the $pquery to get all form input values
			if (!$time) { $time = "2013-03-17"; }
#&debug("create_tiles = $create_tiles: $localdir/$time/$create_tiles_sh");
#open (OUT, ">$localdir/$time/$create_tiles_sh");
#open (OUT, ">$localdir/$time/$create_tiles_sh");
			if ($create_tiles) { open (OUT, ">$localdir/$time/$create_tiles_sh"); }
			$i=$resolution; # Number of degrees for tile axis
			if ($i < 1)
			{
				DEA_High;
			}
			@fields = split (/\|/, $layer);
			$layer = $fields[0];
			$title = $fields[1];
			$basetitle = $title;
#			if (!$time) { $time = "2013-03-17"; }
			@bbox = split(/,/, $bbox);
			$w = int($bbox[0]);
			$s = int($bbox[1]);
			$e = int($bbox[2]);
			$n = int($bbox[3]);
			# The w,s,e,n values must match the tiles created for this resolution
			# The Lon/Lat values must be divisible with the value of resolution.
			# For example, the 2x2 degree tiles will use even numbers for both Lon and Lat. e.g. 110,-44,112,-42; 112,-42,114,-40;
			# The 3x3 tiles will use multiples of 3. e.g. 111,-45,114,-42; 111,-42,114,-39
			$w -= $w % $i; # e.g. 11%3 = 2. 11-2=9; 9%3 = 0
			$s -= $s % $i; 
			$e -= $e % $i; 
			$n -= $n % $i; 
#&debug("create_tiles = $create_tiles");
			for (my $j = $w; $j <= $e; $j+=$i)
			{
				for (my $k = $s; $k <= $n; $k+=$i)
				{
					$w1 = sprintf("%.1f", $j); 
					$s1 = sprintf("%.1f", $k);
					$e1 = sprintf("%.1f", $j+$i);
					$n1 = sprintf("%.1f", $k+$i);
					$tile_file = "$localdir/$time/$resolution/tile_" . $w1 . "_" . $s1 . "_" . $e1 . "_" . $n1 . "_" . $time . "_.png";
					if (!$create_tiles && !-f $tile_file)
					{
#&debug("Skip: $tile_file");	
						next;
					}
					$tileUrl = "$cgi?PNG+$w1+$s1+$e1+$n1+$time+$i";
					$west = $w1;
					$south = $s1;
					$east = $e1;
					$north = $n1;
					$title = "$w1,$s1 $e1,$n1 R$i";
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
			if ($create_tiles) { close(OUT); } # curl.sh
			open (OUT, ">$docroot/WebGoogleEarth/KML/$outfile");
			print OUT $kml;
			close(OUT);
			print "<span style=\"font-family:arial; font-size:12px\">\n";
			print "Click to download: <a href=\"$url/$outfile?$$\">$outfile</a>";
			exit;
		}
=pod
	- Time to create 1632 tiles (1x1 degree) to cover whole of Australia: 30 min.
	- Time to create 407 tiles (2x2 degree) to cover whole of Australia: 27 min.
	- Time to create 192 tiles (3x3 degree) to cover whole of Australia: 22 min.
	- Time to create 3360 tiles (0.5x0.5 degree) to cover whole of Australia: 20 min.
	- Time to create 6460 tiles (0.5x0.5 degree) to cover whole of Australia: 42 min.

	- Time to display the 1 degree tiles across Australia on 7Mbits connection: 2:30min
	- Time to display the 1 degree tiles across Australia on 135Mbits connection: 40 sec
	- Time to display the 1 degree tiles across Australia on 500Mbits connection: 34 sec
	- Time to display the 3 degree tiles across Australia on 500Mbits connection: 15 sec
=cut
		else
		{
			&debug("OK");
		}
	}
}
$|=1;
#$domain = "webgenie.com";
$domain = $ENV{HTTP_HOST};
$ows_domain = "130.56.242.15";
$docroot = $ENV{DOCUMENT_ROOT};
if (!$docroot) { $docroot = "/var/www/html"; }
$cgi = "http://$domain/cgi-bin/google_earth.cgi"; # On VM19
$basedir = "$docroot/WebGoogleEarth/Tiles"; # On VM19
$localdir = "/local/avs900";
$url = "http://$domain";
#$url = $ENV{HTTP_REFERER};
$url = $url . "/WebGoogleEarth/KML";
$aus_bboxes = "aus_bboxes.csv";
$create_tiles_sh = "create_tiles.sh";
$visibility = 1;  
$layer = "LS8:NBAR:TRUE";
$title = "DEA Landsat 8 surface reflectance true colour";
&do_main;
