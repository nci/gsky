INTRODUCTION
============
This document is a detailed instruction set for creating an instance of a Virtual Machine (VM) on tenjin.nci.org.au and to build the GSKY environment on it. If you have already set it up once, then may require only the short instructions. To refresh memory, read the TL;DR section or the detailed text.

TL;DR
=====
- Create a VM instance on tenjin.nci.org.au.
	- Choose the option that gives at least 4GB RAM and 80GB disk.
	- Choose CentOS 7 as the operating system.
- Remember that the VM is accessible only from within NCI network.
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
------------------

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
	- Transfer ‘build_all.sh’ to the home dir.
	- cd ~
	- sudo ./build_all.sh

Do the following if the ‘build_all.sh’ is not given.

	- Login to the VM via SSH
	- cd ~
	- git clone https://github.com/nci/gsky.git
	- cp gsky/install/build_all.sh ~
	- sudo ./build_all.sh
	
------------------

DETAILED INSTRUCTIONS
==================

Build a Virtual Machine
-------------------------

The VM instances on https://tenjin.nci.org.au/dashboard/project/instances/ are where the GSKY installation and servers are built and managed.

- Go to https://tenjin.nci.org.au/dashboard/project/
	- Click on ‘Instances’ and then on ‘Launch Instance’
![Step1](pic1.png) 
![Step2](pic2.png) 

- Fill in details as per the example below.

![Step3](pic3.png) 

- Create/choose a key pair if you wish to use it for password-less login. Otherwise, leave it as blank.*
	- Click the + icon to create a new key pair.
	
![Step4](pic4.png) 

- Create the instance by clicking ‘Launch’

![Step5](pic5.png) 

- Wait 10 min for the installation to complete.
- Click on the instance name.

![Step6](pic6.png) 

 - Click on ‘Console’ tab and then on the hypertext link, “Click here…”
 
![Step7](pic7.png) 

- Login with NCI username/password when prompted. This is now a regular VM console.
- If you are on the NCI network, either through ethernet or VPN, the following step is not required.
- Change to ‘sudo -I’ and add the IP address of your local PC to the iptables
	- This step is necessary only when not on the NCI network through ethernet.
	-cd /etc/puppetlabs/code/environments/production/
	-cat > hieradata/node/siva-gsky.yaml (same name as instance.yaml)
- nci::firewall::ruleset::ssh::sources_array:
	- – “130.56.84.195”
	-Or 
- vim hieradata/node/siva-gsky.yaml
- Change the IP address
	-puppet-apply
	-iptables –list

Create an SSH key pair:
=================
NOTE: You will only need this if intending to login and/or transfer files between the VM and another Unix server. There is not much use of it under Microsoft Windows, as ‘putty’ and ‘winscp’ cannot use the openssh keys. Converting them to putty key (*.ppk) does not work either, as the VM does not recognise it. 

To create:
-------------
- SSH to your “home machine”
	- ssh-keygen -t rsa -f username.key
- Copy the content of ‘username.key.pub’ and paste it into the box.
- Keep the ‘username.key’ safely and securely.
-------------------
