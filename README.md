[![Build Status](https://travis-ci.org/abulimov/cadvisor-companion.svg?branch=master)](https://travis-ci.org/abulimov/cadvisor-companion)

# cAdvisor-companion

This tool is a small companion app for the
[Google's cAdvisor](https://github.com/google/cadvisor).

While cAdvisor
> provides container users an understanding
of the resource usage and performance characteristics of
their running containers
 - https://github.com/google/cadvisor

cAdvisor-companion shows whats going on inside the containers, by providing
API to get info about containerized processes.


## What is it for?

This tool was created mainly for monitoring events enrichment,
but can be used for variety of tasks.

I use cAdvisor-companion to add `ps aux --sort [pcpu|rss]`-like output
to notification events from monitoring, when one of the containers starts to
use too much CPU or memory.

My Nagios-compatible plugin **check_cadvisor.py** for checking
memory and CPU usage inside container is available in
[my *utils* repo](https://github.com/abulimov/utils/blob/master/nagios/check_cadvisor.py).
This plugin can use additional cAdvisor-companion url parameter to enrich
its own output with top mem/CPU-using processes.

Other usage example can be implementing cgroup-aware ps-like or top-like utility,
demo version of such utility is placed under **examples/ps.py**.


## Why not just use Docker top API?

`docker top` command, as for Docker 1.5, is just a wrapper for calling
`docker exec container_name ps` for given  container, and has all
disadvantages of this method.

For example, `ps` is not cgroup-aware, and shows wrong %MEM and %CPU
usage inside container. For additional information about this problem I recommend reading
[article by Fabio Kung](http://fabiokung.com/2014/03/13/memory-inside-linux-containers/).

Another reason is security problems with Docker access over network.
As for Docker 1.5, official documentation tells us that
>"If you are binding to a TCP port, anyone with access to that port has full Docker access; so it is not advisable on an open network."
- http://docs.docker.com/articles/basics/

cAdvisor-companion does not pose any security risks, because
all it needs to collect data is read-only access to hosts `/proc` filesystem.

Also, cAdvisor-companion should work with any cgroup-based container type,
like LXC or systemd-nspawn.



## API v1.0

### Processes

The resource name for processes is as follows:

`GET /api/v1.0/<absolute container name>/processes`

Where the absolute container name follows the lmctfy naming convention. For example:

| Container Name       | Resource Name                             |
|----------------------|-------------------------------------------|
| /foo                 | /api/v1.0/foo/processes                  |
| /docker/2c4dee605d22 | /api/v1.0/docker/2c4dee605d22/processes  |
| /system.slice/docker-7dc0bad.scope/ | /api/v1.0/system.slice/docker-7dc0bad.scope/processes  |

Docker < 1.6 containers are listed under `/docker`, Docker 1.6 on host
with systemd places its containers under cgroups like
`/system.slice/docker-{id}.scope`.

The processes list is returned as a JSON object containing list of timestamps and processes for the container for the last `N` *intervals* (`N` can be set with *count* get param).

**Example request**:

        GET /api/v1.0/docker/8847cf9188b478d504615fc0ab2d15943e24bfab7c643f1de34d898034587200/processes?interval=1&count=1&sort=mem&limit=1

**Example response**:

```
[
    {
        "timestamp": "2015-04-14T16:03:30.172968633Z",
        "processes": [
            {
                "status": {
                    "Name": "python",
                    "State": "S (sleeping)",
                    "Tgid": 18709,
                    "Pid": 18709,
                    "PPid": 32241,
                    ...
                    "CapBnd": 2818844155,
                    "Seccomp": 0,
                    "CpusAllowed": [
                        65535
                    ],
                    "MemsAllowed": [
                        0,
                        2
                    ],
                    "VoluntaryCtxtSwitches": 1141403,
                    "NonvoluntaryCtxtSwitches": 69023
                },
                "stat": {
                    "pid": 18709,
                    "comm": "(python)",
                    "state": "S",
                    "ppid": 32241,
                    "pgrp": 18709,
                    ...
                    "utime": 34616,
                    "stime": 4012,
                    "cutime": 0,
                    "cstime": 0,
                    "num_threads": 1,
                    "itrealvalue": 0,
                    "starttime": 1253207962,
                    "vsize": 371187712,
                    "rss": 40225,

                },
                "cmdline": "python /opt/someprog.py,
                "relativecpuusage": 0,
                "cgroup": "/docker/8847cf9188b478d504615fc0ab2d15943e24bfab7c643f1de34d898034587200"
            }
        ]
    }
]
```

Query Parameters:

-   **count** – show `count` history entries with `interval` seconds between them. Defaults to 1.
-   **sort** – Show processes sorted by `sort` field. sort=(cpu|mem).
-   **limit** – Show `limit` sorted processes, only works with `sort` parameter. Defaults to 1.
-   **interval** – Show history entries with `interval` seconds between them,
    and calculate relative CPU usage for `interval` seconds. Defaults to 1.

## Building executable

Run

```shell
go get github.com/abulimov/cadvisor-companion
go install github.com/abulimov/cadvisor-companion
```
and compiled executable will be in your $GOPATH/bin/

Alternatively, run `go get github.com/abulimov/cadvisor-companion`,
and run
```
cd $GOPATH/github.com/abulimov/cadvisor-companion
make build
```
inside projects directory.

## Building docker image

Just run `make docker` inside projects directory.

## Running

The easiest way to run cAdvisor-companion is using docker container:

    docker run \
    --volume=/:/rootfs:ro \
    --publish=8801:8801 \
    --detach=true \
    --name=cadvisor-companion \
    abulimov/cadvisor-companion:latest

Alternatively, you can build executable yourself, or even build
docker container yourself.

If you have built executable yourself, you can run it from the command line.
Type `./cadvisor-companion`, and the cAdvisor-companion server will stat running
in foreground listening on port 8801.

You can change this behavior with command line options, use
`cadvisor-companion -h` to get help.


## License

Licensed under the [MIT License](http://opensource.org/licenses/MIT),
see **LICENSE**.
