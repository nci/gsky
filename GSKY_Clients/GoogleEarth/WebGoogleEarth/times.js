var times = [];
times[0] = "";
times[1] = "<select name=\"time\">\
				<option value=\"\">Latest</option>\
				<option value=\"2000-12-26T00:00:00.000Z\">2000-12-26</option>\
";
function InsertTimes(item)
{
	var i = item.selectedIndex;
	var times = times[i];
	document.getElementById("times").innerHTML = times;
	showHide("times",'block');
}
