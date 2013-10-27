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

A script for creating an RPM for installtion on RedHat systems is provided.  In
order to run this script the desired default config.yml file and gpg keyring
must have been created.  The script is then run specifying where the RPM
installed service should look for its data files.

A similar script for Ubuntu is planned, but does not currently exist.  An
example upstart config file is provided in the init directory though.
