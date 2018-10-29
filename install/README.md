INTRODUCTION
============

This document is a detailed instruction set for creating an instance of a Virtual Machine (VM) on tenjin.nci.org.au and to build the GSKY environment on it. If you have already set it up once, then may require only the short instructions. To refresh memory, read the TL;DR section or the detailed text.

TL;DR
=====
- Create a VM instance on tenjin.nci.org.au.
	- Choose the option that gives at least 4GB RAM and 80GB disk.
	- Choose CentOS 7 as the operating system.
•	Remember that the VM is accessible only from within NCI network.
	- Use the ethernet for connection when in the office.
	- WiFi through ANU-Secure will not be enough, unless your PC’s IP is added to the firewall.
	- Use VPN to connect from remote locations or from within the office.
- The GSKY environment must be setup on each new VM.
	- Short instructions
	- Detailed instructions
- If no error during installation, it will take about 30 minutes to complete.
	- Ignore the warning messages during the installation.
	- Pay attention to error (crash) and exit messages.
	- If no error, both steps will display “Completed ALL steps. Exiting!”

SHORT INSTRUCTIONS
==================

Build a Virtual Machine
------------
	- Go to: https://tenjin.nci.org.au/dashboard/auth/login/
    - Login with your NCI username and password.
	- Instances >> Launch Instances >> Specify the details >> Launch
	- Wait at least 7 minutes for the setup to complete.

Build the GSKY environment
----------
	- Login to the VM via SSH
	- copy the ‘build_all.sh’ to the home dir.
	- cd ~
	- sudo ./build_all.sh

Do the following if the ‘build_all.sh’ is not given.

	- Login to the VM via SSH
	- cd ~
	- git clone https://github.com/nci/gsky.git
	- cp gsky/install/build_all.sh ~
	- sudo ./build_all.sh
  
