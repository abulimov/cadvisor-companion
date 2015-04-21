#!/usr/bin/env python
# Copyright 2015 Alexander Bulimov. All rights reserved.
# Use of this source code is governed by a MIT
# license that can be found in the LICENSE file.
#
# Demo utility to show usage of cAdvisor-companion API
# to get ps-like output of processes inside given container

from __future__ import unicode_literals
from __future__ import division
from __future__ import print_function
import sys
import argparse
import json
import requests


def get_host_data(cadvisor_url, host_name):
    """Get cAdvisor url, hostname, and return host data in JSON"""
    response = requests.get(cadvisor_url + "/api/v1.2/containers/docker",
                            timeout=10)
    payload = json.loads(response.text)
    for cont in payload["subcontainers"]:
        host_raw_data = requests.get(cadvisor_url + "/api/v1.2/containers/" +
                                     cont["name"],
                                     timeout=10)
        host_data = json.loads(host_raw_data.text)
        if "aliases" in host_data and host_name in host_data["aliases"]:
            return host_data


def get_host_procs(companion_url, host_id, sort_by):
    """Get cAdvisor url, hostname, and return host data in JSON"""
    payload = {'sort': sort_by, 'interval': 10}
    ps_response = requests.get(companion_url + "/api/v1.0" + host_id +
                               "/processes",
                               params=payload,
                               timeout=10)
    ps = json.loads(ps_response.text)
    return ps[-1]["processes"]


def get_machine_data(cadvisor_url):
    """Get cAdvisor url and return parent host data in JSON"""
    response = requests.get(cadvisor_url + "/api/v1.2/machine")
    payload = json.loads(response.text)
    return payload


def show_procs(procs, host_data, machine_data, limit):
    """Pretty print host procs in ps-like fashion"""
    mem_limit = host_data["spec"]["memory"]["limit"]
    mem_limit_host = machine_data["memory_capacity"]

    if mem_limit > mem_limit_host:
        mem_limit = mem_limit_host
    mem_limit_kb = mem_limit / 1024

    result = ""
    result = "\n%5s %5s %5s %5s %8s %8s %6s %s\n" % (
        "USER", "PID", "%CPU", "%MEM", "VSZ", "RSS", "STAT", "COMMAND")
    limit = int(limit)
    if limit > 0 and limit < len(procs):
        real_limit = limit
    else:
        real_limit = len(procs)

    for proc in procs[:real_limit]:
        result += "%(user)5d %(pid)5d %(cpu)5.1f %(mem)5.1f \
                   %(vsz)8d %(rss)8d %(state)6s %(command)s\n" % {
            "user": proc["status"]["RealUid"],
            "pid": proc["stat"]["pid"],
            "cpu": proc["relativecpuusage"],
            "mem": proc["status"]["VmRSS"] * 100 / mem_limit_kb,
            "vsz": proc["status"]["VmSize"],
            "rss": proc["status"]["VmRSS"],
            "state": proc["stat"]["state"],
            "command": proc["cmdline"]}
    return result


def fail(message):
    print(message)
    sys.exit(3)

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='ps from cAdvisor-companion')
    parser.add_argument('-u', dest='url', required=True,
                        help='cAdvisor url')
    parser.add_argument('-U', dest='companion_url', required=True,
                        help='cAdvisor-companion url')
    parser.add_argument('-n', dest='name', required=True,
                        help='Docker container name')
    parser.add_argument('-l', dest='limit', required=False, default=0,
                        help='Limit output processes count')
    group = parser.add_mutually_exclusive_group(required=False)
    group.add_argument('-c', dest='cpu', action='store_true',
                       help='Sort by PCPU')
    group.add_argument('-m', dest='mem', action='store_true',
                       help='Sort by PMEM')
    args = parser.parse_args()

    sort_by = ""
    if args.cpu:
        sort_by = "cpu"
    elif args.mem:
        sort_by = "mem"
    try:
        host_data = get_host_data(args.url, args.name)
        machine_data = get_machine_data(args.url)
        host_procs = get_host_procs(args.companion_url,
                                    host_data["name"], sort_by)
        print(show_procs(host_procs, host_data, machine_data, args.limit))
    except (requests.exceptions.RequestException, ValueError) as e:
        fail(e)
