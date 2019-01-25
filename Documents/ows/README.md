DOCS For GSKY 
============================================================
(Distributed Scalable Geospatial Data Server)
---

INTRODUCTION
-------------

GSKY was developed at [NCI](http://nci.org.au) and is a scalable, distributed 
server which presents a new approach for geospatial data discovery and delivery 
using OGC standards. The most recent release is version 1.0 (June 2018).

The **documents/ows** directoy has all the documents and user guides for the GSKY system. If you find that
anything is missing or requires modifications, please contact the author directly or through the comments.

The documents - in Word, PowerPoint and PDF - listed in here are intended to 
be equally useful for a beginner and an expert of GSKY. Though they were created
mainly for the developers of the system(s), some will function as user guides for the end users.

These docs are the by-products of the author's very painstaking travels through the GSKY and TerriaMap codes and 
other potential GSKY clients. Hopefully they will be useful for other developers and users of GSKY. 

These docs do not comply with the new philosophy of 
*"why make things simple and easily understood when they can be wonderfully complex, elegant and unintelligible?"* (no apologies).

The motivation behind these docs is that no programmer or the end user, especially the end user, 
likes to read the manuals before diving in. The **\*.pptx**, are good alternatives to RTFM. 
The animated presentations, that omit just Sylvester from them, run for a maximum of 10 minutes and will save you 3+ hours
that it takes to go through text-based tutorials. I strongly believe that animated PowerPoint presentations are
much better in teaching than a YouTube video, as one is less likely to fall asleep and will not get distracted by 
another video of a dancing girl.

These \*.pptx presentations eliminate the need for a manual or text document, which was not even an option in the case of GSKY. 
For those who dare to RTFM, there are detailed Word/PDF documents as well.

Having alienated most of you with the intro, let us get into the details! 

Firstly, some mandatory statements...

License
-------

Copyright 2016-2019 National Computational Infrastructure, Australian National University, Canberra, ACT 2601.

Licensed under the Apache License, Version 2.0 (the "License"); you
may not use this package except in compliance with the License.  A
copy of the [License](http://www.apache.org/licenses/LICENSE-2.0) may
be found in this source distribution in [LICENSE-2.0.txt](../../LICENSE-2.0.txt).

Contributions
-------------

Suggestions, enhancement requests, bug reports and patches to GSKY are
welcome via this GitHub page. Please submit patches as a GitHub pull
request. Authors retain copyright over their contributions.

[![Build Status](https://travis-ci.org/nci/gsky.svg?branch=master)](https://github.com/asivapra/gsky)

Citing GSKY in publications
---------------------------

When referring to GSKY in publications please use the citation in
[CITATION.md](../../CITATION.md).  A ready-to-use BibTeX entry for LaTeX
users can also be found in this file.

How to use the documents
----------------------
There are four different types of documents for the same topic, though not in all cases, in here. 
Given below are short instructions on how to use them, which are not meant to be an insult to the 
intelligence of the knowledgeable!

- PowerPoint Presentation (**\*.pptx**):
	- Open in PowerPoint 2016 or later.
	- Click 'Slide Show' on menu bar and click "*From Beginning*".
		- It will auto run the slide show, but it could be too fast or too slow to your liking.
		- If desired, uncheck 'Use Timings' to manually control the timings.
	- The **\*.pptx** file is editable, if you wish to add anything (e.g. "*AAAARRRGHHHH!!*")

- PowerPoint Slide Show (**\*.ppsx**):
	- Open in PowerPoint 2016 or later. 
		- Slide show will start automatically.
		- It will run at the set speed.
	- There is no option to uncheck the timings. 
		- *"If you don't like duck, then you are rather stuck"* - Basil Fawlty (1975).
	- You can fast forward by clicking the right arrow.
		- It will auto restart the timed display if you don't click the arrow again.
	- Or, cancel the timing entirely by pressing the left arrow.
		- Then you MUST click the mouse or right/left arrow to move.
	- The \*.ppsx file is not editable.

- Word document (**\*.docx**):
	- Open in Word 2016 or later.

- PDF document (**\*.pdf**):
	- Open in the latest version of Adobe Acrobat.

List of Documents
-----------------

GSKY is currently delivered through **TerriaMap**. Providing another client for using GSKY might
increase the user base considerably. Several such clients are being considered, 
*e.g.* **ArcGIS, QGIS** and **NASA WorldView**. 

Some documents listed here describe the ArcGIS suite of programs that could be used as GSKY clients. Others
include **"GSKY User Guide"**, **"GSKY Developer Guide"**, **"Setting up GSKY server"**, 
**"Adding data into MAS"**, etc. More documents will be added as required.

This is an evolving README and the list of docs below may not be in alphabetical order, but the captions will
match the names of the documents.

ArcGIS_Online_Tutorial (.pptx)
---

This gives an understanding of **ArcGIS Online**. Though WMS is supported by this browser-based service,
there is an unresolved problem with connecting to the GSKY server. This tutorial is about other capabilities of ArcGIS and
will be useful when GSKY server is connecting and possibly even to add further capabilities in GSKY. 

*This program is about as easy to learn as escaping from the path of an approaching hurricane*. 
Hence we use an example that shows the best hurricane evacuation paths in a city where more than half 
the population has no personal transport.

ArcGIS_Pro_Desktop_Tutorial (.pptx)
---

Similar to the previous doc, this one gives an understanding of **ArcGIS Pro** which is a desktop variant 
of **ArcGIS Online** (sorry! Windows only; no Mac!). It is considerably more complex than the online version but
can display data from GSKY server. We use an example of predicting the future deforestation 
in Amazon wilderness due to the proposed new roads. 

How to use **ArcGIS Pro** as a GSKY client is explained.  

ArcGIS_Earth_Tutorial (.pptx)
---

This is a free software that gets installed as part of the ArcGIS Desktop suite. It can also be downloaded separately.
It supports WMS services and hence can be used as a GSKY client. 
To use it one needs an ArcGIS Online account, either public or a time-limited free trial. The public account is free, 
but not sure whether it too is time/feature limited. 

The tutorial describes the program usage and how to use **ArcGIS Pro** as a GSKY client is explained.

ArcMap_Tutorial (.pptx)
---

This standalone app does not require ArcGIS Online account. It supports WMS services and hence can be used as a GSKY client. 
It works with only 2D maps and sends the Bbox values as Lat/Lon. 

The tutorial describes the program usage and how to use **ArcGIS Pro** as a GSKY client is explained.

GSKY_ArcGIS_Integration (.docx)
---

This document aims to describe the basics of the **ArcGIS** suite of programs and how they can be used as GSKY clients.
The description based on text and images is a companion for the separate \*.pptx presentations for each app.

GSKY-Thredds_Integration (.pptx and .ppsx)
---

When a request is received from TerriaMap, GSKY aggregates the contents from several NetCDF files that constitute
the area and time slices requested. The resulting display is sent back to TerriaMap as a \*.png image, but does not
say which NetCDF files have contributed to it. This document describes a module added to GSKY to list the \*.nc
files and display them for download from a THREDDS server.

Though initially considered that it was a required functionality (my mistake!) it was later found to be a non-requirement.
Still, retaining this document for any possible future use.

GSKY_Crawl_MAS (.docx, .pdf, .pptx and .ppsx)
---

This document describes the steps involved in setting up datasets for OWS services such as GEOGLAM. 
The Word and PDF documents are detailed and list the process as well as code snippets to explain the process in detail. 
It can be used as a reference for new developers.

In the current production setup there are seven separate shell scripts that run consecutively. This document describes
each of them separately and also a new consolidated script that incorporates all the seven scripts.

The PPTX and PPSX describe the process through animated graphics. Viewing these 5 minute slide shows before reading the 
Word document will help to learn the process quicker.

GSKY_Developer_Guide (.pptx, .ppsx)
---

This doc is primarily for GSKY code developers, but has a section for the end users. It lists code snippets
and describe through animation the interaction between parts of the program. The services and requests
within the GSKY server are given as code and descriptions. 

It is a companion doc for GSKY_OWS_Server.docx which describes the code in detail so that a new programmer 
can work with it. 

GSKY_OWS_Server (.docx, .pdf, .pptx and .ppsx)
---

This document is a detailed description of the code base that creates and runs the GSKY server. To understand the process, 
the fundamental interactions at code level between TerriaMap and GSKY are explained but it is by no way a 
comprehensive description of how TerriaMap works.

The **\*.pptx** and **\*.ppsx** are animated presentations of the process flow of the GSKY server, showing code
snippets as required..  

It is hoped that some of the text from the Word doc will be inserted as comments within
the code, which is is virtually without any context-specific comments.  It will make the life easier
for a new programmer, even if s/he is a GOlang expert.

GSKY_User_Guide (.pptx and .ppsx)
---

How to use GSKY through TerriaMap is presented through animations. This 5 minute presentation will give the end user
a working knowledge of the GSKY Web Map Service. It does not include at present the WCS and WPS services.

GSKY_build_all (.pptx and .ppsx)
---

How to install and start a GSKY server. Includes the steps in creating a Virtual Machine on Tenjin. The doc describes
the installation of all dependencies plus the building of GSKY server and is a companion of 'build_all.sh' which is a 
one-step installation script. Code snippet for each step is displayed.

QGIS_Desktop_Tutorial (.pptx)
---

Basics of using QGIS. Though QGIS has WMS capabilities, currently they do not work with GSKY. This part is not
included in this presentation.

Setting_Up_Thredds_Server (.pptx)
---

How to setup a THREDDS server on the Virtual Machine on Tenjin. It was thought to be required for adding a functionality
to GSKY to let the component NetCDF files to be downloaded. Later it became clear that it was not an intended function.
However, this document will serve as a reference for setting up THREDDS server on any machine.

