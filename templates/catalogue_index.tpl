<!DOCTYPE html>
<html lang="en" dir="ltr">
<header>
<meta charset="utf-8">
<meta http-equiv="X-UA-Compatible" content="IE=Edge">
<title>GSKY Catalogues</title>
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="index, follow">
<style>
body {
  font-family:"Segoe UI","Fira Sans","Droid Sans","Helvetica Neue","Arial","sans-serif";
  font-size: 16px;
}

#header {
  padding-top: 18px;
  padding-bottom: 18px;
  margin-top: 0px;
  padding-left: 0px;
  padding-right: 0px;
  margin-bottom: 15px;
  background-color: #fafbfc;
  border-bottom: 0.5px solid #eeeeee; 
}

#header_container {
  width: 980px;
  margin-left: auto;
  margin-right: auto;
}

#header_container h2 {
  color: #24292e;
}

#container {
  width: 980px;
  margin-left: auto;
  margin-right: auto;
}

br{
  display: block;
  margin: 5.7px 0;
  content: " ";
  border: 0.5px solid #000000;
}

ul {
  list-style-type:none;
  margin: 0;
  padding: 0;
}

.nav li {
  display: inline-block;
  font-weight: bold;
}

.list li {
  padding-top: 10px;
  margin-left: 20px;
  padding-bottom: 10px;
  border-bottom: 0.4px solid #eeeeee;
}


.list li:hover {
    background-color: #eaecef;
}

a {
    text-decoration: none;
    display: block;
    padding-left: 5px;
}

a:link, a:visited {
    color: #0366d6;
}

a:hover {
    color: #0366d6;
    text-decoration: underline;
}
</style>
</header>
<body>
<div id="header">
  <div id="header_container">
    <h2>GSKY Catalogues</h2>
  </div>
</div>
<div id="container">
  <ul class="nav">
  {{ range $index, $nav := .Navigations }}
  <li><a href="{{ $nav.URL }}">{{ $nav.Title }}</a></li>
  <li>/</li>
  {{ end }}
  <li>{{ .Title }}</li>
  </ul>

  <br />

  <ul class="list">
  {{ range $index, $e := .Endpoints }}
  <li><a href="{{ $e.URL }}">{{ $e.Title }}</a></li>
  {{ end }}
  </ul>
</div>

</body>
</html>

