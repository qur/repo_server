repo_server
===========

A standalone remotely controllable apt repository server

The purpose of repo server was to provide an apt repository server where
packages can be added and removed from a signed repository remotely using a
command line client.  In addition the ability to create and delete additional
temporary repositories was also required.

client
------

A Python client for the REST API provided by the server can be found in the
client subdirectory.

configuration
-------------

An example configuration file, with comments describing the various options is
provided as example_config.yml

installation
------------

Scripts are provided for building RedHat targeted RPMs and Ubuntu targeted debs
in the package directory.  In order to run these scripts the desired default
config.yml file and gpg keyring must have been created in addition to having
built the repo_server executable.  The script is then run specifying where the
installed service should look for its data files.
