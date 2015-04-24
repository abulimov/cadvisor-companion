## v0.1.4 [2015-04-24]

- Fixed parsing for non-Docker cgroups
- Added support of Docker 1.6

## v0.1.3 [2015-04-24]

- Fixed compilation on go < 1.4
- Updated Makefile
- Fixed wrong mem_limit calculation in examples/ps.py

## v0.1.2 [2015-04-20]

- Fixed time drift by using time.Tick to collect data

## v0.1.1 [2015-04-20]

- Removed non-documented default limit for TopCPU and TopMem requests
- Added example ps-like utility

## v0.1.0 [2015-04-16]

- First public release
- API 1.0 considered stable
- Added basic tests
