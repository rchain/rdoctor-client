### RDoctor client

RDoctor is a tool for collecting logs mostly from RChain nodes for debugging.
This respository contains the client part.

You can get the latest version of `rdoctor` client executable from https://build.rchain-dev.tk/misc/rdoctor/latest/

- executable for MacOS is in `darwin.amd64` directory
- executable for Linux is in `linux.amd64` directory

To use it either

1. put it in some location that is in your `PATH`, for instance `/usr/local/bin`
   (as root), or
2. change into the directory where you downloaded the executable to.

Then you prepend the command you use to run `rnode` with `rnode` or `./rnode`
depending on where you put the executable.

#### Example (MacOS)

    curl -O https://build.rchain-dev.tk/misc/rdoctor/latest/darwin.amd64/rdoctor
    sudo cp rdoctor /usr/local/bin
    cd /path/to/rnode
    rdoctor ./bin/rnode ... # rest of the arguments for rnode
