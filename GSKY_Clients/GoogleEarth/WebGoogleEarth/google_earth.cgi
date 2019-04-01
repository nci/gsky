#!/usr/local/bin/perl
# Created on 1 Apr, 2019
# Last edit: 1 Apr, 2019
# By Dr. Arapaut V. Sivaprasad
# -----------------------------------
#use LWP::UserAgent;
#$ua = LWP::UserAgent->new;
#$browser = LWP::UserAgent->new;
#$browser->agent("Mozilla/5.0");
#$browser->timeout( 15 );
#use URI::Escape;
use DBI;

sub ConnectToDBase
{
$driver = "mysql";
$hostname = "localhost";
$database = "google_earth";
$dbuser="avs";
$dbpassword="2Kenooch";
   $dsn = "DBI:$driver:database=$database;host=$hostname";
   $dbh = DBI->connect($dsn, $dbuser, $dbpassword);
   $drh = DBI->install_driver("mysql");
}

sub execute_query
{
        $sth=$dbh->prepare($query);
        $rv = $sth->execute or die "can't execute the query: $sth->errstr";
}

sub Fetchrow_array
{
   $tablerows = $_[0];
   my @results = ();
   while(@entries = $sth->fetchrow_array)
   {
           for ($jj=0; $jj < $tablerows; $jj++)
           {
                   push (@results, $entries[$jj]);
           }
   }
   my $numreturned = scalar(@results)/$tablerows;
   return @results;
}
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
                $$1 = $2; #  Can't use string ("sc_action") as a SCALAR ref while "strict refs" is in use. Try "no strict 'refs'";
           }
   }
}
sub CreateKML
{
	$kml = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>
<kml xmlns=\"http://www.opengis.net/kml/2.2\" xmlns:gx=\"http://www.google.com/kml/ext/2.2\" xmlns:kml=\"http://www.opengis.net/kml/2.2\" xmlns:atom=\"http://www.w3.org/2005/Atom\">
<GroundOverlay>
    <name>Monthly Decile Australia</name>
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
sub do_main
{
        # Kill previous CGI
        my $psline = `ps -ef | grep google_earth.cgi | grep -v grep`;
        my @fields = split (/\s/, $psline);
        $pid = $fields[1];
        my $thispid = $$;
        if ($pid && $pid ne $thispid) { `kill $pid`; }

        my $cl = $ENV{'CONTENT_LENGTH'};
        $cl //= 0;
        if ($cl > 0)
        {
        }
        else
        {
        	print "Content-type: text/html\n\n"; $headerAdded = 1;
			$sc_action = $ARGV[0];
			$pquery = reformat($ARGV[2]);
			$pquery =~ s/\\//gi;
#&debug("pquery=$pquery");			
			Get_fields;
#&debug("time=$time");			
			if ($time)
			{
				$time="TIME=$time";
			}
			@fields = split (/\|/, $layer);
			$layer = $fields[0];
			$title = $fields[1];
#$gsky_url = "http://130.56.242.15/ows/ge?SERVICE=WMS&amp;VERSION=1.1.1&amp;REQUEST=GetMap&amp;SRS=EPSG:4326&amp;WIDTH=512&amp;HEIGHT=512&amp;LAYERS=$layer&amp;STYLES=default&amp;TRANSPARENT=TRUE&amp;FORMAT=image/png&amp;BBOX=$west,$south,$east,$north&amp;$time";
			CreateKML;
			$outfile = $region . "_" . $title . "_" . $time . ".kml";
			$outfile =~ s/ /_/gi;
			open (OUT, ">KML/$outfile");
			print OUT $kml;
			close(OUT);
			print "<small>Click to download: <a href=\"$url/$outfile\">$outfile</a></small>";
#&debug("layer=$layer");			
        }
}
$|=1;
$url = "https://www.webgenie.com/WebGoogleEarth/KML";
&do_main;
