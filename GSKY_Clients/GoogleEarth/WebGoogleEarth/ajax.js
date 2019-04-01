  // ***************************************************************************
  // ///////////////////////////////////////////////////////////////////////////
  // ***************************************************************************
  //
  //  Site: Metallographic.com
  //   VERSION: 1.0
  // 
  // ***************************************************************************
  // ///////////////////////////////////////////////////////////////////////////
  // ***************************************************************************
  // Functions added by AVS
  // Copyright (c) 2011-2015 by AV Sivaprasad and WebGenie Software Pty Ltd.
// Global variables
var cgi = "google_earth.cgi"; // calls users.cgi
function showHide(id,state)
{
        if (state == undefined) state = 'block';
		var style = document.getElementById(id).style.display;
		document.getElementById(id).style.display=""+state;
}
function ajaxFunction(n,form,item)
{
  var xmlHttp;
  var url;
  try
  {  // Firefox, Opera 8.0+, Safari  
	  xmlHttp=new XMLHttpRequest();  
  }
  catch (e)
  {  // Internet Explorer  
    try
    {    
		xmlHttp=new ActiveXObject("Msxml2.XMLHTTP");    
	}
	catch (e)
    {    
		try
		{      
			xmlHttp=new ActiveXObject("Microsoft.XMLHTTP");      
		}
		catch (e)
      	{	      
			alert("Your browser does not support AJAX!");      
			return false;      
		}    
	}
  }
	xmlHttp.onreadystatechange=function()
	{
//alert(xmlHttp.readyState);
	  if(xmlHttp.readyState==4)
	  {
 		  response = xmlHttp.responseText;
		  if (n == 1) // KML
		  {
//alert('n = ' + n + ' response = ' + response);
			document.getElementById("kml").innerHTML = response;
			showHide("kml", "block");
		  }
	  }
	}
	if (n == 1) // KML
	{
		pquery = 
		"&layer=" + form.layer.value +
		"&region=" + form.region.value +
		"&west=" + form.west.value +
		"&south=" + form.south.value +
		"&east=" + form.east.value +
		"&north=" + form.north.value +
		"&time=" + form.time.value;
		pquery = escape(pquery);
		pquery = pquery.replace("+","%2B");
//alert(layer);		
		var ran_number= Math.random()*5000;
		url = cgi + "?createKML+" + ran_number + "+" + pquery;
//alert(url);
	}
//alert('n = ' + n + ' url = ' + url);	
//return;
	if (url)
	{
		xmlHttp.open("GET",url,true);
		xmlHttp.send(null);  
	}
	else
	{
		return;
	}
}

