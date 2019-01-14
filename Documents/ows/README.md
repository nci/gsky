DOCS For GSKY 
============================================================
(Distributed Scalable Geospatial Data Server)
---



FOREWORD
-------------

GSKY was developed at [NCI](http://nci.org.au) and is a scalable, distributed 
server which presents a new approach for geospatial data discovery and delivery 
using OGC standards. The most recent release is version 1.0 (June 2018).

This directoy has all the documents and user guides for the GSKY system. If you find that
anything is missing or requires modifications, please contact the author directly or through
the comments.

These documents - in Word, PowerPoint and PDF - listed in here are intended to 
be useful for a beginner as well as the expert user of GSKY. Though written
mainly for the developers of the system(s), they will also double as user guides for the end users.

The docs are the by-products of the author's very frustrating travel 
through the GSKY code, and its associate programs, and they go against the new
philosophy of *"why make things simple when they can be wonderfully complex and un-intelligible?"*.

The motivation behind these docs is that no programmer or the end user, especially the end user, 
likes to read the docs before diving in. The animated pictorial docs, 
that omit just Silvester from them, eliminate the requirement which, in the case of GSKY, 
was not even an option. For those who dare to RTFM, there are detailed Word/PDF documents as well.

Having alienated most of you with the intro, let us get into the details! 

Firstly, some mandatory statements...

License
-------

Copyright 2016-2019 National Computational Infrastructure, Australian National University, Canberra, ACT 2601.

Licensed under the Apache License, Version 2.0 (the "License"); you
may not use this package except in compliance with the License.  A
copy of the [License](http://www.apache.org/licenses/LICENSE-2.0) may
be found in this source distribution in `LICENSE-2.0.txt`.

Contributions
-------------

Suggestions, enhancement requests, bug reports and patches to GSKY are
welcome via this GitHub page. Please submit patches as a GitHub pull
request. Authors retain copyright over their contributions.

[![Build Status](https://travis-ci.org/nci/gsky.svg?branch=master)](https://github.com/asivapra/gsky)

Citing GSKY in publications
---------------------------

When referring to GSKY in publications please use the citation in
[CITATION.md](CITATION.md).  A ready-to-use BibTeX entry for LaTeX
users can also be found in this file.

How to use the documents
----------------------
There are four different types of documents for the same topic, though some may be missing, in here. 
Given below are short instructions on how to use them 
(not intended to be an insult to the intelligence of the knowledgeable!)

- PowerPoint Presentation (\*.pptx):
	- Open in Power Point 2016 or later.
	- Click 'Slide Show' on menu bar.
		- Normally it will auto run the slide show, but it could be too fast or too slow to your liking.
	- Uncheck 'Use Timings' (optional) to manually control the timings.
	- Click "From Beginning" or the "slide show icon" on the bottom status bar.
	- The *.pptx file is editable, if you wish to add anything (e.g. *AAAARRRGHHHH!*)

- PowerPoint Slide Show (\*.ppsx):
	- Open in Power Point 2016 or later. Slide show will start automatically.
		- It will run at the set speed.
	- There is no option to uncheck the timings. 
		- *"If you don't like duck, then you are rather stuck"* - Basil Fawlty (1975).
	- But you can fast forward by clicking the right arrow.
		- It will auto start the timed display if you don't click the arrow again to insult it!
	- Or, cancel the timing completely by pressing the left arrow.
		- Then you MUST click the mouse or right arrow to move forward.
	- The *.ppsx file is not editable.

- Word document (\*.docx):
	- Open in Word 2016 or later.

- PDF document (\*.pdf):
	- Open in the latest version of Adobe Acrobat.

Documents in Alphabetical Order
---------------------------

ArcGIS_Online_Tutorial.pptx
---

GSKY is currently delivered through TerriaMap alone. Providing another client for using GSKY might
increase the user base considerably. Several such clients are being considered, 
***e.g.*** **ArcGIS, QGIS** and **NASA WorldView**.

This document gives a basic understanding of **ArcGIS Online**. Though it will not give
any help with using GSKY through ArcGIS, learning about the capabilities of ArcGIS will be useful later 
(possibly even to add further capabilities in GSKY). 

This program is about as easy to learn as escaping from the path of an approaching hurricane. 
So, we used an example that shows the best hurricane evacuation paths in a city where more than half 
the population has no transport.

ArcGIS_Pro_Desktop_Tutorial.pptx
---

Similar to the previous doc, this one gives a basic understanding of **ArcGIS Pro** which is a desktop variant of 
**ArcGIS Online** (sorry, Windows only, no Mac!). We use an example of predicting the future deforestation 
in Amazon wilderness due to the proposed new roads. It is considerably more complex than the online version, but there is 
no need to run for your life (wild life will do it instead). 

GSKY-Thredds_Integration (\*.pptx and \*.ppsx)
---

When a request is received from TerriaMap, GSKY aggregates the contents from several NetCDF files that constitute
the area and time slices requested. The resulting display is sent back to TerriaMap as a \*.png image, but does not
say which NetCDF files have contributed to it. This document describes a module added to GSKY to list the \*.nc
files and display them as links to download from a THREDDS server.

Though initially thought that it was a required functionality (my mistake!) it was later found to be of no consequence.
Still, retaining this document for any possible future requirement.

GSKY_Crawl_MAS (.docx, .pdf, .pptx and .ppsx)
---

This document describes the steps involved in setting up datasets for OWS services such as GEOGLAM. 
The Word and PDF documents are detailed and list the process as well as code snippets to explain the process in detail. 
It can be used as a reference for new developers.

In the current production setup there are seven separate shell scripts that run consecutively. This document describes
each of them separately and also a new combined script that incorporates all the seven scripts.

The PPTX and PPSX describe the process through animated graphics. Viewing these 5 min slide shows before reading the 
Word document will help learn the process quicker.




