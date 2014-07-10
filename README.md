#About:
This is a small tool to measure site to site RTT/latency (curently only thru the UDPecho;it does separate echo for DSCP classes
CS1-6) with ability to autodiscover probes (with help of central controller) and reporting the results to
external storage/graphing tool (graphite atm; probes sends result of measures to central controller, which
sends it forward to reporter). Right now it's a lil bit chatty but have all the basic features (so could be,
and actualy does, used for measurment in real network)

#TODO:
* increase overall stability (right now it's highly unstable) such as:
    error handling
    probes reconnect to master logic
    reconect to reporter logic
    etc
* add support for multimaster topologies (includes hierarhical topologies 2+ layers of master nodes) 
* ipv6 support  

