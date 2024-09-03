# Allocation Manager

Allocation Manager is our out-of-band hardware management server that
gates individual actions run via bazel until the requested hardware is
available for use.

This is necessary since the hardware is varied, and matching requests
to available resources must be done late, avoiding potential queueing
from Bazel RBE (i.e. Buildbarn).

The `allocation_manager` server maintains a list of resources, and
individual actions are wrapped with a client that obtains a token from
the `allocation_manager` server before running their command. The
`allocation_manager` protocol has keepalive mechanisms that will
aggressively expire the token if clients become unresponsive, unblocking
subsequent actions.

More details in [this
doc](https://docs.google.com/document/d/159yGV740cevREkp2P57w4_qASaO2sRlRv-y1-5YYiEU/edit#heading=h.i8mhtcz6pa0u).
