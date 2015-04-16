FROM progrium/busybox
MAINTAINER lazywolf0@gmail.com

# Grab cadvisor from the staging directory.
ADD cadvisor-companion /usr/bin/cadvisor-companion

EXPOSE 8801
ENTRYPOINT ["/usr/bin/cadvisor-companion"]
