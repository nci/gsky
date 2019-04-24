#!/usr/bin/env perl
# Created on 31 Mar, 2019
# Last edit: 3 Apr, 2019
# By Dr. Arapaut V. Sivaprasad
=pod
This CGI is for creating the KMLs for displaying the GSKY layers via Google Earth Web.
See http://www.webgenie.com/WebGoogleEarth/
=cut
# -----------------------------------
require "common.pl";
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
#p($bbox);	
	# To create the multi "GroundOverlay" KML for displaying the DEA tiles
	my $n_tiles = $_[0];
	my $title = $_[1];
	my $skip_curl = $_[2];
#p($title);
	$groundOverlay .= "
$placemark
<!-- $n_tiles -->
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
		print OUT "echo $n_tiles\n"; $n_tiles--;
		print OUT "curl '$gskyUrl&BBOX=0,0,0,0'\n";
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
#&debug("request_string = $request_string",1);	
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
			if ($sc_action eq "WMS")
			{
# http://130.56.242.19/cgi-bin/google_earth.cgi?WMS+landsat8_nbar_16day+136.8,-34.8,136.9,-34.7+2013-06-07+0.1
				$layer = $fields[1];
				$bbox = $fields[2];
				$time = $fields[3];
				$r = $fields[4];
				$create_tiles = $fields[5];
			}
		}
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
		if ($sc_action eq "DeleteEmptyTiles") # For DEA. Delete the PNG files that are empty
		{
			# Usage: ./google_earth.cgi DeleteEmptyTiles resolution
			# e.g. ./google_earth.cgi DeleteEmptyTiles 1
			# Usage: /var/www/cgi-bin/google_earth.cgi
			$td = ".";
			$ls = `ls -l $td`;
			my @ls = split(/\n/, $ls);
			my $len = $#ls;
			$n = 0;
			for (my $j=0; $j <= $len; $j++)
			{
				my $line = $ls[$j];
				$line =~ tr/  / /s;
				my @fields = split(/ /, $line);
				if ($fields[4] == 2132 || $fields[4] == 5141) # Empty tiles
				{
					$filename = "$td/$fields[8]"; 
					$n++;
					print "$n.	unlink ($filename) - $fields[4]\n";
					unlink ($filename);
				}
			}
			exit;
		}
		if ($sc_action eq "Help") # Help to create the tiles. Out of date.
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
		sub CountTheTilesLow 
		{
			my $w = $_[0];
			my $s = $_[1];
			my $e = $_[2];
			my $n = $_[3];
			my $r = $_[4];
			my $layer = $_[5];
			$n_tiles = 0;
			for (my $j = $w; $j < $e; $j+=$r)
			{
				for (my $k = $s; $k < $n; $k+=$r)
				{
					$w1 = sprintf("%.1f", $j); 
					$s1 = sprintf("%.1f", $k);
					$e1 = sprintf("%.1f", $j+$r);
					$n1 = sprintf("%.1f", $k+$r);
					$tile_filename = $w1 . "_" . $s1 . "_" . $e1 . "_" . $n1 . "_" . $time . "_$r" . ".png";
					$tile_file = "$basedir/$layer/$time/$r/$tile_filename";
					$tileurl = "http://$domain/GEWeb/DEA_Layers/$layer/$time/$r/" . $w1 . "_" . $s1 . "_" . $e1 . "_" . $n1 . "_" . $time . "_$r" . ".png";
					if (!-f $tile_file)
					{
						if (!$create_tiles)
						{
							next;
						}
					}
					$n_tiles++;
					if($n1 >= $n) { last; }
				}
			}
			&debug("Actual number of tiles: <big>$n_tiles</big>");
			if ($n_tiles <= 0)
			{
				&debug("<font style=\"color:red; font-size:12px\">No tiles in the selected region. Please choose another region.</font>");
			}
#			if ($n_tiles > 300)
#			{
#				&debug("<font style=\"color:red; font-size:12px\">On a slow internet connection this could take a long time to display and/or crash Google Earth.<br>Please consider choosing a smaller region or a lower resolution.</font>");
#			}
		}
		sub ElapsedTime
		{
			my $n = $_[0];
			$ct1 = time();
			$et = $ct1 - $ct0;
			&debug ("$n. $et sec.");
			$ct0 = $ct1;
		}
		sub CountTheTiles 
		{
			my $w = $_[0];
			my $s = $_[1];
			my $e = $_[2];
			my $n = $_[3];
			my $r = 0.1;
			my $m = int(1/$r);
			GetTheLargeTile($w,$s,$e,$n,$r); # Find the 3x3 tile that covers this bbox
			my $n_tiles = 0;
			for (my $j0 = $w; $j0 < $e; $j0++)
			{
				$j = $j0/$m;
				for (my $k0 = $s; $k0 < $n; $k0++)
				{
					$fin = 0;
					my $k = $k0/$m;
					my $w1 = sprintf("%.1f", $j); 
					my $s1 = sprintf("%.1f", $k);
					my $e1 = sprintf("%.1f", $j+$r);
					my $n1 = sprintf("%.1f", $k+$r);
					my $this_tile = $w1 . "_" . $s1 . "_" . $e1 . "_" . $n1;
					my $tile_filename = $w1 . "_" . $s1 . "_" . $e1 . "_" . $n1 . "_" . $time . "_$r" . ".png";
					GetHash($tile_filename,0.1,$r);
					if ($tilesHash{$this_tile}) # %tilesHash is global
					{
						$n_tiles++;
					}
				}
			}
			&debug("Actual number of tiles: <big>$n_tiles</big>");
			if ($n_tiles <= 0)
			{
				&debug("<font style=\"color:red; font-size:12px\">No tiles in the selected region. Please choose another region.</font>");
			}

#			if ($n_tiles > 50 && $n_tiles <= 100)
#			{
#				&debug("<font style=\"color:red; font-size:12px\">This could take a long time to fetch the tiles.<br>Please consider choosing a smaller region or a lower resolution.</font>");
#			}
			
			if ($n_tiles > 100)
			{
				&debug("<font style=\"color:red; font-size:12px\">Too many tiles to be fetched. A smaller BBox is required for high resolution.</font>");
				&debug("<font style=\"color:lime; font-size:12px\">Giving Up!</font>");
				exit;
			}
		}
		sub DEA_High
		{
			my $layer = $_[0];
			if (!$time) { $time = "2013-03-17"; } # $time is global, coming from the HTML page
			my @bbox = split(/,/, $bbox);
			my $r = $resolution;
			my $m = int(1/$r);
			my $w = int($bbox[0] * $m);
			my $s = int($bbox[1] * $m);
			my $e = int($bbox[2] * $m);
			my $n = int($bbox[3] * $m);
			
			# For High Res alone, we need a place mark in the middle of the tiled area.
			my $pmx = $bbox[0] + (($bbox[2] - $bbox[0])/2);
			my $pmy = $bbox[1] + (($bbox[3] - $bbox[1])/2);
			$placemark = "<Placemark>
	<Point>
	  <coordinates>$pmx,$pmy,0</coordinates>
	</Point>
</Placemark>
";
#p("$pmx, $pmy");			
			CountTheTiles($w,$s,$e,$n);
			my @keys = sort keys %tilesHash;
			my $n_tiles = 0;
			foreach my $key (@keys)
			{
				if($tilesHash{$key})
				{
					my $tile = $key;
					$tile =~ s/_/,/g;
					my @bbox = split(/,/,$tile);
					$west = $bbox[0];
					$south = $bbox[1];
					$east = $bbox[2];
					$north = $bbox[3];
					$n_tiles++;
					$tile_file = "$localdir/GEWeb/DEA_Layers/$layer/$time/$r/$west" . "_" . $south . "_" . $east . "_" . $north . "_" . $time . "_" . $r . ".png";
					if (!-f $tile_file)
					{
						my $gskyFetchUrl = "http://$domain/cgi-bin/google_earth.cgi?WMS+$layer+$tile+$time+$r&BBOX=0,0,0,0";
						`curl '$gskyFetchUrl'`; # Fetch and write the PNG file
					}
		            $gskyUrl = "http://$domain/GEWeb/DEA_Layers/$layer/$time/$r/$west" . "_" . $south . "_" . $east . "_" . $north . "_" . $time . "_" . $r . ".png";
					if ($callGsky)
					{
						$tileUrl = $gskyUrl;
						$tileUrl =~ s/&/&amp;/gi;
					}
					$title = "$west,$south,$east,$north R$r";
					GroundOverlayTiles($n_tiles,$title);
					$placemark = ""; # Blank this for next round
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
			Get_fields;	# Parse the $pquery to get all form input values
			my $r = $resolution;
			my @fields = split (/\|/, $layer);
			my $layer = $fields[0];
			my $title = $fields[1];
			my $basetitle = $title;
			if (!$time) { $time = "2013-03-17"; } # $time is global, coming from the HTML page
			else
			{
				$time =~ s/T.*$//gi;
			}
			my $create_tiles_dir = "$localdir/GEWeb/DEA_Layers/$layer/$time/$r";
#&debug($create_tiles_dir);
			if (!-d "$create_tiles_dir")
			{
				`mkdir -p "$create_tiles_dir"`;
			}
			if ($create_tiles) 
			{ 
				open (OUT, ">$create_tiles_dir/$create_tiles_sh"); 
				print OUT "echo \"$create_tiles_dir/$create_tiles_sh\"\n";
			}
			my $i=$resolution; # Number of degrees for tile axis
			if ($i < 1)
			{
				DEA_High($layer);
			}
			@bbox = split(/,/, $bbox); # $bbox is global, coming from the HTML page
			my $w = int($bbox[0]);
			my $s = int($bbox[1]);
			my $e = int($bbox[2]);
			my $n = int($bbox[3]);
			# The w,s,e,n values must match the tiles created for this resolution
			# The Lon/Lat values must be divisible with the value of resolution.
			# For example, the 2x2 degree tiles will use even numbers for both Lon and Lat. e.g. 110,-44,112,-42; 112,-42,114,-40;
			# The 3x3 tiles will use multiples of 3. e.g. 111,-45,114,-42; 111,-42,114,-39
			$w -= $w % $i; # e.g. 11%3 = 2. 11-2=9; 9%3 = 0
			$s -= $s % $i; 
			$e -= $e % $i; 
			$n -= $n % $i; 
			if ($w == $e) { $e+=$i; }
			if ($s == $n) { $n+=$i; }
			CountTheTilesLow($w,$s,$e,$n,$i,$layer);
			for (my $j = $w; $j < $e; $j+=$i)
			{
				for (my $k = $s; $k < $n; $k+=$i)
				{
					$w1 = sprintf("%.1f", $j); 
					$s1 = sprintf("%.1f", $k);
					$e1 = sprintf("%.1f", $j+$i);
					$n1 = sprintf("%.1f", $k+$i);
					$tile_filename = $w1 . "_" . $s1 . "_" . $e1 . "_" . $n1 . "_" . $time . "_$r" . ".png";
					$tile_file = "$basedir/$layer/$time/$r/$tile_filename";
					if (!$create_tiles && !-f $tile_file)
					{
						next;
					}
					$tileUrl = "$cgi?PNG+$w1+$s1+$e1+$n1+$time+$i";
					$west = $w1;
					$south = $s1;
					$east = $e1;
					$north = $n1;
#		            $gskyUrl = "http://$domain/cgi-bin/google_earth.cgi?WMS+$layer+$west,$south,$east,$north+$time+$r+$create_tiles";
		            $gskyUrl = "http://$domain/GEWeb/DEA_Layers/$layer/$time/$r/$west" . "_" . $south . "_" . $east . "_" . $north . "_" . $time . "_" . $r . ".png";
					if ($callGsky)
					{
						$tileUrl = $gskyUrl;
						$tileUrl =~ s/&/&amp;/gi;
					}
					$title = "$w1,$s1,$e1,$n1 R$i";
					GroundOverlayTiles($title);
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
			if ($create_tiles) 
			{ 
				print OUT "cd '$create_tiles_dir'\n"; 
				print OUT "/var/www/cgi-bin/google_earth.cgi DeleteEmptyTiles\n"; 
				print OUT "echo \"Finished!\"\n";
				print OUT "/bin/date\n";
				close(OUT); 
				`mv "$create_tiles_dir/$create_tiles_sh" $localdir/GEWeb`;
			}
			open (OUT, ">$docroot/WebGoogleEarth/KML/$outfile");
			print OUT $kml;
			close(OUT);
			print "<span style=\"font-family:arial; font-size:12px\">\n";
			print "Click to download: <a href=\"$url/$outfile?$$\">$outfile</a>";
			exit;
		}
		if ($sc_action eq "WMS") # This is called for create_tiles and DEA_HIGH.
		{
			$imgdir = "$localdir/GEWeb/DEA_Layers/$layer/$time/$r";
			if (!-d $imgdir)
			{
				`mkdir -p $imgdir`;
			}
			$imgfile = "$imgdir/" . $bbox . "_" . $time . "_" . $r . ".png";
			$imgurl = "http://$domain/GEWeb/DEA_Layers/$layer/$time/$r/" . $bbox . "_" . $time . "_" . $r . ".png";
			$imgfile =~ s/,/_/gi;
			$imgurl =~ s/,/_/gi;
			if (-f $imgfile && !$create_tiles)
			{
				print "Location: $imgurl\n\n";
				exit;
			}
			else
			{
				$url = "https://gsky.nci.org.au/ows/dea?SERVICE=WMS&VERSION=1.1.1&REQUEST=GetMap&SRS=EPSG:4326&WIDTH=512&HEIGHT=512&LAYERS=$layer&STYLES=&TRANSPARENT=TRUE&FORMAT=image/png&BBOX=$bbox&TIME=$time" . "T00:00:00.000Z";
				$png = `curl '$url'`;
				if ($png)
				{
					open (OUT, ">$imgfile");
					print OUT $png;
					close (OUT);
					if (!$create_tiles)
					{
						print "Location: $imgurl\n\n";
					}
					else
					{
						print "Content-type: text/html\n\n"; $headerAdded = 1;
						print "Created: $ii. $imgfile\n";
					}
				}
				exit;
			}
		}
		sub CreateTilesForThisLayer
		{
			print "Content-type: text/html\n\n"; $headerAdded = 1;
			my @times = split(/,/,$times);
			my $len = $#filecontent;
			foreach my $date (@times)
			{
				$date =~ s/T.*Z//g;
				print "$date\n";
				for (my $j=0; $j <= $len; $j++)
				{
					my $line = $filecontent[$j];
					chop ($line);
					$line =~ s/\$layer/$layer/g;
					$line =~ s/\$date/$date/g;
					my $res = `$line`;
					if ($res) { print OUT "$res"; }
				}
			}
		}
		if ($sc_action eq "CreateAllTiles") # Read a file to create the tiles (3x3 deg) for all layers and time slices
		{
			open(INP, "<$localdir/GEWeb/create_tiles_tem.sh");
			@filecontent = <INP>;
			close(INP);
			open (OUT, ">>/var/www/cgi-bin/logs.txt");
			open (INP, "<$localdir/GEWeb/layers.txt");
			@layers = <INP>;
			close(INP);
			foreach $line (@layers)
			{
				if($line =~ /^#/) { next; }
				if($line =~ /Name:(.*)\n/)
				{
					$layer = $1;
					&debug("layer = $layer");						
				}
				if($line =~ /Title:(.*)\n/)
				{
					$title = $1;
				}
				if($line =~ /Times:(.*)\n/)
				{
					$times = $1;
					CreateTilesForThisLayer;
				}
			}
			close(OUT);
		}

		sub GetTheLargeTile
		{
			my $i = 3; # The tile res
			my $r = $_[4];  
			my $m = int(1/$r);
			my $w = int($_[0]/$m);
			my $s = int($_[1]/$m);
			my $e = int($_[2]/$m);
			my $n = int($_[3]/$m);

			$w -= ($w % $i);
			$s -= ($s % $i);
			$e -= ($e % $i);
			$n -= ($n % $i);
			$ii = 0;
			for (my $j = $w; $j < $e; $j+=$i)
			{
				for (my $k = $s; $k < $n; $k+=$i)
				{
					$w1 = sprintf("%.1f", $j); 
					$s1 = sprintf("%.1f", $k);
					$e1 = sprintf("%.1f", $j+$i);
					$n1 = sprintf("%.1f", $k+$i);
					$tile_filename = $w1 . "_" . $s1 . "_" . $e1 . "_" . $n1 . "_" . $time . "_$r" . ".png";
					$tile_file = "$basedir/$layer/$time/$r/$tile_filename";
					GetLargeHash($tile_filename,3,$r);
					$tileurl = "http://$domain/GEWeb/DEA_Layers/$layer/$time/3/" . $w1 . "_" . $s1 . "_" . $e1 . "_" . $n1 . "_" . $time . "_3" . ".png";
					if($n1 >= $n) { last; }
				}
			}
		}
		
		sub GetLargeHash # Make a hash of the 3x3 tiles within the bbox
		{
			my $filename = $_[0];
			my $rl = $_[1];
			my $r = $_[2];
			%tilesHash = {};
			@bbox = split (/_/, $filename);
			$m = int(1/$r);
			my $w = int($bbox[0]) * $m;
			my $s = int($bbox[1]) * $m;
			my $e = (int($bbox[2]) - $r) * $m;
			my $n = (int($bbox[3]) - $r) * $m;
			for (my $j0 = $w; $j0 < $e; $j0++)
			{
				$j = $j0/$m;
				for (my $k0 = $s; $k0 < $n; $k0++)
				{
					$fin = 0;
					$k = $k0/$m;
					my $w1 = sprintf("%.1f", $j); 
					my $s1 = sprintf("%.1f", $k);
					my $e1 = sprintf("%.1f", $j+$r);
					my $n1 = sprintf("%.1f", $k+$r);
					$sub_tile = $w1 . "_" . $s1 . "_" . $e1 . "_" . $n1;
					if ($rl == 3) 
					{ 
						$ii++;
						$tilesHash{$sub_tile} = 0; 
					}
					if ($rl == 0.1) 
					{ 
						$ii++;
						$tilesHash{$sub_tile} = 1; 
					}
				}
			}
		}
		sub GetHash # Make a hash of the 0.1x0.1 tiles within the bbox
		{
			my $filename = $_[0];
			my $rl = $_[1];
			my $r = $_[2];
			my @bbox = split (/_/, $filename);
			my $m = int(1/$r);
			my $w = int($bbox[0] * $m);
			my $s = int($bbox[1] * $m);
			my $ee = int($bbox[2] * $m);
			my $n = int($bbox[3] * $m);
			for (my $j0 = $w; $j0 < $ee; $j0++)
			{
				my $j = $j0/$m;
				for (my $k0 = $s; $k0 < $n; $k0++)
				{
					my $k = $k0/$m;
					my $w1 = sprintf("%.1f", $j); 
					my $s1 = sprintf("%.1f", $k);
					my $e1 = sprintf("%.1f", $j+$r);
					my $n1 = sprintf("%.1f", $k+$r);
					my $sub_tile = $w1 . "_" . $s1 . "_" . $e1 . "_" . $n1;
					$tilesHash{$sub_tile} = 1; 
					if($n1 >= $n) { last; }
				}
			}
		}
		if ($sc_action eq "CountSubTiles") # Determine whether a 0.1x0.1 tile is within a 3x3 tile
		{
		}
		if ($sc_action eq "Kill")
		{
			# Kill previous CGI
			$pquery = reformat($ARGV[2]);
			$pquery =~ s/\\//gi;
#&debug("pquery=$pquery");
			Get_fields;	# Parse the $pquery to get all form input values
#			@fields = split (/\|/, $layer);
#			$layer = $fields[0];
			print "Content-type: text/html\n\n"; $headerAdded = 1;
			$layer =~ s/\|/\\|/g;
			my $pscmd = "ps -ef | grep \"/var/www/cgi-bin/google_earth.cgi DEA.*$layer\" | grep -v grep";
			my $psline = `$pscmd`;
			$psline =~ tr/  / /s;
#print "pscmd=$pscmd\n";
			my @fields = split (/\s/, $psline);
			$pid = $fields[1];
#print "$pid\n";
			my $thispid = $$;
#print "$thispid\n";
			if ($pid && $pid ne $thispid) 
			{ 
				`kill $pid`;
				print "<font style=\"color:#FF0000\">Killed the process. ID = <b>$pid</b></font>";
			}
			else
			{
				print "Could not find any process to kill.\n";
			}
		}
		
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
$basedir = "$docroot/GEWeb/DEA_Layers";
$localdir = "/local";
$url = "http://$domain";
#$url = $ENV{HTTP_REFERER};
$url = $url . "/WebGoogleEarth/KML";
$aus_bboxes = "aus_bboxes.csv";
$create_tiles_sh = "create_tiles.sh";
$visibility = 1;  
#$layer = "LS8:NBAR:TRUE";
$layer = "landsat8_nbart_16day";
$title = "DEA Landsat 8 surface reflectance true colour";
$callGsky = 1; # Use GetMap calls to GSKY instead of using the PNG files at high res
$ct0 = time();
&do_main;
=pod
Cache location: C:\Users\avs29\AppData\Local\Google\Chrome\User Data\Default\Cache
=cut
