use Data::Dumper ;
sub p
{
	# Print scalars
	my $item = $_[0];
	if(!$item) { $item = "--------------------------------------ITEM is Empty-----------------\n"; }
	my $exit = $_[1];
	$exit //= 0;
	my ( $caller_package , $caller_filename0 , $caller_line0 ) = caller () ;
	my ( $caller_package , $caller_filename1 , $caller_line1 ) = caller (1) ;
	my ( $caller_package , $caller_filename2 , $caller_line2 ) = caller (2) ;
	my ( $caller_package , $caller_filename3 , $caller_line3 ) = caller (3) ;
	my ( $caller_package , $caller_filename4 , $caller_line4 ) = caller (4) ;
#	my $line = "$caller_filename0:$caller_line0; $caller_filename1:$caller_line1; $caller_filename2:$caller_line2; $caller_filename3:$caller_line3; $caller_filename4:$caller_line4;\n";
#	my $line = "$caller_filename0:$caller_line0  $caller_filename1:$caller_line1  $caller_filename2:$caller_line2  $caller_filename3:$caller_line3  $caller_filename4:$caller_line4 \n"; 
	my $line = "Line $caller_line0:$caller_line1:$caller_line2:$caller_line3:$caller_line4\n"; 
	$line =~ s/\/usr\/local\/sbin\///gi;
	$line =~ s/\/usr\/local\/lib\/CSIRO_Cloud\///gi;
	
	print "$line<br>\n";
	print "&nbsp;&nbsp;&nbsp;$item<br>\n";
	if($exit == 1) { print "Exiting............\n"; exit; }
}
sub pd
{
	# Print arrays and hashes, sent as references. e.g. pd(\@array) or pd(\%hash)
	my $item = $_[0];
	my $exit = $_[1];
	$exit //= 0;
	$Data::Dumper::Indent = 2 ;
	$Data::Dumper::Sortkeys = 1 ;
	$Data::Dumper::Useqq = 0 ;
	$Data::Dumper::Pad = "\t" ;
	my ( $caller_package , $caller_filename0 , $caller_line0 ) = caller () ;
	my ( $caller_package , $caller_filename1 , $caller_line1 ) = caller (1) ;
	my ( $caller_package , $caller_filename2 , $caller_line2 ) = caller (2) ;
	my $line = "$caller_filename0: $caller_line0; $caller_filename1: $caller_line1; $caller_filename2: $caller_line2\n";
	$line =~ s/\/usr\/local\/sbin\///gi;
	$line =~ s/\/usr\/local\/lib\/CSIRO_Cloud\///gi;
	print $line;
	print Dumper $item;
	if($exit == 1) { exit; }
}
sub debug
{
  $line = $_[0];
  $exit = $_[1];
  if (!$headerAdded) { print "Content-type: text/html\n\n"; $headerAdded = 1; }
  print "$line<br>\n";
  if ($exit) { exit; }
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
sub numerically { $a <=> $b }
sub Monify
{
   $in = $_[0];
   $in = sprintf "%.2f", $in;
   return $in;
}
sub commify 
{
   (my $num = shift) =~ s/\G(\d{1,3})(?=(?:\d\d\d)+(?:\.|$))/$1,/g; 
   return $num; 
}
sub LWPMethod
{
	my $Url = $_[0];
	my $response = $browser->get(
	  $Url,
	  [
	  ],
	);
	my $message = $response->message;
	my $result = $response->content;
	return $result;
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
#   $chr =~ s/&/^/g;  # Replace if it is the & char.
   $tmp =~ s/%$num/$chr/g;
  }
  return($tmp);
}
sub ConnectToDBase
{
	my $database = $_[0];
	$driver = "mysql";
	$hostname = "localhost";
#	$database = "sdm"; 
	$dbuser="avs";
	$dbpassword="2Kenooch";
	$dsn = "DBI:$driver:database=$database;host=$hostname";
	$dbh = DBI->connect($dsn, $dbuser, $dbpassword);
	$drh = DBI->install_driver("mysql");
}
sub execute_query
{
	$query =~ s/[^[:ascii:]\x91-\x94\x96\x97]/ /gi;
	open(OUT, ">>lastquery.txt");
	print OUT "$query\n";
	close(OUT);
	$sth=$dbh->prepare($query);
	$rv = $sth->execute or die "can't execute the query:" . $sth->errstr;
}

sub Fetchrow_array
{
   while(my @entries = $sth->fetchrow_array)
   {
	   push (@results, @entries);
   }
   return @results;
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
1;

